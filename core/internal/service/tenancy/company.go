package tenancy

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/google/uuid"
)

var slugPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

type Company struct {
	ID               uuid.UUID `json:"id"`
	Name             string    `json:"name"`
	Slug             string    `json:"slug"`
	Status           string    `json:"status"`
	Timezone         string    `json:"timezone"`
	Locale           string    `json:"locale"`
	MembershipID     uuid.UUID `json:"membership_id"`
	Role             string    `json:"role"`
	MembershipStatus string    `json:"membership_status"`
	CreatedAt        time.Time `json:"created_at"`
}

type User struct {
	ID                 uuid.UUID `json:"id"`
	LegacyAccountID    int64     `json:"legacy_account_id"`
	Username           string    `json:"username"`
	MembershipVersion  int64     `json:"membership_version"`
	PlatformSuperAdmin bool      `json:"platform_super_admin"`
}

func UserForAccount(ctx context.Context, accountID int64) (User, error) {
	var user User
	err := g.DB().Model("users").Where("legacy_account_id", accountID).Scan(&user)
	if err != nil {
		return User{}, fmt.Errorf("resolve SaaS user: %w", err)
	}
	if user.ID == uuid.Nil {
		return User{}, fmt.Errorf("resolve SaaS user: account %d has not been migrated", accountID)
	}
	return user, nil
}

func UserByID(ctx context.Context, userID uuid.UUID) (User, error) {
	var user User
	err := g.DB().Model("users").Where("id", userID).WhereNull("deleted_at").Scan(&user)
	if err != nil {
		return User{}, fmt.Errorf("resolve SaaS user: %w", err)
	}
	if user.ID == uuid.Nil {
		return User{}, fmt.Errorf("SaaS user not found")
	}
	return user, nil
}

func ListCompanies(ctx context.Context, userID uuid.UUID) ([]Company, error) {
	companies := make([]Company, 0)
	err := g.DB().Model("companies c").
		InnerJoin("company_members cm", "cm.company_id = c.id").
		Where("cm.user_id", userID).
		Where("cm.status", "active").
		WhereNull("c.deleted_at").
		Fields("c.id,c.name,c.slug,c.status,c.timezone,c.locale,c.created_at,cm.id membership_id,cm.role,cm.status membership_status").
		Order("c.created_at ASC").
		Scan(&companies)
	if err != nil {
		return nil, fmt.Errorf("list companies: %w", err)
	}
	return companies, nil
}

func Membership(ctx context.Context, userID, companyID uuid.UUID) (TenantContext, error) {
	var row struct {
		MembershipID      uuid.UUID `json:"membership_id"`
		Role              string    `json:"role"`
		UserID            uuid.UUID `json:"user_id"`
		MembershipVersion int64     `json:"membership_version"`
		SuperAdmin        bool      `json:"platform_super_admin"`
	}
	err := g.DB().Model("company_members cm").
		InnerJoin("users u", "u.id = cm.user_id").
		InnerJoin("companies c", "c.id = cm.company_id").
		Where("cm.user_id", userID).
		Where("cm.company_id", companyID).
		Where("cm.status", "active").
		Where("c.status IN (?)", g.Slice{"trial", "active", "past_due"}).
		Fields("cm.id membership_id,cm.role,u.id user_id,u.membership_version,u.platform_super_admin").
		Scan(&row)
	if err != nil {
		return TenantContext{}, fmt.Errorf("resolve company membership: %w", err)
	}
	if row.MembershipID == uuid.Nil {
		return TenantContext{}, ErrMembershipNotFound
	}
	return TenantContext{
		UserID: row.UserID, CompanyID: companyID, MembershipID: row.MembershipID,
		Role: row.Role, MembershipVersion: row.MembershipVersion, PlatformSuperAdmin: row.SuperAdmin,
	}, nil
}

func DefaultMembership(ctx context.Context, accountID int64) (TenantContext, error) {
	user, err := UserForAccount(ctx, accountID)
	if err != nil {
		return TenantContext{}, err
	}
	companies, err := ListCompanies(ctx, user.ID)
	if err != nil || len(companies) == 0 {
		return TenantContext{}, ErrMembershipNotFound
	}
	tenant, err := Membership(ctx, user.ID, companies[0].ID)
	if err != nil {
		return TenantContext{}, err
	}
	tenant.AccountID = accountID
	return tenant, nil
}

func CreateCompany(ctx context.Context, actor User, name, slug, timezone, locale string) (Company, error) {
	name = strings.TrimSpace(name)
	slug = strings.ToLower(strings.TrimSpace(slug))
	if name == "" || len(name) > 160 {
		return Company{}, fmt.Errorf("company name must contain 1 to 160 characters")
	}
	if !slugPattern.MatchString(slug) || len(slug) > 80 {
		return Company{}, fmt.Errorf("slug must contain lowercase letters, numbers, and single hyphens")
	}
	if timezone == "" {
		timezone = "UTC"
	}
	if locale == "" {
		locale = "en"
	}

	companyID, membershipID := uuid.New(), uuid.New()
	err := g.DB().Transaction(ctx, func(_ context.Context, tx gdb.TX) error {
		if _, err := tx.Exec("SELECT set_config('app.company_id', ?, true)", companyID.String()); err != nil {
			return err
		}
		if _, err := tx.Model("companies").Data(g.Map{
			"id": companyID, "name": name, "slug": slug, "status": "trial",
			"timezone": timezone, "locale": locale, "created_by": actor.ID,
		}).Insert(); err != nil {
			return fmt.Errorf("create company: %w", err)
		}
		if _, err := tx.Model("company_members").Data(g.Map{
			"id": membershipID, "company_id": companyID, "user_id": actor.ID,
			"role": "owner", "status": "active", "joined_at": time.Now().UTC(),
		}).Insert(); err != nil {
			return fmt.Errorf("create owner membership: %w", err)
		}
		if _, err := tx.Model("company_settings").Data(g.Map{"company_id": companyID}).Insert(); err != nil {
			return fmt.Errorf("create company settings: %w", err)
		}
		if _, err := tx.Exec(`INSERT INTO company_subscriptions
            (company_id, plan_id, status, trial_ends_at, current_period_start, current_period_end)
            SELECT ?, id, 'trialing', NOW() + INTERVAL '14 days', NOW(), NOW() + INTERVAL '14 days'
            FROM subscription_plans WHERE code = 'trial'`, companyID); err != nil {
			return fmt.Errorf("create trial subscription: %w", err)
		}
		_, err := tx.Model("audit_logs").Data(g.Map{
			"company_id": companyID, "actor_user_id": actor.ID, "action": "company.created",
			"resource_type": "company", "resource_id": companyID.String(),
			"metadata": g.Map{"name": name, "slug": slug},
		}).Insert()
		return err
	})
	if err != nil {
		return Company{}, err
	}
	return Company{ID: companyID, Name: name, Slug: slug, Status: "trial", Timezone: timezone, Locale: locale, MembershipID: membershipID, Role: "owner", MembershipStatus: "active", CreatedAt: time.Now().UTC()}, nil
}
