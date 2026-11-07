package identity

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

var usernamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{2,63}$`)
var slugCharacters = regexp.MustCompile(`[^a-z0-9]+`)

type Registration struct {
	Username, Email, Password, CompanyName, Timezone, Locale string
}

func BuildActionToken(tokenID uuid.UUID, purpose, secret string) (string, []byte, error) {
	if tokenID == uuid.Nil || purpose == "" || len(secret) < 32 {
		return "", nil, fmt.Errorf("token ID, purpose, and a secret of at least 32 characters are required")
	}
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(purpose))
	_, _ = mac.Write(tokenID[:])
	raw := append(tokenID[:], mac.Sum(nil)...)
	token := base64.RawURLEncoding.EncodeToString(raw)
	hash := sha256.Sum256([]byte(token))
	return token, hash[:], nil
}

func Register(ctx context.Context, input Registration, tokenSecret, publicURL string) (uuid.UUID, error) {
	input.Username = strings.TrimSpace(input.Username)
	input.Email = strings.ToLower(strings.TrimSpace(input.Email))
	input.CompanyName = strings.TrimSpace(input.CompanyName)
	if !usernamePattern.MatchString(input.Username) {
		return uuid.Nil, fmt.Errorf("username must contain 3 to 64 letters, numbers, dots, underscores, or hyphens")
	}
	if !strings.Contains(input.Email, "@") || len(input.Email) > 320 {
		return uuid.Nil, fmt.Errorf("valid email is required")
	}
	if err := validatePassword(input.Password); err != nil {
		return uuid.Nil, err
	}
	if input.CompanyName == "" || len(input.CompanyName) > 160 {
		return uuid.Nil, fmt.Errorf("company name must contain 1 to 160 characters")
	}
	if input.Timezone == "" {
		input.Timezone = "UTC"
	}
	if input.Locale == "" {
		input.Locale = "en"
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		return uuid.Nil, err
	}
	userID, companyID, membershipID, verificationID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	_, verificationHash, err := BuildActionToken(verificationID, "verify-email", tokenSecret)
	if err != nil {
		return uuid.Nil, err
	}
	slug := slugify(input.CompanyName) + "-" + strings.ToLower(companyID.String()[:8])

	err = g.DB().Transaction(ctx, func(_ context.Context, tx gdb.TX) error {
		result, txErr := tx.Model("account").Data(g.Map{
			"username": input.Username, "password": string(hash), "email": input.Email,
			"status": 0, "language": input.Locale, "create_time": time.Now().Unix(), "update_time": time.Now().Unix(),
		}).Insert()
		if txErr != nil {
			return fmt.Errorf("create account: %w", txErr)
		}
		accountID, txErr := result.LastInsertId()
		if txErr != nil {
			return txErr
		}
		if _, txErr = tx.Model("users").Data(g.Map{
			"id": userID, "legacy_account_id": accountID, "username": input.Username,
			"email": input.Email, "password_hash": string(hash), "status": "pending",
		}).Insert(); txErr != nil {
			return fmt.Errorf("create user: %w", txErr)
		}
		if _, txErr = tx.Model("companies").Data(g.Map{
			"id": companyID, "name": input.CompanyName, "slug": slug, "status": "trial",
			"timezone": input.Timezone, "locale": input.Locale, "created_by": userID,
		}).Insert(); txErr != nil {
			return fmt.Errorf("create company: %w", txErr)
		}
		if _, txErr = tx.Exec("SELECT set_config('app.company_id', ?, true)", companyID.String()); txErr != nil {
			return txErr
		}
		if _, txErr = tx.Model("company_members").Data(g.Map{
			"id": membershipID, "company_id": companyID, "user_id": userID, "role": "owner", "status": "active", "joined_at": time.Now().UTC(),
		}).Insert(); txErr != nil {
			return txErr
		}
		if _, txErr = tx.Model("company_settings").Data(g.Map{"company_id": companyID}).Insert(); txErr != nil {
			return txErr
		}
		if _, txErr = tx.Exec(`INSERT INTO company_subscriptions(company_id,plan_id,status,trial_ends_at,current_period_start,current_period_end)
            SELECT ?,id,'trialing',NOW()+INTERVAL '14 days',NOW(),NOW()+INTERVAL '14 days' FROM subscription_plans WHERE code='trial'`, companyID); txErr != nil {
			return txErr
		}
		if _, txErr = tx.Model("email_verification_tokens").Data(g.Map{
			"id": verificationID, "user_id": userID, "token_hash": gdb.Raw("decode('" + hex.EncodeToString(verificationHash) + "','hex')"), "expires_at": time.Now().UTC().Add(24 * time.Hour),
		}).Insert(); txErr != nil {
			return txErr
		}
		payload, _ := json.Marshal(g.Map{"token_id": verificationID, "purpose": "verify-email", "email": input.Email, "public_url": publicURL})
		if _, txErr = tx.Model("outbox_events").Data(g.Map{
			"id": uuid.New(), "company_id": companyID, "aggregate_type": "user", "aggregate_id": userID.String(),
			"event_type": "identity.email_verification.requested", "payload": payload, "idempotency_key": verificationID.String(),
		}).Insert(); txErr != nil {
			return txErr
		}
		return nil
	})
	return userID, err
}

func VerifyEmail(ctx context.Context, token string) error {
	hash := sha256.Sum256([]byte(strings.TrimSpace(token)))
	return g.DB().Transaction(ctx, func(_ context.Context, tx gdb.TX) error {
		var record struct {
			ID        uuid.UUID  `json:"id"`
			UserID    uuid.UUID  `json:"user_id"`
			ExpiresAt time.Time  `json:"expires_at"`
			UsedAt    *time.Time `json:"used_at"`
		}
		if err := tx.Model("email_verification_tokens").Where("token_hash = decode(?, 'hex')", hex.EncodeToString(hash[:])).LockUpdate().Scan(&record); err != nil {
			return fmt.Errorf("load email verification token: %w", err)
		}
		if record.ID == uuid.Nil || record.UsedAt != nil || time.Now().UTC().After(record.ExpiresAt) {
			return fmt.Errorf("verification token is invalid or expired")
		}
		now := time.Now().UTC()
		var user struct {
			LegacyAccountID int64 `json:"legacy_account_id"`
		}
		if err := tx.Model("users").Fields("legacy_account_id").Where("id", record.UserID).Scan(&user); err != nil {
			return err
		}
		if _, err := tx.Model("users").Data(g.Map{"email_verified_at": now, "status": "active", "updated_at": now}).Where("id", record.UserID).Update(); err != nil {
			return err
		}
		if _, err := tx.Model("account").Data(g.Map{"status": 1, "update_time": now.Unix()}).Where("account_id", user.LegacyAccountID).Update(); err != nil {
			return err
		}
		_, err := tx.Model("email_verification_tokens").Data(g.Map{"used_at": now}).Where("id", record.ID).Update()
		return err
	})
}

func RequestPasswordReset(ctx context.Context, email, tokenSecret, publicURL string) error {
	email = strings.ToLower(strings.TrimSpace(email))
	var user struct {
		ID    uuid.UUID `json:"id"`
		Email string    `json:"email"`
	}
	if err := g.DB().Model("users").Where("LOWER(email) = ?", email).Where("status", "active").WhereNull("deleted_at").Scan(&user); err != nil {
		return err
	}
	if user.ID == uuid.Nil {
		return nil
	}
	tokenID := uuid.New()
	_, tokenHash, err := BuildActionToken(tokenID, "reset-password", tokenSecret)
	if err != nil {
		return err
	}
	return g.DB().Transaction(ctx, func(_ context.Context, tx gdb.TX) error {
		var membership struct {
			CompanyID uuid.UUID `json:"company_id"`
		}
		if err := tx.Model("company_members").Fields("company_id").Where("user_id", user.ID).Where("status", "active").OrderAsc("created_at").Limit(1).Scan(&membership); err != nil {
			return err
		}
		companyID := membership.CompanyID
		if companyID == uuid.Nil {
			return fmt.Errorf("active company membership is required")
		}
		if _, err := tx.Exec("SELECT set_config('app.company_id', ?, true)", companyID.String()); err != nil {
			return err
		}
		_, _ = tx.Model("password_reset_tokens").Data(g.Map{"used_at": time.Now().UTC()}).Where("user_id", user.ID).WhereNull("used_at").Update()
		if _, err := tx.Model("password_reset_tokens").Data(g.Map{"id": tokenID, "user_id": user.ID, "token_hash": gdb.Raw("decode('" + hex.EncodeToString(tokenHash) + "','hex')"), "expires_at": time.Now().UTC().Add(time.Hour)}).Insert(); err != nil {
			return err
		}
		payload, _ := json.Marshal(g.Map{"token_id": tokenID, "purpose": "reset-password", "email": user.Email, "public_url": publicURL})
		_, err := tx.Model("outbox_events").Data(g.Map{
			"id": uuid.New(), "company_id": companyID, "aggregate_type": "user", "aggregate_id": user.ID.String(),
			"event_type": "identity.password_reset.requested", "payload": payload, "idempotency_key": tokenID.String(),
		}).Insert()
		return err
	})
}

func ResetPassword(ctx context.Context, token, password string) error {
	if err := validatePassword(password); err != nil {
		return err
	}
	hashToken := sha256.Sum256([]byte(strings.TrimSpace(token)))
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	return g.DB().Transaction(ctx, func(_ context.Context, tx gdb.TX) error {
		var record struct {
			ID        uuid.UUID  `json:"id"`
			UserID    uuid.UUID  `json:"user_id"`
			ExpiresAt time.Time  `json:"expires_at"`
			UsedAt    *time.Time `json:"used_at"`
		}
		if err := tx.Model("password_reset_tokens").Where("token_hash = decode(?, 'hex')", hex.EncodeToString(hashToken[:])).LockUpdate().Scan(&record); err != nil {
			return fmt.Errorf("load password reset token: %w", err)
		}
		if record.ID == uuid.Nil || record.UsedAt != nil || time.Now().UTC().After(record.ExpiresAt) {
			return fmt.Errorf("password reset token is invalid or expired")
		}
		var user struct {
			LegacyAccountID int64 `json:"legacy_account_id"`
		}
		if err := tx.Model("users").Where("id", record.UserID).Scan(&user); err != nil {
			return fmt.Errorf("load password reset user: %w", err)
		}
		now := time.Now().UTC()
		if _, err := tx.Model("users").Data(g.Map{"password_hash": string(passwordHash), "membership_version": gdb.Raw("membership_version+1"), "updated_at": now}).Where("id", record.UserID).Update(); err != nil {
			return err
		}
		if _, err := tx.Model("account").Data(g.Map{"password": string(passwordHash), "update_time": now.Unix()}).Where("account_id", user.LegacyAccountID).Update(); err != nil {
			return err
		}
		if _, err := tx.Model("password_reset_tokens").Data(g.Map{"used_at": now}).Where("id", record.ID).Update(); err != nil {
			return err
		}
		_, err := tx.Model("auth_sessions").Data(g.Map{"revoked_at": now, "revoke_reason": "password_reset"}).Where("user_id", record.UserID).WhereNull("revoked_at").Update()
		return err
	})
}

func validatePassword(password string) error {
	if len(password) < 12 || len(password) > 128 {
		return fmt.Errorf("password must contain 12 to 128 characters")
	}
	var upper, lower, digit bool
	for _, value := range password {
		upper = upper || value >= 'A' && value <= 'Z'
		lower = lower || value >= 'a' && value <= 'z'
		digit = digit || value >= '0' && value <= '9'
	}
	if !upper || !lower || !digit {
		return fmt.Errorf("password must contain uppercase, lowercase, and numeric characters")
	}
	return nil
}

func slugify(value string) string {
	value = strings.Trim(slugCharacters.ReplaceAllString(strings.ToLower(value), "-"), "-")
	if value == "" {
		return "workspace"
	}
	if len(value) > 60 {
		value = strings.Trim(value[:60], "-")
	}
	return value
}
