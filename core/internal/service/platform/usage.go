package platform

import (
	"context"
	"fmt"
	"time"

	"billionmail-core/internal/service/tenancy"
	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

type Entitlements map[string]int64

func (e Entitlements) Allows(metric string, current, requested int64) bool {
	limit, exists := e[metric]
	return exists && (limit < 0 || current+requested <= limit)
}

func IncrementUsage(ctx context.Context, tenant tenancy.TenantContext, metric string, quantity int64, at time.Time) error {
	if metric == "" || quantity <= 0 {
		return fmt.Errorf("usage metric and positive quantity are required")
	}
	periodStart := time.Date(at.UTC().Year(), at.UTC().Month(), 1, 0, 0, 0, 0, time.UTC)
	periodEnd := periodStart.AddDate(0, 1, 0)
	return g.DB().Transaction(ctx, func(_ context.Context, tx gdb.TX) error {
		if _, err := tx.Exec("SELECT set_config('app.company_id', ?, true)", tenant.CompanyID.String()); err != nil {
			return err
		}
		_, err := tx.Exec(`INSERT INTO tenant_usage(company_id,metric,period_start,period_end,quantity)
            VALUES(?,?,?,?,?)
            ON CONFLICT(company_id,metric,period_start)
            DO UPDATE SET quantity=tenant_usage.quantity+EXCLUDED.quantity,updated_at=NOW()`,
			tenant.CompanyID, metric, periodStart, periodEnd, quantity)
		return err
	})
}

func CompanyEntitlements(ctx context.Context, tenant tenancy.TenantContext) (Entitlements, error) {
	var row struct {
		Entitlements Entitlements `json:"entitlements"`
	}
	err := g.DB().Model("company_subscriptions cs").
		InnerJoin("subscription_plans sp", "sp.id=cs.plan_id").
		Where("cs.company_id", tenant.CompanyID).
		WhereIn("cs.status", g.Slice{"trialing", "active", "past_due"}).
		Fields("sp.entitlements").Scan(&row)
	if err != nil {
		return nil, err
	}
	if row.Entitlements == nil {
		return nil, fmt.Errorf("active subscription not found")
	}
	return row.Entitlements, nil
}
