package migrations

import (
	"billionmail-core/internal/service/identity"
	"billionmail-core/internal/service/integrations"
	platformqueue "billionmail-core/internal/service/queue"
	"billionmail-core/internal/service/scheduler"
	"billionmail-core/internal/service/secrets"
	"billionmail-core/internal/service/tenancy"
	"billionmail-core/internal/service/whatsappchannel"
	"billionmail-core/internal/service/whatsappwebhook"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	_ "github.com/gogf/gf/contrib/drivers/pgsql/v2"
	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/google/uuid"
	"github.com/hibiken/asynq"
)

type recordingPublisher struct{ events []platformqueue.Event }

type integrationSecretStore struct{}

func (integrationSecretStore) Encrypt(_ context.Context, _ uuid.UUID, plaintext []byte) (secrets.Envelope, error) {
	ciphertext := make([]byte, len(plaintext))
	for index := range plaintext {
		ciphertext[index] = plaintext[index] ^ 0xa5
	}
	return secrets.Envelope{Ciphertext: ciphertext, EncryptedDataKey: []byte("integration-key"), KeyARN: "integration-kms-key"}, nil
}

func (integrationSecretStore) Decrypt(_ context.Context, _ uuid.UUID, envelope secrets.Envelope) ([]byte, error) {
	plaintext := make([]byte, len(envelope.Ciphertext))
	for index := range envelope.Ciphertext {
		plaintext[index] = envelope.Ciphertext[index] ^ 0xa5
	}
	return plaintext, nil
}

func (p *recordingPublisher) Publish(_ context.Context, event platformqueue.Event) error {
	p.events = append(p.events, event)
	return nil
}

func TestFoundationMigrationAndTenantRLS(t *testing.T) {
	password := os.Getenv("BM_TEST_DB_PASSWORD")
	if password == "" {
		t.Skip("BM_TEST_DB_PASSWORD is not configured")
	}
	database := envOr("BM_TEST_DB_NAME", "billionmail_tenant_test")
	err := gdb.SetConfig(gdb.Config{"default": gdb.ConfigGroup{gdb.ConfigNode{
		Host: envOr("BM_TEST_DB_HOST", "127.0.0.1"), Port: envOr("BM_TEST_DB_PORT", "25432"),
		User: envOr("BM_TEST_DB_USER", "billionmail"), Pass: password, Name: database,
		Type: "pgsql", Role: "master", MaxOpenConnCount: 4,
	}}})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	bootstrap := []string{
		`CREATE TABLE account(account_id BIGSERIAL PRIMARY KEY,username VARCHAR(64) UNIQUE NOT NULL,password VARCHAR(255) NOT NULL,email VARCHAR(255) NOT NULL DEFAULT '',status INT NOT NULL DEFAULT 1,language VARCHAR(16),create_time BIGINT,update_time BIGINT,last_login_time BIGINT)`,
		`CREATE TABLE role(role_id BIGSERIAL PRIMARY KEY,role_name VARCHAR(64) UNIQUE NOT NULL)`,
		`CREATE TABLE account_role(id BIGSERIAL PRIMARY KEY,account_id BIGINT REFERENCES account(account_id),role_id BIGINT REFERENCES role(role_id))`,
		`INSERT INTO account(username,password,email,status) VALUES('legacy-admin','hash','admin@example.test',1)`,
		`INSERT INTO role(role_name) VALUES('admin')`,
		`INSERT INTO account_role(account_id,role_id) SELECT a.account_id,r.role_id FROM account a CROSS JOIN role r`,
		`CREATE TABLE bm_contacts(id BIGSERIAL PRIMARY KEY,email VARCHAR(320) NOT NULL)`,
		`INSERT INTO bm_contacts(email) VALUES('legacy@example.test')`,
	}
	for _, statement := range bootstrap {
		if _, err = g.DB().Exec(ctx, statement); err != nil {
			t.Fatalf("bootstrap failed: %v", err)
		}
	}
	if err = Up(ctx); err != nil {
		t.Fatalf("first Up: %v", err)
	}
	if err = Up(ctx); err != nil {
		t.Fatalf("idempotent Up: %v", err)
	}
	if ready, err := Ready(ctx); err != nil || !ready {
		t.Fatalf("applied schema is not ready: %v", err)
	}
	if _, err := g.DB().Model("goose_db_version").Where("version_id", 8).Data(g.Map{"is_applied": false}).Update(); err != nil {
		t.Fatal(err)
	}
	if ready, err := Ready(ctx); err != nil || ready {
		t.Fatalf("incomplete schema reported ready: %v", err)
	}
	if _, err := g.DB().Model("goose_db_version").Where("version_id", 8).Data(g.Map{"is_applied": true}).Update(); err != nil {
		t.Fatal(err)
	}

	companyValue, err := g.DB().Model("companies").Where("is_legacy_default", true).Value("id")
	if err != nil {
		t.Fatal(err)
	}
	companyID := companyValue.String()
	if _, err = uuid.Parse(companyID); err != nil {
		t.Fatalf("invalid default company: %q", companyID)
	}
	count, err := g.DB().Model("bm_contacts").Where("company_id", companyID).Count()
	if err != nil || count != 1 {
		t.Fatalf("legacy contact backfill count=%d error=%v", count, err)
	}
	memberCount, err := g.DB().Model("company_members").Where("company_id", companyID).Count()
	if err != nil || memberCount != 1 {
		t.Fatalf("default membership count=%d error=%v", memberCount, err)
	}

	otherCompany := uuid.New()
	if _, err = g.DB().Model("companies").Data(g.Map{"id": otherCompany, "name": "Other", "slug": "other", "status": "active"}).Insert(); err != nil {
		t.Fatal(err)
	}
	if _, err = g.DB().Model("company_settings").Data(g.Map{"company_id": otherCompany}).Insert(); err != nil {
		t.Fatal(err)
	}
	roleName := "bm_tenant_rls_test"
	_, _ = g.DB().Exec(ctx, fmt.Sprintf("DROP ROLE IF EXISTS %s", roleName))
	if _, err = g.DB().Exec(ctx, fmt.Sprintf("CREATE ROLE %s NOLOGIN", roleName)); err != nil {
		t.Fatal(err)
	}
	defer g.DB().Exec(ctx, fmt.Sprintf("DROP ROLE IF EXISTS %s", roleName))
	if _, err = g.DB().Exec(ctx, fmt.Sprintf("GRANT USAGE ON SCHEMA public TO %s; GRANT SELECT,INSERT ON company_settings TO %s", roleName, roleName)); err != nil {
		t.Fatal(err)
	}

	err = g.DB().Transaction(ctx, func(_ context.Context, tx gdb.TX) error {
		if _, txErr := tx.Exec(fmt.Sprintf("SET LOCAL ROLE %s", roleName)); txErr != nil {
			return txErr
		}
		if _, txErr := tx.Exec("SELECT set_config('app.company_id', ?, true)", companyID); txErr != nil {
			return txErr
		}
		visible, txErr := tx.Model("company_settings").Count()
		if txErr != nil {
			return txErr
		}
		if visible != 1 {
			return fmt.Errorf("RLS exposed %d company settings, want 1", visible)
		}
		if _, txErr = tx.Exec("SAVEPOINT cross_tenant_write"); txErr != nil {
			return txErr
		}
		_, crossTenantErr := tx.Model("company_settings").Data(g.Map{"company_id": otherCompany}).Insert()
		if crossTenantErr == nil {
			return fmt.Errorf("RLS allowed cross-tenant insert")
		}
		if _, txErr = tx.Exec("ROLLBACK TO SAVEPOINT cross_tenant_write"); txErr != nil {
			return txErr
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Run("identity lifecycle", func(t *testing.T) {
		secret := strings.Repeat("integration-secret-", 3)
		userID, registerErr := identity.Register(ctx, identity.Registration{
			Username: "new-owner", Email: "owner@newco.test", Password: "SecurePassword123",
			CompanyName: "New Company", Timezone: "UTC", Locale: "en",
		}, secret, "https://app.example.test")
		if registerErr != nil {
			t.Fatal(registerErr)
		}
		var user struct {
			Status            string     `json:"status"`
			EmailVerifiedAt   *time.Time `json:"email_verified_at"`
			LegacyAccountID   int64      `json:"legacy_account_id"`
			MembershipVersion int64      `json:"membership_version"`
		}
		if scanErr := g.DB().Model("users").Where("id", userID).Scan(&user); scanErr != nil {
			t.Fatal(scanErr)
		}
		if user.Status != "pending" || user.EmailVerifiedAt != nil {
			t.Fatalf("registration bypassed verification: %+v", user)
		}
		accountStatus, statusErr := g.DB().Model("account").Where("account_id", user.LegacyAccountID).Value("status")
		if statusErr != nil || accountStatus.Int() != 0 {
			t.Fatalf("legacy login was enabled before verification: status=%v err=%v", accountStatus, statusErr)
		}

		verificationValue, tokenErr := g.DB().Model("email_verification_tokens").Where("user_id", userID).Value("id")
		if tokenErr != nil {
			t.Fatal(tokenErr)
		}
		verificationID, tokenErr := uuid.Parse(verificationValue.String())
		if tokenErr != nil {
			t.Fatal(tokenErr)
		}
		verificationToken, _, tokenErr := identity.BuildActionToken(verificationID, "verify-email", secret)
		if tokenErr != nil {
			t.Fatalf("build verification token: %v", tokenErr)
		}
		if verifyErr := identity.VerifyEmail(ctx, verificationToken); verifyErr != nil {
			t.Fatalf("verification failed: %v", verifyErr)
		}
		accountStatus, _ = g.DB().Model("account").Where("account_id", user.LegacyAccountID).Value("status")
		if accountStatus.Int() != 1 {
			t.Fatal("verification did not enable legacy login")
		}

		if resetErr := identity.RequestPasswordReset(ctx, "owner@newco.test", secret, "https://app.example.test"); resetErr != nil {
			t.Fatal(resetErr)
		}
		resetValue, resetErr := g.DB().Model("password_reset_tokens").Where("user_id", userID).WhereNull("used_at").Value("id")
		if resetErr != nil {
			t.Fatal(resetErr)
		}
		resetID, _ := uuid.Parse(resetValue.String())
		resetToken, _, _ := identity.BuildActionToken(resetID, "reset-password", secret)
		companyValue, _ := g.DB().Model("company_members").Where("user_id", userID).Value("company_id")
		companyID, _ := uuid.Parse(companyValue.String())
		if _, sessionErr := g.DB().Model("auth_sessions").Data(g.Map{
			"id": uuid.New(), "user_id": userID, "token_family": uuid.New(), "refresh_token_hash": []byte("integration-session"),
			"active_company_id": companyID, "expires_at": time.Now().Add(time.Hour),
		}).Insert(); sessionErr != nil {
			t.Fatal(sessionErr)
		}
		if resetErr = identity.ResetPassword(ctx, resetToken, "ReplacementPassword456"); resetErr != nil {
			t.Fatal(resetErr)
		}
		activeSessions, _ := g.DB().Model("auth_sessions").Where("user_id", userID).WhereNull("revoked_at").Count()
		if activeSessions != 0 {
			t.Fatal("password reset did not revoke active sessions")
		}
		outboxWithoutTenant, _ := g.DB().Model("outbox_events").Where("aggregate_id", userID.String()).WhereNull("company_id").Count()
		if outboxWithoutTenant != 0 {
			t.Fatal("identity outbox event was not company scoped")
		}

		secretStore := integrationSecretStore{}
		channelInput := whatsappchannel.ConnectInput{
			WABAID: "waba-integration", BusinessName: "Integration Business", AccessToken: "first-meta-token",
			Scopes:       []string{"whatsapp_business_management", "whatsapp_business_messaging"},
			PhoneNumbers: []whatsappchannel.PhoneAsset{{PhoneNumberID: "phone-integration", DisplayPhoneNumber: "+15550001111"}},
		}
		channelTenant := tenancy.TenantContext{UserID: userID, CompanyID: companyID, Role: "owner"}
		connection, connectErr := whatsappchannel.Connect(ctx, channelTenant, secretStore, channelInput)
		if connectErr != nil {
			t.Fatalf("connect WhatsApp channel: %v", connectErr)
		}
		channelInput.AccessToken = "rotated-meta-token"
		rotated, connectErr := whatsappchannel.Connect(ctx, channelTenant, secretStore, channelInput)
		if connectErr != nil || rotated.AccountID != connection.AccountID {
			t.Fatalf("rotate WhatsApp credential: connection=%+v err=%v", rotated, connectErr)
		}
		credentials, _ := g.DB().Model("whatsapp_credentials").Where("company_id", companyID).Where("whatsapp_account_id", connection.AccountID).Count()
		revokedCredentials, _ := g.DB().Model("whatsapp_credentials").Where("company_id", companyID).Where("whatsapp_account_id", connection.AccountID).WhereNotNull("revoked_at").Count()
		if credentials != 2 || revokedCredentials != 1 {
			t.Fatalf("credential rotation stored=%d revoked=%d, want 2/1", credentials, revokedCredentials)
		}
		accessToken, tokenLoadErr := whatsappchannel.AccessToken(ctx, companyID, connection.AccountID, secretStore)
		if tokenLoadErr != nil || string(accessToken) != "rotated-meta-token" {
			t.Fatalf("load active credential token=%q err=%v", accessToken, tokenLoadErr)
		}
		if _, wrongTenantErr := whatsappchannel.AccessToken(ctx, otherCompany, connection.AccountID, secretStore); wrongTenantErr == nil {
			t.Fatal("cross-tenant credential lookup succeeded")
		}
		crossTenantInput := channelInput
		crossTenantInput.PhoneNumbers = []whatsappchannel.PhoneAsset{{PhoneNumberID: "other-phone"}}
		_, crossTenantErr := whatsappchannel.Connect(ctx, tenancy.TenantContext{UserID: uuid.New(), CompanyID: otherCompany, Role: "owner"}, secretStore, crossTenantInput)
		if !errors.Is(crossTenantErr, whatsappchannel.ErrChannelUnavailable) {
			t.Fatalf("cross-tenant WABA claim error=%v, want channel unavailable", crossTenantErr)
		}
		webhookBody := []byte(`{"object":"whatsapp_business_account","entry":[{"changes":[{"field":"messages","value":{"metadata":{"phone_number_id":"phone-integration"},"messages":[{"id":"wamid.integration","type":"text"}]}}]}]}`)
		firstWebhook, webhookErr := whatsappwebhook.Ingest(ctx, webhookBody, "sha256=test")
		if webhookErr != nil || firstWebhook.Duplicate {
			t.Fatalf("first webhook ingest result=%+v err=%v", firstWebhook, webhookErr)
		}
		secondWebhook, webhookErr := whatsappwebhook.Ingest(ctx, webhookBody, "sha256=test")
		if webhookErr != nil || !secondWebhook.Duplicate {
			t.Fatalf("duplicate webhook ingest result=%+v err=%v", secondWebhook, webhookErr)
		}
		webhookCount, _ := g.DB().Model("webhook_events").Where("company_id", companyID).Count()
		if webhookCount != 1 {
			t.Fatalf("webhook idempotency stored %d rows, want 1", webhookCount)
		}

		publisher := &recordingPublisher{}
		dispatcher := platformqueue.Dispatcher{Publisher: publisher, WorkerID: "integration-worker"}
		published, dispatchErr := dispatcher.Dispatch(ctx)
		if dispatchErr != nil {
			t.Fatal(dispatchErr)
		}
		if published < 2 || len(publisher.events) != published {
			t.Fatalf("outbox published=%d recorded=%d, want identity events", published, len(publisher.events))
		}
		pending, _ := g.DB().Model("outbox_events").WhereNull("published_at").WhereNull("dead_lettered_at").Count()
		if pending != 0 {
			t.Fatalf("outbox left %d due events unpublished", pending)
		}
	})

	t.Run("integration credentials and tenant isolation", func(t *testing.T) {
		company, _ := uuid.Parse(companyID)
		userValue, err := g.DB().Model("users").Where("username", "legacy-admin").Value("id")
		if err != nil {
			t.Fatal(err)
		}
		user, _ := uuid.Parse(userValue.String())
		for i := 0; i < 12; i++ {
			_, token, err := tenancy.CreateSession(ctx, user, company, "integration-test", "127.0.0.1", time.Hour)
			if err != nil {
				t.Fatal(err)
			}
			next, _, err := tenancy.RotateSession(ctx, token, time.Hour)
			if err != nil {
				t.Fatalf("binary refresh hash roundtrip: %v", err)
			}
			if _, _, err = tenancy.RotateSession(ctx, token, time.Hour); !errors.Is(err, tenancy.ErrInvalidRefreshToken) {
				t.Fatalf("refresh reuse err=%v", err)
			}
			active, err := g.DB().Model("auth_sessions").Where("token_family", next.TokenFamily).WhereNull("revoked_at").Count()
			if err != nil || active != 0 {
				t.Fatalf("reuse revocation did not commit: active=%d err=%v", active, err)
			}
		}
		tenant := tenancy.TenantContext{CompanyID: company, UserID: user, Role: "owner"}
		store := integrationSecretStore{}
		id, err := integrations.Create(ctx, tenant, store, integrations.Input{Provider: "test-provider", Capability: "image", Name: "primary", Secret: "private-test-credential"})
		if err != nil {
			t.Fatal(err)
		}
		connections, err := integrations.List(ctx, tenant)
		if err != nil || len(connections) != 1 || connections[0].Status != "unverified" {
			t.Fatalf("list=%+v err=%v", connections, err)
		}
		other := tenancy.TenantContext{CompanyID: otherCompany, UserID: user, Role: "owner"}
		connections, err = integrations.List(ctx, other)
		if err != nil || len(connections) != 0 {
			t.Fatalf("cross-tenant list=%+v err=%v", connections, err)
		}
		if _, err = integrations.Credential(ctx, otherCompany, id, store); err == nil {
			t.Fatal("cross-tenant credential readable")
		}
		if err = integrations.Rotate(ctx, other, store, id, "stolen"); err == nil {
			t.Fatal("cross-tenant credential rotation allowed")
		}
		if err = integrations.Rotate(ctx, tenant, store, id, "rotated-test-credential"); err != nil {
			t.Fatal(err)
		}
		credential, err := integrations.Credential(ctx, company, id, store)
		if err != nil || string(credential) != "rotated-test-credential" {
			t.Fatalf("active credential lookup err=%v", err)
		}
		if err = integrations.Disconnect(ctx, tenant, id); err != nil {
			t.Fatal(err)
		}
		if _, err = integrations.Credential(ctx, company, id, store); err == nil {
			t.Fatal("disconnected credential readable")
		}
		if _, err = integrations.Create(ctx, tenancy.TenantContext{CompanyID: company, UserID: user, Role: "viewer"}, store, integrations.Input{}); !errors.Is(err, tenancy.ErrPermissionDenied) {
			t.Fatalf("viewer create err=%v", err)
		}
	})

	t.Run("durable scheduler", func(t *testing.T) {
		company, _ := uuid.Parse(companyID)
		eventType := "platform.schedule.probe"
		makeSchedule := func(fields g.Map) uuid.UUID {
			t.Helper()
			id := uuid.New()
			data := g.Map{"id": id, "company_id": company, "name": "probe", "event_type": eventType, "next_run_at": time.Now().Add(-time.Minute)}
			for key, value := range fields {
				data[key] = value
			}
			if _, err := g.DB().Model("marketing_schedules").Data(data).Insert(); err != nil {
				t.Fatal(err)
			}
			return id
		}
		for i := 0; i < 12; i++ {
			makeSchedule(nil)
		}
		var wg sync.WaitGroup
		errorsCh := make(chan error, 2)
		for _, worker := range []string{"scheduler-a", "scheduler-b"} {
			wg.Add(1)
			go func(worker string) {
				defer wg.Done()
				_, err := (scheduler.Scheduler{WorkerID: worker, AllowedEvents: map[string]bool{eventType: true}}).Tick(ctx)
				errorsCh <- err
			}(worker)
		}
		wg.Wait()
		close(errorsCh)
		for err := range errorsCh {
			if err != nil {
				t.Fatal(err)
			}
		}
		runs, err := g.DB().Model("marketing_job_runs").Where("company_id", company).Count()
		if err != nil || runs != 12 {
			t.Fatalf("concurrent claim runs=%d err=%v, want 12", runs, err)
		}
		runner := scheduler.Scheduler{WorkerID: "recovery-worker", AllowedEvents: map[string]bool{eventType: true}}
		if count, err := runner.Tick(ctx); err != nil || count != 0 {
			t.Fatalf("duplicate tick count=%d err=%v", count, err)
		}
		leasedID := makeSchedule(g.Map{"leased_by": "terminated-worker", "leased_until": time.Now().Add(time.Hour)})
		if count, err := runner.Tick(ctx); err != nil || count != 0 {
			t.Fatalf("stole live lease count=%d err=%v", count, err)
		}
		if _, err := g.DB().Model("marketing_schedules").Where("id", leasedID).Data(g.Map{"leased_until": time.Now().Add(-time.Second)}).Update(); err != nil {
			t.Fatal(err)
		}
		if count, err := runner.Tick(ctx); err != nil || count != 1 {
			t.Fatalf("expired lease recovery count=%d err=%v", count, err)
		}
		pausedID := makeSchedule(g.Map{"status": "paused"})
		unknownID := makeSchedule(g.Map{"event_type": "unregistered.external.send"})
		if count, err := runner.Tick(ctx); err != nil || count != 0 {
			t.Fatalf("unsafe schedules count=%d err=%v", count, err)
		}
		for id, want := range map[uuid.UUID]string{pausedID: "paused", unknownID: "failed"} {
			status, err := g.DB().Model("marketing_schedules").Where("id", id).Value("status")
			if err != nil || status.String() != want {
				t.Fatalf("schedule status=%v err=%v want %s", status, err, want)
			}
		}
		// A failure after the outbox insert must roll back both records. The
		// durable lease is then recovered after the crashed owner expires.
		atomicID := makeSchedule(nil)
		if _, err := g.DB().Exec(ctx, `CREATE FUNCTION fail_test_job_insert() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'injected failure'; END $$; CREATE TRIGGER fail_test_job_insert BEFORE INSERT ON marketing_job_runs FOR EACH ROW EXECUTE FUNCTION fail_test_job_insert()`); err != nil {
			t.Fatal(err)
		}
		if _, err := runner.Tick(ctx); err == nil {
			t.Fatal("injected transaction failure did not propagate")
		}
		if _, err := g.DB().Exec(ctx, `DROP TRIGGER fail_test_job_insert ON marketing_job_runs; DROP FUNCTION fail_test_job_insert()`); err != nil {
			t.Fatal(err)
		}
		outboxCount, err := g.DB().Model("outbox_events").Where("aggregate_id", atomicID.String()).Count()
		if err != nil || outboxCount != 0 {
			t.Fatalf("failed transaction leaked outbox=%d err=%v", outboxCount, err)
		}
		if _, err := g.DB().Model("marketing_schedules").Where("id", atomicID).Data(g.Map{"leased_until": time.Now().Add(-time.Second)}).Update(); err != nil {
			t.Fatal(err)
		}
		if count, err := runner.Tick(ctx); err != nil || count != 1 {
			t.Fatalf("rolled back occurrence recovery count=%d err=%v", count, err)
		}
	})
	t.Run("business profile", func(t *testing.T) {
		company, _ := uuid.Parse(companyID)
		userValue, err := g.DB().Model("users").Where("username", "legacy-admin").Value("id")
		if err != nil {
			t.Fatal(err)
		}
		user, _ := uuid.Parse(userValue.String())
		testBusinessProfile(t, ctx, tenancy.TenantContext{CompanyID: company, UserID: user, Role: "owner"}, tenancy.TenantContext{CompanyID: otherCompany, UserID: user, Role: "owner"})
	})
	t.Run("product catalog", func(t *testing.T) {
		company, _ := uuid.Parse(companyID)
		userValue, err := g.DB().Model("users").Where("username", "legacy-admin").Value("id")
		if err != nil {
			t.Fatal(err)
		}
		user, _ := uuid.Parse(userValue.String())
		testProducts(t, ctx, tenancy.TenantContext{CompanyID: company, UserID: user, Role: "owner"}, tenancy.TenantContext{CompanyID: otherCompany, UserID: user, Role: "owner"})
	})
	t.Run("persistent project workspace", func(t *testing.T) {
		company, _ := uuid.Parse(companyID)
		userValue, err := g.DB().Model("users").Where("username", "legacy-admin").Value("id")
		if err != nil {
			t.Fatal(err)
		}
		user, _ := uuid.Parse(userValue.String())
		testProjectWorkspace(t, ctx, tenancy.TenantContext{CompanyID: company, UserID: user, Role: "owner"}, tenancy.TenantContext{CompanyID: otherCompany, UserID: user, Role: "owner"})
	})
	t.Run("asset upload validation", func(t *testing.T) {
		company, _ := uuid.Parse(companyID)
		actorValue, err := g.DB().Model("users").Where("username", "legacy-admin").Value("id")
		if err != nil {
			t.Fatal(err)
		}
		actor, _ := uuid.Parse(actorValue.String())
		testAssetLifecycle(t, ctx, company, otherCompany, actor)
		t.Run("HTTP storage handoff", func(t *testing.T) {
			testAssetHTTPContract(t, ctx, tenancy.TenantContext{CompanyID: company, UserID: actor, Role: "owner"})
		})
	})
	t.Run("dead letter replay with Redis", func(t *testing.T) {
		redisAddr := os.Getenv("BM_TEST_REDIS_ADDR")
		if redisAddr == "" {
			t.Skip("BM_TEST_REDIS_ADDR is not configured")
		}
		var event platformqueue.Event
		if err := g.DB().Model("outbox_events").Where("event_type", "platform.schedule.probe").Limit(1).Scan(&event); err != nil {
			t.Fatal(err)
		}
		client := asynq.NewClient(asynq.RedisClientOpt{Addr: redisAddr})
		defer client.Close()
		inspector := asynq.NewInspector(asynq.RedisClientOpt{Addr: redisAddr})
		defer inspector.Close()
		publisher := platformqueue.AsynqPublisher{Client: client}
		if err := publisher.Publish(ctx, event); err != nil {
			t.Fatal(err)
		}
		if err := publisher.Publish(ctx, event); err != nil {
			t.Fatalf("duplicate enqueue: %v", err)
		}
		if err := inspector.ArchiveTask("default", event.ID.String()); err != nil {
			t.Fatal(err)
		}
		payload, _ := json.Marshal(event)
		if err := platformqueue.RecordTaskFailure(ctx, asynq.NewTask(platformqueue.OutboxTaskType, payload), asynq.SkipRetry); err != nil {
			t.Fatal(err)
		}
		actorValue, err := g.DB().Model("users").Where("username", "legacy-admin").Value("id")
		if err != nil {
			t.Fatal(err)
		}
		actor, _ := uuid.Parse(actorValue.String())
		tenant := tenancy.TenantContext{CompanyID: event.CompanyID, UserID: actor, Role: "owner"}
		if err := platformqueue.Replay(ctx, tenancy.TenantContext{CompanyID: otherCompany, UserID: actor, Role: "owner"}, event.ID); err == nil {
			t.Fatal("cross-tenant replay allowed")
		}
		if err := platformqueue.Replay(ctx, tenant, event.ID); err != nil {
			t.Fatal(err)
		}
		if err := platformqueue.Replay(ctx, tenant, event.ID); err == nil {
			t.Fatal("duplicate replay accepted")
		}
		calls := 0
		processor := platformqueue.Processor{Consumer: "replay-test", Handlers: map[string]platformqueue.Handler{"platform.schedule.probe": func(_ context.Context, command platformqueue.Event) error {
			calls++
			if string(command.Payload) != "{}" {
				return fmt.Errorf("worker trusted Redis command content")
			}
			return nil
		}}}
		if err := processor.HandleTask(ctx, asynq.NewTask(platformqueue.OutboxTaskType, payload)); err != nil {
			t.Fatal(err)
		}
		if calls != 0 {
			t.Fatal("stale archived generation executed")
		}
		if err := g.DB().Model("outbox_events").Where("id", event.ID).Scan(&event); err != nil {
			t.Fatal(err)
		}
		if event.DispatchGeneration != 1 {
			t.Fatalf("replay generation=%d", event.DispatchGeneration)
		}
		if err := publisher.Publish(ctx, event); err != nil {
			t.Fatal(err)
		}
		if _, err := inspector.GetTaskInfo("default", event.ID.String()+":1"); err != nil {
			t.Fatalf("new generation collided with archive: %v", err)
		}
		event.Payload = json.RawMessage(`{"tampered":true}`)
		payload, _ = json.Marshal(event)
		for i := 0; i < 2; i++ {
			if err := processor.HandleTask(ctx, asynq.NewTask(platformqueue.OutboxTaskType, payload)); err != nil {
				t.Fatal(err)
			}
		}
		if calls != 1 {
			t.Fatalf("completed event executed %d times", calls)
		}
		status, err := g.DB().Model("marketing_job_runs").Where("outbox_event_id", event.ID).Value("status")
		if err != nil || status.String() != "succeeded" {
			t.Fatalf("job status=%v err=%v", status, err)
		}
	})
}

func envOr(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
