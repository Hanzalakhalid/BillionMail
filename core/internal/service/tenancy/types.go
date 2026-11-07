package tenancy

import (
	"context"
	"errors"

	"github.com/google/uuid"
)

type contextKey string

const tenantContextKey contextKey = "tenant-context"

var (
	ErrTenantContextMissing = errors.New("tenant context is required")
	ErrMembershipNotFound   = errors.New("active company membership not found")
	ErrPermissionDenied     = errors.New("insufficient company permission")
)

// TenantContext is the mandatory identity carried by tenant-owned use cases.
type TenantContext struct {
	AccountID          int64
	UserID             uuid.UUID
	CompanyID          uuid.UUID
	MembershipID       uuid.UUID
	Role               string
	MembershipVersion  int64
	PlatformSuperAdmin bool
}

func WithContext(ctx context.Context, tenant TenantContext) context.Context {
	return context.WithValue(ctx, tenantContextKey, tenant)
}

func FromContext(ctx context.Context) (TenantContext, bool) {
	tenant, ok := ctx.Value(tenantContextKey).(TenantContext)
	return tenant, ok
}

func Require(ctx context.Context) (TenantContext, error) {
	tenant, ok := FromContext(ctx)
	if !ok || tenant.CompanyID == uuid.Nil || tenant.UserID == uuid.Nil {
		return TenantContext{}, ErrTenantContextMissing
	}
	return tenant, nil
}

type PermissionSet map[string]struct{}

func (p PermissionSet) Allows(permission string) bool {
	_, ok := p[permission]
	return ok
}

var rolePermissions = map[string][]string{
	"owner":   {"*"},
	"admin":   {"company.read", "company.update", "member.read", "member.manage", "channel.*", "flow.*", "contact.*", "conversation.*", "integration.*", "asset.*", "business.*", "product.*", "project.*", "job.manage", "analytics.read", "audit.read", "usage.read"},
	"manager": {"company.read", "member.read", "channel.read", "flow.*", "contact.*", "conversation.*", "integration.read", "asset.*", "business.*", "product.*", "project.*", "analytics.read", "usage.read"},
	"agent":   {"company.read", "contact.read", "contact.update", "conversation.read", "conversation.reply", "conversation.assign"},
	"viewer":  {"company.read", "channel.read", "flow.read", "contact.read", "conversation.read", "asset.read", "business.read", "product.read", "project.read", "analytics.read", "usage.read"},
}

func PermissionsForRole(role string) PermissionSet {
	set := PermissionSet{}
	for _, permission := range rolePermissions[role] {
		set[permission] = struct{}{}
	}
	return set
}

func Authorize(tenant TenantContext, permission string) error {
	if tenant.PlatformSuperAdmin || tenant.Role == "owner" {
		return nil
	}
	permissions := PermissionsForRole(tenant.Role)
	if permissions.Allows(permission) {
		return nil
	}
	for candidate := range permissions {
		if len(candidate) > 2 && candidate[len(candidate)-2:] == ".*" && len(permission) >= len(candidate)-1 && permission[:len(candidate)-1] == candidate[:len(candidate)-1] {
			return nil
		}
	}
	return ErrPermissionDenied
}
