package migrations

import (
	"context"
	"errors"
	"testing"

	"billionmail-core/internal/service/business"
	"billionmail-core/internal/service/tenancy"
	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/google/uuid"
)

func testBusinessProfile(t *testing.T, ctx context.Context, tenant, other tenancy.TenantContext) {
	if profile, err := business.Get(ctx, tenant); err != nil || profile != nil {
		t.Fatalf("empty profile=%+v err=%v", profile, err)
	}
	input := business.Input{Name: "Canonical store", Website: "https://example.test", Industry: "Retail", BrandVoice: "Helpful", Locale: "en", Timezone: "Asia/Karachi"}
	key := uuid.NewString()
	first, err := business.Save(ctx, tenant, key, input)
	if err != nil {
		t.Fatal(err)
	}
	if first.Revision != 1 || first.ID == uuid.Nil {
		t.Fatalf("first profile=%+v", first)
	}
	duplicate, err := business.Save(ctx, tenant, key, input)
	if err != nil || duplicate.ID != first.ID || duplicate.Revision != 1 {
		t.Fatalf("retry=%+v err=%v", duplicate, err)
	}
	changed := input
	changed.Name = "Updated store"
	changed.Revision = 1
	next, err := business.Save(ctx, tenant, uuid.NewString(), changed)
	if err != nil || next.ID != first.ID || next.Revision != 2 {
		t.Fatalf("update=%+v err=%v", next, err)
	}
	if _, err := business.Save(ctx, tenant, uuid.NewString(), changed); !errors.Is(err, business.ErrConflict) {
		t.Fatalf("stale revision accepted: %v", err)
	}
	if _, err := business.Save(ctx, tenant, key, changed); !errors.Is(err, business.ErrConflict) {
		t.Fatalf("key reuse accepted: %v", err)
	}
	retry, err := business.Save(ctx, tenant, key, input)
	if err != nil || retry.Revision != 1 {
		t.Fatalf("historical retry=%+v err=%v", retry, err)
	}
	current, err := business.Get(ctx, tenant)
	if err != nil || current.Revision != 2 || current.Name != changed.Name {
		t.Fatalf("current=%+v err=%v", current, err)
	}
	if profile, err := business.Get(ctx, other); err != nil || profile != nil {
		t.Fatalf("cross-tenant profile=%+v err=%v", profile, err)
	}
	otherProfile, err := business.Save(ctx, other, key, input)
	if err != nil || otherProfile.ID == first.ID {
		t.Fatalf("workspace key collision: %v", err)
	}
	viewer := tenant
	viewer.Role = "viewer"
	if _, err := business.Save(ctx, viewer, uuid.NewString(), input); !errors.Is(err, tenancy.ErrPermissionDenied) {
		t.Fatalf("viewer save err=%v", err)
	}
	count, err := g.DB().Model("audit_logs").Where("company_id", tenant.CompanyID).Where("action", "business.profile.saved").Count()
	if err != nil || count != 2 {
		t.Fatalf("audits=%d err=%v, want only two changes", count, err)
	}
	if _, err := g.DB().Exec(ctx, "GRANT SELECT,UPDATE ON marketing_business_profiles TO bm_tenant_rls_test; GRANT SELECT ON marketing_mutation_receipts TO bm_tenant_rls_test"); err != nil {
		t.Fatal(err)
	}
	err = g.DB().Transaction(ctx, func(_ context.Context, tx gdb.TX) error {
		if _, err := tx.Exec("SET LOCAL ROLE bm_tenant_rls_test"); err != nil {
			return err
		}
		if _, err := tx.Exec("SELECT set_config('app.company_id', ?, true)", tenant.CompanyID.String()); err != nil {
			return err
		}
		visible, err := tx.Model("marketing_business_profiles").Count()
		if err != nil {
			return err
		}
		if visible != 1 {
			t.Errorf("RLS visible business profiles=%d", visible)
		}
		result, err := tx.Model("marketing_business_profiles").Where("company_id", other.CompanyID).Data(g.Map{"name": "Unauthorized overwrite"}).Update()
		if err != nil {
			return err
		}
		changed, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if changed != 0 {
			t.Error("RLS allowed cross-workspace profile update")
		}
		visible, err = tx.Model("marketing_mutation_receipts").Where("operation", "business.save").Count()
		if err != nil {
			return err
		}
		if visible != 2 {
			t.Errorf("RLS visible mutation receipts=%d", visible)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
