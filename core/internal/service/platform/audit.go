package platform

import (
	"context"
	"fmt"

	"billionmail-core/internal/service/tenancy"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/google/uuid"
)

type AuditEntry struct {
	Action       string
	ResourceType string
	ResourceID   string
	RequestID    string
	IPAddress    string
	Metadata     g.Map
}

func RecordAudit(ctx context.Context, tenant tenancy.TenantContext, entry AuditEntry) error {
	if tenant.UserID == uuid.Nil || entry.Action == "" || entry.ResourceType == "" {
		return fmt.Errorf("audit actor, action, and resource type are required")
	}
	data := g.Map{
		"company_id": tenant.CompanyID, "actor_user_id": tenant.UserID,
		"action": entry.Action, "resource_type": entry.ResourceType,
		"resource_id": entry.ResourceID, "request_id": entry.RequestID,
		"metadata": entry.Metadata,
	}
	if entry.IPAddress != "" {
		data["ip_address"] = entry.IPAddress
	}
	_, err := g.DB().Model("audit_logs").Data(data).Insert()
	return err
}
