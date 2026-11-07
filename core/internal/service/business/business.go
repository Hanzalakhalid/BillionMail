package business

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/url"
	"strings"
	"time"
	_ "time/tzdata"

	"billionmail-core/internal/service/tenancy"
	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/google/uuid"
)

var ErrConflict = errors.New("profile revision or idempotency key conflicts with an existing change")

type Input struct {
	Name        string `json:"name"`
	Website     string `json:"website"`
	Industry    string `json:"industry"`
	Description string `json:"description"`
	BrandVoice  string `json:"brand_voice"`
	Locale      string `json:"locale"`
	Timezone    string `json:"timezone"`
	Revision    int64  `json:"revision"`
}
type Profile struct {
	Input
	ID        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func Validate(input Input) error {
	if strings.TrimSpace(input.Name) == "" || len(input.Name) > 160 || len(input.Website) > 2048 || len(input.Industry) > 160 || len(input.Description) > 16000 || len(input.BrandVoice) > 8000 || input.Revision < 0 || len(input.Locale) < 2 || len(input.Locale) > 32 {
		return errors.New("valid business name, locale and profile field lengths are required")
	}
	for _, letter := range input.Locale {
		if !((letter >= 'a' && letter <= 'z') || (letter >= 'A' && letter <= 'Z') || (letter >= '0' && letter <= '9') || letter == '-') {
			return errors.New("locale must be a language tag")
		}
	}
	if input.Timezone == "" {
		return errors.New("business timezone is required")
	}
	if _, err := time.LoadLocation(input.Timezone); err != nil {
		return errors.New("business timezone must be an IANA timezone")
	}
	if input.Website != "" {
		parsed, err := url.Parse(input.Website)
		if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Hostname() == "" || parsed.User != nil {
			return errors.New("business website must be an HTTP URL without credentials")
		}
	}
	return nil
}

func Get(ctx context.Context, tenant tenancy.TenantContext) (*Profile, error) {
	if err := tenancy.Authorize(tenant, "business.read"); err != nil {
		return nil, err
	}
	var profile *Profile
	err := g.DB().Transaction(ctx, func(_ context.Context, tx gdb.TX) error {
		if _, err := tx.Exec("SELECT set_config('app.company_id', ?, true)", tenant.CompanyID.String()); err != nil {
			return err
		}
		return tx.Model("marketing_business_profiles").Where("company_id", tenant.CompanyID).WhereNull("deleted_at").Scan(&profile)
	})
	return profile, err
}

// Save uses optimistic concurrency plus transactional response receipts. A
// retried request returns its original result even after a later revision.
func Save(ctx context.Context, tenant tenancy.TenantContext, key string, input Input) (Profile, error) {
	if err := tenancy.Authorize(tenant, "business.manage"); err != nil {
		return Profile{}, err
	}
	input.Name = strings.TrimSpace(input.Name)
	input.Website = strings.TrimSpace(input.Website)
	if err := Validate(input); err != nil {
		return Profile{}, err
	}
	if tenant.CompanyID == uuid.Nil || tenant.UserID == uuid.Nil || key == "" || len(key) > 128 {
		return Profile{}, errors.New("workspace, actor and idempotency key are required")
	}
	encoded, _ := json.Marshal(input)
	digest := sha256.Sum256(encoded)
	hash := hex.EncodeToString(digest[:])
	var result Profile
	err := g.DB().Transaction(ctx, func(_ context.Context, tx gdb.TX) error {
		if _, err := tx.Exec("SELECT set_config('app.company_id', ?, true)", tenant.CompanyID.String()); err != nil {
			return err
		}
		if _, err := tx.Exec("SELECT pg_advisory_xact_lock(hashtextextended(?,0))", tenant.CompanyID.String()+":business-profile"); err != nil {
			return err
		}
		receipts, err := tx.Model("marketing_mutation_receipts").Where("company_id", tenant.CompanyID).Where("operation", "business.save").Where("idempotency_key", key).Count()
		if err != nil {
			return err
		}
		if receipts > 0 {
			var receipt struct {
				RequestHash string          `json:"request_hash"`
				Response    json.RawMessage `json:"response"`
			}
			if err := tx.Model("marketing_mutation_receipts").Where("company_id", tenant.CompanyID).Where("operation", "business.save").Where("idempotency_key", key).Scan(&receipt); err != nil {
				return err
			}
			if receipt.RequestHash != hash {
				return ErrConflict
			}
			return json.Unmarshal(receipt.Response, &result)
		}
		count, err := tx.Model("marketing_business_profiles").Where("company_id", tenant.CompanyID).Count()
		if err != nil {
			return err
		}
		now := time.Now().UTC()
		values := g.Map{"name": input.Name, "website": input.Website, "industry": input.Industry, "description": input.Description, "brand_voice": input.BrandVoice, "locale": input.Locale, "timezone": input.Timezone, "updated_at": now, "updated_by": tenant.UserID}
		if count == 0 {
			if input.Revision != 0 {
				return ErrConflict
			}
			result = Profile{Input: input, ID: uuid.New(), CreatedAt: now, UpdatedAt: now}
			result.Revision = 1
			values["id"] = result.ID
			values["company_id"] = tenant.CompanyID
			values["created_by"] = tenant.UserID
			values["created_at"] = now
			values["revision"] = 1
			if _, err := tx.Model("marketing_business_profiles").Data(values).Insert(); err != nil {
				return err
			}
		} else {
			if err := tx.Model("marketing_business_profiles").Where("company_id", tenant.CompanyID).LockUpdate().Scan(&result); err != nil {
				return err
			}
			if result.Revision != input.Revision {
				return ErrConflict
			}
			values["revision"] = result.Revision + 1
			if _, err := tx.Model("marketing_business_profiles").Where("company_id", tenant.CompanyID).Data(values).Update(); err != nil {
				return err
			}
			revision := result.Revision + 1
			result.Input = input
			result.Revision = revision
			result.UpdatedAt = now
		}
		response, err := json.Marshal(result)
		if err != nil {
			return err
		}
		if _, err := tx.Model("marketing_mutation_receipts").Data(g.Map{"company_id": tenant.CompanyID, "operation": "business.save", "idempotency_key": key, "request_hash": hash, "response": string(response), "created_by": tenant.UserID}).Insert(); err != nil {
			return err
		}
		_, err = tx.Model("audit_logs").Data(g.Map{"company_id": tenant.CompanyID, "actor_user_id": tenant.UserID, "action": "business.profile.saved", "resource_type": "business_profile", "resource_id": result.ID.String(), "metadata": g.Map{"revision": result.Revision}}).Insert()
		return err
	})
	return result, err
}
