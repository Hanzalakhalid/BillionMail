package whatsappchannel

import (
	"context"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"billionmail-core/internal/service/secrets"
	"billionmail-core/internal/service/tenancy"
	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/google/uuid"
	"github.com/lib/pq"
)

const maxPhoneNumbersPerConnection = 100

var ErrChannelUnavailable = errors.New("WhatsApp account or phone number is already connected to another workspace")

type SecretStore interface {
	Encrypt(context.Context, uuid.UUID, []byte) (secrets.Envelope, error)
	Decrypt(context.Context, uuid.UUID, secrets.Envelope) ([]byte, error)
}

type PhoneAsset struct {
	PhoneNumberID      string
	DisplayPhoneNumber string
	VerifiedName       string
	QualityRating      string
	MessagingLimitTier string
}

type ConnectInput struct {
	WABAID         string
	BusinessName   string
	AccessToken    string
	Scopes         []string
	TokenExpiresAt *time.Time
	PhoneNumbers   []PhoneAsset
}

type Connection struct {
	AccountID      uuid.UUID
	PhoneNumberIDs []uuid.UUID
}

type Channel struct {
	ID                 uuid.UUID `json:"id"`
	AccountID          uuid.UUID `json:"account_id"`
	WABAID             string    `json:"waba_id"`
	BusinessName       string    `json:"business_name"`
	PhoneNumberID      string    `json:"phone_number_id"`
	DisplayPhoneNumber string    `json:"display_phone_number"`
	VerifiedName       string    `json:"verified_name"`
	Status             string    `json:"status"`
}

func List(ctx context.Context, tenant tenancy.TenantContext) ([]Channel, error) {
	if err := tenancy.Authorize(tenant, "channel.read"); err != nil {
		return nil, err
	}
	var channels []Channel
	err := g.DB().Transaction(ctx, func(_ context.Context, tx gdb.TX) error {
		if _, err := tx.Exec("SELECT set_config('app.company_id', ?, true)", tenant.CompanyID.String()); err != nil {
			return err
		}
		return tx.Model("whatsapp_phone_numbers p").
			InnerJoin("whatsapp_accounts a", "a.id=p.whatsapp_account_id AND a.company_id=p.company_id").
			Fields("p.id,p.whatsapp_account_id account_id,a.waba_id,a.business_name,p.phone_number_id,p.display_phone_number,p.verified_name,p.status").
			Where("p.company_id", tenant.CompanyID).WhereNull("p.deleted_at").WhereNull("a.deleted_at").
			OrderDesc("p.created_at").Scan(&channels)
	})
	return channels, err
}

// Connect persists discovered Meta assets and a new encrypted credential version atomically.
// A global provider identity can never be reassigned through this use case.
func Connect(ctx context.Context, tenant tenancy.TenantContext, store SecretStore, input ConnectInput) (Connection, error) {
	if tenant.CompanyID == uuid.Nil || tenant.UserID == uuid.Nil {
		return Connection{}, tenancy.ErrTenantContextMissing
	}
	if err := tenancy.Authorize(tenant, "channel.manage"); err != nil {
		return Connection{}, err
	}
	if store == nil {
		return Connection{}, errors.New("secret store is required")
	}
	if err := validateConnect(input); err != nil {
		return Connection{}, err
	}
	input.WABAID = strings.TrimSpace(input.WABAID)
	input.BusinessName = strings.TrimSpace(input.BusinessName)
	envelope, err := store.Encrypt(ctx, tenant.CompanyID, []byte(strings.TrimSpace(input.AccessToken)))
	if err != nil {
		return Connection{}, fmt.Errorf("encrypt Meta credential: %w", err)
	}

	result := Connection{}
	err = g.DB().Transaction(ctx, func(_ context.Context, tx gdb.TX) error {
		if _, txErr := tx.Exec("SELECT set_config('app.company_id', ?, true)", tenant.CompanyID.String()); txErr != nil {
			return txErr
		}
		var account struct {
			ID uuid.UUID `json:"id"`
		}
		if txErr := tx.Model("whatsapp_accounts").Fields("id").
			Where("company_id", tenant.CompanyID).Where("provider", "meta").
			Where("waba_id", input.WABAID).WhereNull("deleted_at").Scan(&account); txErr != nil && !errors.Is(txErr, sql.ErrNoRows) {
			return txErr
		}
		now := time.Now().UTC()
		if account.ID == uuid.Nil {
			account.ID = uuid.New()
			if _, txErr := tx.Model("whatsapp_accounts").Data(g.Map{
				"id": account.ID, "company_id": tenant.CompanyID, "provider": "meta",
				"waba_id": input.WABAID, "business_name": input.BusinessName, "status": "connected",
				"scopes": pq.Array(input.Scopes), "token_expires_at": input.TokenExpiresAt, "last_verified_at": now,
			}).Insert(); txErr != nil {
				return channelConflict(txErr)
			}
		} else {
			if _, txErr := tx.Model("whatsapp_accounts").Data(g.Map{
				"business_name": input.BusinessName, "status": "connected", "scopes": pq.Array(input.Scopes),
				"token_expires_at": input.TokenExpiresAt, "last_verified_at": now, "reconnect_required_at": nil, "updated_at": now,
			}).Where("id", account.ID).Where("company_id", tenant.CompanyID).Update(); txErr != nil {
				return txErr
			}
		}
		result.AccountID = account.ID

		versionValue, txErr := tx.Model("whatsapp_credentials").
			Where("company_id", tenant.CompanyID).Where("whatsapp_account_id", account.ID).
			Where("credential_type", "access_token").Max("credential_version")
		if txErr != nil && !errors.Is(txErr, sql.ErrNoRows) {
			return txErr
		}
		version := int(versionValue) + 1
		if _, txErr = tx.Model("whatsapp_credentials").Data(g.Map{"revoked_at": now}).
			Where("company_id", tenant.CompanyID).Where("whatsapp_account_id", account.ID).
			Where("credential_type", "access_token").WhereNull("revoked_at").Update(); txErr != nil {
			return txErr
		}
		if _, txErr = tx.Model("whatsapp_credentials").Data(g.Map{
			"id": uuid.New(), "company_id": tenant.CompanyID, "whatsapp_account_id": account.ID,
			// Binary values go in as hex so the driver cannot mangle them as text.
			"credential_type": "access_token", "ciphertext": gdb.Raw("decode('" + hex.EncodeToString(envelope.Ciphertext) + "','hex')"),
			"encrypted_data_key": gdb.Raw("decode('" + hex.EncodeToString(envelope.EncryptedDataKey) + "','hex')"), "key_arn": envelope.KeyARN,
			"credential_version": version, "expires_at": input.TokenExpiresAt,
		}).Insert(); txErr != nil {
			return txErr
		}

		for _, phone := range input.PhoneNumbers {
			var existing struct {
				ID uuid.UUID `json:"id"`
			}
			if txErr = tx.Model("whatsapp_phone_numbers").Fields("id").
				Where("company_id", tenant.CompanyID).Where("provider", "meta").
				Where("phone_number_id", phone.PhoneNumberID).WhereNull("deleted_at").Scan(&existing); txErr != nil && !errors.Is(txErr, sql.ErrNoRows) {
				return txErr
			}
			if existing.ID == uuid.Nil {
				existing.ID = uuid.New()
				if _, txErr = tx.Model("whatsapp_phone_numbers").Data(phoneData(tenant.CompanyID, account.ID, existing.ID, phone, now)).Insert(); txErr != nil {
					return channelConflict(txErr)
				}
			} else if _, txErr = tx.Model("whatsapp_phone_numbers").Data(g.Map{
				"whatsapp_account_id": account.ID, "display_phone_number": phone.DisplayPhoneNumber,
				"verified_name": phone.VerifiedName, "quality_rating": phone.QualityRating,
				"messaging_limit_tier": phone.MessagingLimitTier, "status": "active", "updated_at": now,
			}).Where("id", existing.ID).Where("company_id", tenant.CompanyID).Update(); txErr != nil {
				return txErr
			}
			result.PhoneNumberIDs = append(result.PhoneNumberIDs, existing.ID)
		}

		_, txErr = tx.Model("audit_logs").Data(g.Map{
			"company_id": tenant.CompanyID, "actor_user_id": tenant.UserID,
			"action": "whatsapp.account.connected", "resource_type": "whatsapp_account",
			"resource_id": account.ID.String(), "metadata": g.Map{"waba_id": input.WABAID, "phone_count": len(input.PhoneNumbers)},
		}).Insert()
		return txErr
	})
	if err != nil {
		return Connection{}, err
	}
	return result, nil
}

// AccessToken is intentionally an internal worker use case. It returns plaintext only to
// the caller and never exposes credentials through an API response.
func AccessToken(ctx context.Context, companyID, accountID uuid.UUID, store SecretStore) ([]byte, error) {
	if companyID == uuid.Nil || accountID == uuid.Nil || store == nil {
		return nil, errors.New("company, WhatsApp account, and secret store are required")
	}
	var envelope secrets.Envelope
	err := g.DB().Transaction(ctx, func(_ context.Context, tx gdb.TX) error {
		if _, txErr := tx.Exec("SELECT set_config('app.company_id', ?, true)", companyID.String()); txErr != nil {
			return txErr
		}
		// bytea is read as hex text so the driver never reinterprets the binary.
		var credential struct {
			Ciphertext       string `json:"ciphertext"`
			EncryptedDataKey string `json:"encrypted_data_key"`
			KeyARN           string `json:"key_arn"`
		}
		if txErr := tx.Model("whatsapp_credentials").
			Fields("encode(ciphertext,'hex') AS ciphertext, encode(encrypted_data_key,'hex') AS encrypted_data_key, key_arn").
			Where("company_id", companyID).Where("whatsapp_account_id", accountID).
			Where("credential_type", "access_token").WhereNull("revoked_at").
			OrderDesc("credential_version").Limit(1).Scan(&credential); txErr != nil {
			return txErr
		}
		if credential.Ciphertext == "" {
			return errors.New("active WhatsApp credential not found")
		}
		ciphertext, decodeErr := hex.DecodeString(credential.Ciphertext)
		if decodeErr != nil {
			return fmt.Errorf("decode WhatsApp credential: %w", decodeErr)
		}
		dataKey, decodeErr := hex.DecodeString(credential.EncryptedDataKey)
		if decodeErr != nil {
			return fmt.Errorf("decode WhatsApp credential key: %w", decodeErr)
		}
		envelope = secrets.Envelope{Ciphertext: ciphertext, EncryptedDataKey: dataKey, KeyARN: credential.KeyARN}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return store.Decrypt(ctx, companyID, envelope)
}

func validateConnect(input ConnectInput) error {
	input.WABAID = strings.TrimSpace(input.WABAID)
	if input.WABAID == "" || len(input.WABAID) > 128 || strings.TrimSpace(input.AccessToken) == "" {
		return errors.New("WABA ID and access token are required")
	}
	if len(input.PhoneNumbers) == 0 || len(input.PhoneNumbers) > maxPhoneNumbersPerConnection {
		return fmt.Errorf("between 1 and %d phone numbers are required", maxPhoneNumbersPerConnection)
	}
	seen := make(map[string]struct{}, len(input.PhoneNumbers))
	for _, phone := range input.PhoneNumbers {
		phoneID := strings.TrimSpace(phone.PhoneNumberID)
		if phoneID == "" || len(phoneID) > 128 {
			return errors.New("every phone number must have a valid provider ID")
		}
		if _, duplicate := seen[phoneID]; duplicate {
			return errors.New("duplicate phone number provider ID")
		}
		seen[phoneID] = struct{}{}
	}
	if input.TokenExpiresAt != nil && input.TokenExpiresAt.Before(time.Now().UTC()) {
		return errors.New("access token is already expired")
	}
	return nil
}

func phoneData(companyID, accountID, phoneID uuid.UUID, phone PhoneAsset, now time.Time) g.Map {
	return g.Map{
		"id": phoneID, "company_id": companyID, "whatsapp_account_id": accountID, "provider": "meta",
		"phone_number_id": strings.TrimSpace(phone.PhoneNumberID), "display_phone_number": phone.DisplayPhoneNumber,
		"verified_name": phone.VerifiedName, "quality_rating": phone.QualityRating,
		"messaging_limit_tier": phone.MessagingLimitTier, "status": "active", "updated_at": now,
	}
}

func channelConflict(err error) error {
	if strings.Contains(strings.ToLower(err.Error()), "duplicate key") || strings.Contains(strings.ToLower(err.Error()), "unique constraint") {
		return ErrChannelUnavailable
	}
	return err
}
