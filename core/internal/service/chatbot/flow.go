package chatbot

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"billionmail-core/internal/service/tenancy"
	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/google/uuid"
)

func List(ctx context.Context, tenant tenancy.TenantContext) ([]Flow, error) {
	if err := tenancy.Authorize(tenant, "flow.read"); err != nil {
		return nil, err
	}
	var flows []Flow
	err := tenantTx(ctx, tenant.CompanyID, func(tx gdb.TX) error {
		return tx.Model("chatbot_flows f").
			LeftJoin("whatsapp_phone_numbers p", "p.id=f.phone_number_id AND p.company_id=f.company_id").
			Fields(`f.id,f.company_id,f.phone_number_id,p.display_phone_number,p.verified_name,
                f.name,f.status,f.created_at,f.updated_at,
                COALESCE((SELECT MAX(version) FROM chatbot_flow_versions v WHERE v.flow_id=f.id AND v.status='draft'),0) draft_version,
                COALESCE((SELECT MAX(version) FROM chatbot_flow_versions v WHERE v.flow_id=f.id AND v.status='published'),0) published_version`).
			Where("f.company_id", tenant.CompanyID).WhereNull("f.deleted_at").OrderDesc("f.updated_at").Scan(&flows)
	})
	return flows, err
}

func Get(ctx context.Context, tenant tenancy.TenantContext, id uuid.UUID) (Flow, error) {
	if err := tenancy.Authorize(tenant, "flow.read"); err != nil {
		return Flow{}, err
	}
	var flow Flow
	err := tenantTx(ctx, tenant.CompanyID, func(tx gdb.TX) error {
		if err := tx.Model("chatbot_flows f").
			LeftJoin("whatsapp_phone_numbers p", "p.id=f.phone_number_id AND p.company_id=f.company_id").
			Fields("f.id,f.company_id,f.phone_number_id,p.display_phone_number,p.verified_name,f.name,f.status,f.created_at,f.updated_at").
			Where("f.id", id).Where("f.company_id", tenant.CompanyID).WhereNull("f.deleted_at").Scan(&flow); err != nil {
			return err
		}
		if flow.ID == uuid.Nil {
			return sql.ErrNoRows
		}
		var version struct {
			Version int             `json:"version"`
			Status  string          `json:"status"`
			Graph   json.RawMessage `json:"graph"`
		}
		if err := tx.Model("chatbot_flow_versions").Fields("version,status,graph").Where("flow_id", id).
			Where("company_id", tenant.CompanyID).OrderAsc("CASE WHEN status='draft' THEN 0 ELSE 1 END").OrderDesc("version").Limit(1).Scan(&version); err != nil {
			return err
		}
		if len(version.Graph) == 0 {
			return errors.New("bot has no version")
		}
		if err := json.Unmarshal(version.Graph, &flow.Graph); err != nil {
			return fmt.Errorf("decode bot graph: %w", err)
		}
		if version.Status == "draft" {
			flow.DraftVersion = version.Version
		} else {
			flow.PublishedVersion = version.Version
		}
		return nil
	})
	return flow, err
}

func Create(ctx context.Context, tenant tenancy.TenantContext, name string, phoneID uuid.UUID, graph Graph) (uuid.UUID, error) {
	if err := tenancy.Authorize(tenant, "flow.manage"); err != nil {
		return uuid.Nil, err
	}
	name = strings.TrimSpace(name)
	if name == "" || len(name) > 160 {
		return uuid.Nil, errors.New("bot name is required and must be at most 160 characters")
	}
	if err := ValidateGraph(graph); err != nil {
		return uuid.Nil, err
	}
	flowID, versionID := uuid.New(), uuid.New()
	raw, _ := json.Marshal(graph)
	err := tenantTx(ctx, tenant.CompanyID, func(tx gdb.TX) error {
		count, err := tx.Model("whatsapp_phone_numbers").Where("id", phoneID).Where("company_id", tenant.CompanyID).Where("status", "active").WhereNull("deleted_at").Count()
		if err != nil {
			return err
		}
		if count != 1 {
			return errors.New("select an active WhatsApp number from this company")
		}
		if _, err = tx.Model("chatbot_flows").Data(g.Map{"id": flowID, "company_id": tenant.CompanyID, "phone_number_id": phoneID, "name": name, "created_by": tenant.UserID}).Insert(); err != nil {
			return err
		}
		if _, err = tx.Model("chatbot_flow_versions").Data(g.Map{"id": versionID, "company_id": tenant.CompanyID, "flow_id": flowID, "version": 1, "status": "draft", "graph": raw, "created_by": tenant.UserID}).Insert(); err != nil {
			return err
		}
		return audit(tx, tenant, "chatbot.created", flowID, g.Map{"name": name})
	})
	return flowID, err
}

func UpdateDraft(ctx context.Context, tenant tenancy.TenantContext, id uuid.UUID, name string, phoneID uuid.UUID, graph Graph) error {
	if err := tenancy.Authorize(tenant, "flow.manage"); err != nil {
		return err
	}
	name = strings.TrimSpace(name)
	if name == "" || len(name) > 160 {
		return errors.New("bot name is required")
	}
	if err := ValidateGraph(graph); err != nil {
		return err
	}
	raw, _ := json.Marshal(graph)
	return tenantTx(ctx, tenant.CompanyID, func(tx gdb.TX) error {
		var flow struct {
			ID uuid.UUID `json:"id"`
		}
		if err := tx.Model("chatbot_flows").Fields("id").Where("id", id).Where("company_id", tenant.CompanyID).WhereNull("deleted_at").LockUpdate().Scan(&flow); err != nil {
			return err
		}
		if flow.ID == uuid.Nil {
			return sql.ErrNoRows
		}
		count, err := tx.Model("whatsapp_phone_numbers").Where("id", phoneID).Where("company_id", tenant.CompanyID).Where("status", "active").Count()
		if err != nil {
			return err
		}
		if count != 1 {
			return errors.New("select an active WhatsApp number from this company")
		}
		var draft struct {
			ID      uuid.UUID `json:"id"`
			Version int       `json:"version"`
		}
		_ = tx.Model("chatbot_flow_versions").Fields("id,version").Where("flow_id", id).Where("status", "draft").Scan(&draft)
		if draft.ID == uuid.Nil {
			maxValue, maxErr := tx.Model("chatbot_flow_versions").Where("flow_id", id).Max("version")
			if maxErr != nil {
				return maxErr
			}
			draft.ID, draft.Version = uuid.New(), int(maxValue)+1
			if _, err = tx.Model("chatbot_flow_versions").Data(g.Map{"id": draft.ID, "company_id": tenant.CompanyID, "flow_id": id, "version": draft.Version, "status": "draft", "graph": raw, "created_by": tenant.UserID}).Insert(); err != nil {
				return err
			}
		} else if _, err = tx.Model("chatbot_flow_versions").Data(g.Map{"graph": raw}).Where("id", draft.ID).Where("company_id", tenant.CompanyID).Update(); err != nil {
			return err
		}
		_, err = tx.Model("chatbot_flows").Data(g.Map{"name": name, "phone_number_id": phoneID, "updated_at": time.Now().UTC()}).Where("id", id).Where("company_id", tenant.CompanyID).Update()
		return err
	})
}

func Publish(ctx context.Context, tenant tenancy.TenantContext, id uuid.UUID) error {
	if err := tenancy.Authorize(tenant, "flow.publish"); err != nil {
		return err
	}
	return tenantTx(ctx, tenant.CompanyID, func(tx gdb.TX) error {
		var flow struct {
			ID            uuid.UUID `json:"id"`
			PhoneNumberID uuid.UUID `json:"phone_number_id"`
		}
		if err := tx.Model("chatbot_flows").Fields("id,phone_number_id").Where("id", id).Where("company_id", tenant.CompanyID).WhereNull("deleted_at").LockUpdate().Scan(&flow); err != nil {
			return err
		}
		if flow.ID == uuid.Nil {
			return sql.ErrNoRows
		}
		var draft struct {
			ID    uuid.UUID       `json:"id"`
			Graph json.RawMessage `json:"graph"`
		}
		if err := tx.Model("chatbot_flow_versions").Fields("id,graph").Where("flow_id", id).Where("company_id", tenant.CompanyID).Where("status", "draft").LockUpdate().Scan(&draft); err != nil {
			return err
		}
		if draft.ID == uuid.Nil {
			return errors.New("save a new draft before publishing")
		}
		var graph Graph
		if err := json.Unmarshal(draft.Graph, &graph); err != nil {
			return err
		}
		if err := ValidateGraph(graph); err != nil {
			return err
		}
		now := time.Now().UTC()
		if _, err := tx.Model("chatbot_flow_versions").Data(g.Map{"status": "published", "published_at": now}).Where("id", draft.ID).Where("status", "draft").Update(); err != nil {
			return err
		}
		if _, err := tx.Model("chatbot_flows").Data(g.Map{"status": "paused", "updated_at": now}).Where("company_id", tenant.CompanyID).Where("phone_number_id", flow.PhoneNumberID).Where("status", "active").WhereNot("id", id).Update(); err != nil {
			return err
		}
		if _, err := tx.Model("chatbot_flows").Data(g.Map{"status": "active", "active_version_id": draft.ID, "updated_at": now}).Where("id", id).Where("company_id", tenant.CompanyID).Update(); err != nil {
			return err
		}
		return audit(tx, tenant, "chatbot.published", id, g.Map{"version_id": draft.ID})
	})
}

func Delete(ctx context.Context, tenant tenancy.TenantContext, id uuid.UUID) error {
	if err := tenancy.Authorize(tenant, "flow.manage"); err != nil {
		return err
	}
	return tenantTx(ctx, tenant.CompanyID, func(tx gdb.TX) error {
		result, err := tx.Model("chatbot_flows").Data(g.Map{"deleted_at": time.Now().UTC(), "status": "paused"}).Where("id", id).Where("company_id", tenant.CompanyID).WhereNull("deleted_at").Update()
		if err != nil {
			return err
		}
		rows, _ := result.RowsAffected()
		if rows == 0 {
			return sql.ErrNoRows
		}
		return audit(tx, tenant, "chatbot.deleted", id, nil)
	})
}

func tenantTx(ctx context.Context, companyID uuid.UUID, fn func(gdb.TX) error) error {
	if companyID == uuid.Nil {
		return tenancy.ErrTenantContextMissing
	}
	return g.DB().Transaction(ctx, func(_ context.Context, tx gdb.TX) error {
		if _, err := tx.Exec("SELECT set_config('app.company_id', ?, true)", companyID.String()); err != nil {
			return err
		}
		return fn(tx)
	})
}

func audit(tx gdb.TX, tenant tenancy.TenantContext, action string, id uuid.UUID, metadata any) error {
	if metadata == nil {
		metadata = g.Map{}
	}
	_, err := tx.Model("audit_logs").Data(g.Map{"company_id": tenant.CompanyID, "actor_user_id": tenant.UserID, "action": action, "resource_type": "chatbot_flow", "resource_id": id.String(), "metadata": metadata}).Insert()
	return err
}
