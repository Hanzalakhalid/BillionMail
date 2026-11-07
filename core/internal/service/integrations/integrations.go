package integrations

import (
	"context"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"billionmail-core/internal/service/secrets"
	"billionmail-core/internal/service/tenancy"
	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/google/uuid"
)

// Connection exposes metadata only. Credentials never leave the service layer.
type Connection struct {
	ID             uuid.UUID       `json:"id"`
	Provider       string          `json:"provider"`
	Capability     string          `json:"capability"`
	Name           string          `json:"name"`
	Status         string          `json:"status"`
	Configuration  json.RawMessage `json:"configuration"`
	Capabilities   json.RawMessage `json:"capabilities"`
	LastVerifiedAt *time.Time      `json:"last_verified_at"`
	LastError      string          `json:"last_error"`
	CreatedAt      time.Time       `json:"created_at"`
	UpdatedAt      time.Time       `json:"updated_at"`
}

type Input struct {
	Provider      string          `json:"provider"`
	Capability    string          `json:"capability"`
	Name          string          `json:"name"`
	Configuration json.RawMessage `json:"configuration"`
	Secret        string          `json:"secret"`
}

func Validate(input Input) error {
	input.Provider = strings.TrimSpace(input.Provider)
	input.Capability = strings.TrimSpace(input.Capability)
	input.Name = strings.TrimSpace(input.Name)
	if input.Provider == "" || len(input.Provider) > 80 || input.Capability == "" || len(input.Capability) > 80 || input.Name == "" || len(input.Name) > 160 || input.Secret == "" {
		return errors.New("provider, capability, name, and secret are required within their length limits")
	}
	if len(input.Configuration) == 0 {
		input.Configuration = json.RawMessage(`{}`)
	}
	var config map[string]any
	if err := json.Unmarshal(input.Configuration, &config); err != nil || config == nil {
		return errors.New("configuration must be a JSON object")
	}
	return validateConfiguration(config)
}

func validateConfiguration(config map[string]any) error {
	for key, value := range config {
		lower := strings.ToLower(key)
		if strings.Contains(lower, "secret") || strings.Contains(lower, "token") || strings.Contains(lower, "password") || strings.Contains(lower, "api_key") || strings.Contains(lower, "apikey") || strings.Contains(lower, "credential") || strings.Contains(lower, "private_key") {
			return fmt.Errorf("configuration field %q may contain a credential; use secret", key)
		}
		switch nested := value.(type) {
		case map[string]any:
			if err := validateConfiguration(nested); err != nil {
				return err
			}
		case []any:
			for _, item := range nested {
				if child, ok := item.(map[string]any); ok {
					if err := validateConfiguration(child); err != nil {
						return err
					}
				}
			}
		}
	}
	return nil
}

func List(ctx context.Context, tenant tenancy.TenantContext) ([]Connection, error) {
	if err := tenancy.Authorize(tenant, "integration.read"); err != nil {
		return nil, err
	}
	result := make([]Connection, 0)
	err := g.DB().Transaction(ctx, func(_ context.Context, tx gdb.TX) error {
		if _, err := tx.Exec("SELECT set_config('app.company_id', ?, true)", tenant.CompanyID.String()); err != nil {
			return err
		}
		return tx.Model("integration_connections").Fields("id,provider,capability,name,status,configuration,capabilities,last_verified_at,last_error,created_at,updated_at").Where("company_id", tenant.CompanyID).WhereNull("deleted_at").OrderDesc("created_at").Scan(&result)
	})
	return result, err
}

func Create(ctx context.Context, tenant tenancy.TenantContext, store secrets.EnvelopeStore, input Input) (uuid.UUID, error) {
	if err := tenancy.Authorize(tenant, "integration.manage"); err != nil {
		return uuid.Nil, err
	}
	if store == nil {
		return uuid.Nil, errors.New("credential encryption is unavailable")
	}
	if err := Validate(input); err != nil {
		return uuid.Nil, err
	}
	envelope, err := store.Encrypt(ctx, tenant.CompanyID, []byte(input.Secret))
	if err != nil {
		return uuid.Nil, err
	}
	id := uuid.New()
	err = g.DB().Transaction(ctx, func(_ context.Context, tx gdb.TX) error {
		if _, err := tx.Exec("SELECT set_config('app.company_id', ?, true)", tenant.CompanyID.String()); err != nil {
			return err
		}
		if _, err := tx.Model("integration_connections").Data(g.Map{"id": id, "company_id": tenant.CompanyID, "provider": strings.TrimSpace(input.Provider), "capability": strings.TrimSpace(input.Capability), "name": strings.TrimSpace(input.Name), "configuration": input.Configuration, "created_by": tenant.UserID}).Insert(); err != nil {
			return err
		}
		if err := insertCredential(tx, tenant, id, 1, envelope); err != nil {
			return err
		}
		_, err := tx.Model("audit_logs").Data(g.Map{"company_id": tenant.CompanyID, "actor_user_id": tenant.UserID, "action": "integration.created", "resource_type": "integration_connection", "resource_id": id.String(), "metadata": g.Map{"provider": input.Provider, "capability": input.Capability}}).Insert()
		return err
	})
	return id, err
}

func Rotate(ctx context.Context, tenant tenancy.TenantContext, store secrets.EnvelopeStore, id uuid.UUID, secret string) error {
	if err := tenancy.Authorize(tenant, "integration.manage"); err != nil {
		return err
	}
	if store == nil || id == uuid.Nil || secret == "" {
		return errors.New("connection and new credential are required")
	}
	envelope, err := store.Encrypt(ctx, tenant.CompanyID, []byte(secret))
	if err != nil {
		return err
	}
	return g.DB().Transaction(ctx, func(_ context.Context, tx gdb.TX) error {
		if _, err := tx.Exec("SELECT set_config('app.company_id', ?, true)", tenant.CompanyID.String()); err != nil {
			return err
		}
		var exists struct {
			ID uuid.UUID `json:"id"`
		}
		if err := tx.Model("integration_connections").Fields("id").Where("id", id).Where("company_id", tenant.CompanyID).WhereNull("deleted_at").LockUpdate().Scan(&exists); err != nil {
			return err
		}
		if exists.ID == uuid.Nil {
			return sql.ErrNoRows
		}
		version, err := tx.Model("integration_credential_versions").Where("company_id", tenant.CompanyID).Where("connection_id", id).Max("version")
		if err != nil {
			return err
		}
		now := time.Now().UTC()
		if _, err = tx.Model("integration_credential_versions").Data(g.Map{"revoked_at": now}).Where("company_id", tenant.CompanyID).Where("connection_id", id).WhereNull("revoked_at").Update(); err != nil {
			return err
		}
		if err = insertCredential(tx, tenant, id, int(version)+1, envelope); err != nil {
			return err
		}
		if _, err = tx.Model("integration_connections").Data(g.Map{"status": "unverified", "last_verified_at": nil, "last_error": nil, "updated_at": now}).Where("id", id).Where("company_id", tenant.CompanyID).Update(); err != nil {
			return err
		}
		_, err = tx.Model("audit_logs").Data(g.Map{"company_id": tenant.CompanyID, "actor_user_id": tenant.UserID, "action": "integration.credential.rotated", "resource_type": "integration_connection", "resource_id": id.String()}).Insert()
		return err
	})
}

func Disconnect(ctx context.Context, tenant tenancy.TenantContext, id uuid.UUID) error {
	if err := tenancy.Authorize(tenant, "integration.manage"); err != nil {
		return err
	}
	if id == uuid.Nil {
		return errors.New("connection is required")
	}
	return g.DB().Transaction(ctx, func(_ context.Context, tx gdb.TX) error {
		if _, err := tx.Exec("SELECT set_config('app.company_id', ?, true)", tenant.CompanyID.String()); err != nil {
			return err
		}
		var exists struct {
			ID uuid.UUID `json:"id"`
		}
		if err := tx.Model("integration_connections").Fields("id").Where("id", id).Where("company_id", tenant.CompanyID).WhereNull("deleted_at").LockUpdate().Scan(&exists); err != nil {
			return err
		}
		if exists.ID == uuid.Nil {
			return sql.ErrNoRows
		}
		now := time.Now().UTC()
		if _, err := tx.Model("integration_credential_versions").Data(g.Map{"revoked_at": now}).Where("company_id", tenant.CompanyID).Where("connection_id", id).WhereNull("revoked_at").Update(); err != nil {
			return err
		}
		if _, err := tx.Model("integration_connections").Data(g.Map{"status": "disconnected", "deleted_at": now, "updated_at": now}).Where("company_id", tenant.CompanyID).Where("id", id).Update(); err != nil {
			return err
		}
		_, err := tx.Model("audit_logs").Data(g.Map{"company_id": tenant.CompanyID, "actor_user_id": tenant.UserID, "action": "integration.disconnected", "resource_type": "integration_connection", "resource_id": id.String()}).Insert()
		return err
	})
}

// Credential is for server-side adapters only; never return it from an HTTP handler.
func Credential(ctx context.Context, companyID, id uuid.UUID, store secrets.EnvelopeStore) ([]byte, error) {
	if companyID == uuid.Nil || id == uuid.Nil || store == nil {
		return nil, errors.New("company, connection, and credential store are required")
	}
	var envelope secrets.Envelope
	err := g.DB().Transaction(ctx, func(_ context.Context, tx gdb.TX) error {
		if _, err := tx.Exec("SELECT set_config('app.company_id', ?, true)", companyID.String()); err != nil {
			return err
		}
		var row struct {
			Ciphertext       string `json:"ciphertext"`
			EncryptedDataKey string `json:"encrypted_data_key"`
			KeyARN           string `json:"key_arn"`
		}
		if err := tx.Model("integration_credential_versions c").InnerJoin("integration_connections i", "i.id=c.connection_id AND i.company_id=c.company_id").Fields("encode(c.ciphertext,'hex') ciphertext,encode(c.encrypted_data_key,'hex') encrypted_data_key,c.key_arn").Where("c.company_id", companyID).Where("c.connection_id", id).WhereNull("c.revoked_at").WhereNull("i.deleted_at").Limit(1).Scan(&row); err != nil {
			return err
		}
		if row.Ciphertext == "" {
			return sql.ErrNoRows
		}
		value, err := hex.DecodeString(row.Ciphertext)
		if err != nil {
			return err
		}
		key, err := hex.DecodeString(row.EncryptedDataKey)
		if err != nil {
			return err
		}
		envelope = secrets.Envelope{Ciphertext: value, EncryptedDataKey: key, KeyARN: row.KeyARN}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return store.Decrypt(ctx, companyID, envelope)
}

func insertCredential(tx gdb.TX, tenant tenancy.TenantContext, id uuid.UUID, version int, envelope secrets.Envelope) error {
	_, err := tx.Model("integration_credential_versions").Data(g.Map{"id": uuid.New(), "company_id": tenant.CompanyID, "connection_id": id, "version": version, "ciphertext": gdb.Raw("decode('" + hex.EncodeToString(envelope.Ciphertext) + "','hex')"), "encrypted_data_key": gdb.Raw("decode('" + hex.EncodeToString(envelope.EncryptedDataKey) + "','hex')"), "key_arn": envelope.KeyARN, "created_by": tenant.UserID}).Insert()
	return err
}
