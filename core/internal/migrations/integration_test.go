package migrations

import (
	"billionmail-core/internal/service/identity"
	platformqueue "billionmail-core/internal/service/queue"
	"billionmail-core/internal/service/secrets"
	"billionmail-core/internal/service/tenancy"
	"billionmail-core/internal/service/whatsappchannel"
	"billionmail-core/internal/service/whatsappwebhook"
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	_ "github.com/gogf/gf/contrib/drivers/pgsql/v2"
	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/google/uuid"
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
}

func envOr(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
