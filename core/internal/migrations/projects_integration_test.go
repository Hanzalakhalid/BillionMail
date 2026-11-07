package migrations

import (
	"billionmail-core/internal/service/projects"
	"billionmail-core/internal/service/tenancy"
	"context"
	"errors"
	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/google/uuid"
	"testing"
)

func testProjectWorkspace(t *testing.T, ctx context.Context, tenant, other tenancy.TenantContext) {
	input := projects.Input{Title: "Launch", Objective: "Introduce the product"}
	key := uuid.NewString()
	first, err := projects.Save(ctx, tenant, key, input)
	if err != nil || first.ID == "" || first.Revision != 1 || len(first.Missing) != 4 {
		t.Fatalf("first=%+v err=%v", first, err)
	}
	retry, err := projects.Save(ctx, tenant, key, input)
	if err != nil || retry.ID != first.ID {
		t.Fatalf("retry=%+v err=%v", retry, err)
	}
	if recovered, err := projects.Get(ctx, tenant, "urn:uuid:"+first.ID); err != nil || recovered.ID != first.ID {
		t.Fatalf("UUID normalization=%+v err=%v", recovered, err)
	}
	input = first.Input
	input.ID = "urn:uuid:" + first.ID
	input.Requirements = projects.Requirements{Audience: "Subscribers", Channels: []string{"email"}, Languages: []string{"en"}, SuccessMetric: "Purchases"}
	next, err := projects.Save(ctx, tenant, uuid.NewString(), input)
	if err != nil || next.Revision != 2 || len(next.Missing) != 0 || next.Status != "discovery" {
		t.Fatalf("next=%+v err=%v", next, err)
	}
	if _, err := projects.Save(ctx, tenant, uuid.NewString(), input); !errors.Is(err, projects.ErrConflict) {
		t.Fatalf("stale save=%v", err)
	}
	if _, err := projects.Save(ctx, tenant, key, input); !errors.Is(err, projects.ErrConflict) {
		t.Fatalf("key reuse=%v", err)
	}
	if _, err := projects.Get(ctx, other, first.ID); !errors.Is(err, projects.ErrUnavailable) {
		t.Fatalf("cross-tenant get=%v", err)
	}
	if _, err := projects.Save(ctx, other, uuid.NewString(), input); !errors.Is(err, projects.ErrUnavailable) {
		t.Fatalf("cross-tenant save=%v", err)
	}
	messageKey := uuid.NewString()
	message, err := projects.AddMessage(ctx, tenant, messageKey, first.ID, "Keep the tone warm.")
	if err != nil || message.Role != "user" {
		t.Fatalf("message=%+v err=%v", message, err)
	}
	messageRetry, err := projects.AddMessage(ctx, tenant, messageKey, "urn:uuid:"+first.ID, "Keep the tone warm.")
	if err != nil || messageRetry.ID != message.ID {
		t.Fatalf("message retry=%+v err=%v", messageRetry, err)
	}
	if _, err := projects.AddMessage(ctx, other, uuid.NewString(), first.ID, "Intrusion"); !errors.Is(err, projects.ErrUnavailable) {
		t.Fatalf("cross-tenant message=%v", err)
	}
	page, err := projects.Events(ctx, tenant, first.ID, 0, 2)
	if err != nil || len(page.Items) != 2 || page.NextAfter <= 0 {
		t.Fatalf("events=%+v err=%v", page, err)
	}
	tail, err := projects.Events(ctx, tenant, "urn:uuid:"+first.ID, page.NextAfter, 100)
	if err != nil || len(tail.Items) != 1 || tail.Items[0].Type != "message.created" {
		t.Fatalf("tail=%+v err=%v", tail, err)
	}
	if _, err := projects.Events(ctx, other, first.ID, 0, 100); !errors.Is(err, projects.ErrUnavailable) {
		t.Fatalf("cross-tenant events=%v", err)
	}
	viewer := tenant
	viewer.Role = "viewer"
	if _, err := projects.Get(ctx, viewer, first.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := projects.AddMessage(ctx, viewer, uuid.NewString(), first.ID, "write"); !errors.Is(err, tenancy.ErrPermissionDenied) {
		t.Fatalf("viewer write=%v", err)
	}
	if _, err := g.DB().Model("marketing_project_messages").Where("id", message.ID).Data(g.Map{"content": "Changed history"}).Update(); err == nil {
		t.Fatal("conversation history was mutable")
	}
	if _, err := g.DB().Model("marketing_project_events").Where("id", page.Items[0].ID).Data(g.Map{"type": "changed"}).Update(); err == nil {
		t.Fatal("durable events were mutable")
	}
	otherProject, err := projects.Save(ctx, other, uuid.NewString(), projects.Input{Title: "Other project", Objective: "Other workspace"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := projects.AddMessage(ctx, other, uuid.NewString(), otherProject.ID, "Private note"); err != nil {
		t.Fatal(err)
	}
	if _, err := projects.Save(ctx, tenant, uuid.NewString(), projects.Input{Title: "Second project", Objective: "Second objective"}); err != nil {
		t.Fatal(err)
	}
	firstPage, err := projects.List(ctx, tenant, "", 1)
	if err != nil || len(firstPage.Items) != 1 || firstPage.NextCursor == "" {
		t.Fatalf("first page=%+v err=%v", firstPage, err)
	}
	secondPage, err := projects.List(ctx, tenant, firstPage.NextCursor, 1)
	if err != nil || len(secondPage.Items) != 1 || secondPage.Items[0].ID == firstPage.Items[0].ID {
		t.Fatalf("second page=%+v err=%v", secondPage, err)
	}
	if _, err := projects.List(ctx, tenant, "invalid", 1); err == nil {
		t.Fatal("accepted invalid project cursor")
	}
	if _, err := g.DB().Exec(ctx, "GRANT SELECT ON marketing_projects,marketing_project_messages,marketing_project_events TO bm_tenant_rls_test"); err != nil {
		t.Fatal(err)
	}
	err = g.DB().Transaction(ctx, func(_ context.Context, tx gdb.TX) error {
		if _, err := tx.Exec("SET LOCAL ROLE bm_tenant_rls_test"); err != nil {
			return err
		}
		if _, err := tx.Exec("SELECT set_config('app.company_id', ?, true)", tenant.CompanyID.String()); err != nil {
			return err
		}
		for _, table := range []string{"marketing_projects", "marketing_project_messages", "marketing_project_events"} {
			count, err := tx.Model(table).Where("company_id", other.CompanyID).Count()
			if err != nil {
				return err
			}
			if count != 0 {
				t.Errorf("cross-tenant %s count=%d", table, count)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
