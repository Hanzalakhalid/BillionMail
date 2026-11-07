package migrations

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/png"
	"sync"
	"testing"

	"billionmail-core/internal/service/assets"
	"billionmail-core/internal/service/products"
	platformqueue "billionmail-core/internal/service/queue"
	"billionmail-core/internal/service/tenancy"
	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/google/uuid"
)

func testProducts(t *testing.T, ctx context.Context, tenant, other tenancy.TenantContext) {
	imageVersion := func(owner tenancy.TenantContext, complete bool) uuid.UUID {
		t.Helper()
		var buffer bytes.Buffer
		if err := png.Encode(&buffer, image.NewRGBA(image.Rect(0, 0, 8, 6))); err != nil {
			t.Fatal(err)
		}
		store := &memoryAssetStore{objects: map[string][]byte{}}
		upload, err := assets.BeginUpload(ctx, owner, store, uuid.NewString(), assets.UploadInput{Title: "Product reference", ContentType: "image/png", ByteSize: int64(buffer.Len())})
		if err != nil {
			t.Fatal(err)
		}
		if !complete {
			return upload.VersionID
		}
		store.objects[upload.Form.Fields["key"]] = buffer.Bytes()
		job, err := assets.CompleteUpload(ctx, owner, upload.VersionID)
		if err != nil {
			t.Fatal(err)
		}
		var event platformqueue.Event
		if err := g.DB().Model("outbox_events").Where("company_id", owner.CompanyID).Where("id", job.JobID).Scan(&event); err != nil {
			t.Fatal(err)
		}
		if err := (assets.Validator{Store: store, Scanner: assetScanner{}}).Handle(ctx, event); err != nil {
			t.Fatal(err)
		}
		return upload.VersionID
	}
	reference := imageVersion(tenant, true)
	otherReference := imageVersion(other, true)
	unvalidated := imageVersion(tenant, false)
	input := products.Input{Name: "Reusable bottle", SKU: "BOTTLE-1", Description: "Reusable steel bottle", Benefits: []string{"Reusable", "Insulated"}, URL: "https://example.test/bottle", Price: "24.95", Currency: "USD", AssetVersionIDs: []uuid.UUID{reference}}
	key := uuid.NewString()
	first, err := products.Save(ctx, tenant, key, input)
	if err != nil {
		t.Fatal(err)
	}
	if first.ID == uuid.Nil || first.VersionID == uuid.Nil || first.Revision != 1 {
		t.Fatalf("created=%+v", first)
	}
	duplicate, err := products.Save(ctx, tenant, key, input)
	if err != nil || duplicate.ID != first.ID || duplicate.VersionID != first.VersionID {
		t.Fatalf("retry=%+v err=%v", duplicate, err)
	}
	input.ID = first.ID
	input.Revision = 1
	input.Description = "New product description"
	input.AssetVersionIDs = []uuid.UUID{}
	next, err := products.Save(ctx, tenant, uuid.NewString(), input)
	if err != nil || next.Revision != 2 || next.VersionID == first.VersionID {
		t.Fatalf("update=%+v err=%v", next, err)
	}
	if _, err := products.Save(ctx, tenant, uuid.NewString(), input); !errors.Is(err, products.ErrConflict) {
		t.Fatalf("stale revision=%v", err)
	}
	if _, err := products.Save(ctx, tenant, key, input); !errors.Is(err, products.ErrConflict) {
		t.Fatalf("different keyed mutation=%v", err)
	}
	historical, err := products.Versions(ctx, tenant, first.ID, 0)
	if err != nil || len(historical) != 2 || historical[1].Product.Description != "Reusable steel bottle" || len(historical[1].Product.AssetVersionIDs) != 1 || historical[1].Product.AssetVersionIDs[0] != reference {
		t.Fatalf("history=%+v err=%v", historical, err)
	}
	older, err := products.Versions(ctx, tenant, first.ID, 2)
	if err != nil || len(older) != 1 || older[0].Version != 1 {
		t.Fatalf("history page=%+v err=%v", older, err)
	}
	if _, err := g.DB().Model("marketing_product_versions").Where("id", first.VersionID).Data(g.Map{"snapshot": "{}"}).Update(); err == nil {
		t.Fatal("immutable product version overwritten")
	}
	sourceCount, err := g.DB().Model("marketing_product_sources").Where("company_id", tenant.CompanyID).Where("product_version_id", first.VersionID).Where("kind", "url").WhereNull("verified_at").Count()
	if err != nil || sourceCount != 1 {
		t.Fatalf("unfetched URL provenance=%d err=%v", sourceCount, err)
	}
	if _, err := products.Get(ctx, other, first.ID); !errors.Is(err, products.ErrUnavailable) {
		t.Fatalf("cross-tenant get=%v", err)
	}
	otherEdit := input
	otherEdit.Revision = 2
	if _, err := products.Save(ctx, other, uuid.NewString(), otherEdit); err == nil {
		t.Fatal("cross-tenant product update allowed")
	}
	for _, version := range []uuid.UUID{otherReference, unvalidated, uuid.New()} {
		bad := input
		bad.ID = uuid.Nil
		bad.Revision = 0
		bad.SKU = ""
		bad.AssetVersionIDs = []uuid.UUID{version}
		if _, err := products.Save(ctx, tenant, uuid.NewString(), bad); err == nil {
			t.Fatal("invalid reference accepted")
		}
	}
	clash := input
	clash.ID = uuid.Nil
	clash.Revision = 0
	clash.SKU = "bottle-1"
	if _, err := products.Save(ctx, tenant, uuid.NewString(), clash); !errors.Is(err, products.ErrConflict) {
		t.Fatalf("case-insensitive SKU collision=%v", err)
	}
	otherProduct := clash
	otherProduct.AssetVersionIDs = []uuid.UUID{otherReference}
	if _, err := products.Save(ctx, other, key, otherProduct); err != nil {
		t.Fatalf("tenant SKU/idempotency scope: %v", err)
	}
	viewer := tenant
	viewer.Role = "viewer"
	if _, err := products.Get(ctx, viewer, first.ID); err != nil {
		t.Fatalf("viewer read=%v", err)
	}
	if _, err := products.Save(ctx, viewer, uuid.NewString(), clash); !errors.Is(err, tenancy.ErrPermissionDenied) {
		t.Fatalf("viewer save=%v", err)
	}
	if err := products.Delete(ctx, other, first.ID); !errors.Is(err, products.ErrUnavailable) {
		t.Fatalf("cross-tenant delete=%v", err)
	}

	t.Run("concurrent same-key creation", func(t *testing.T) {
		candidate := clash
		candidate.SKU = "CONCURRENT"
		candidate.AssetVersionIDs = []uuid.UUID{}
		key := uuid.NewString()
		results := make(chan products.Product, 2)
		failures := make(chan error, 2)
		var wg sync.WaitGroup
		for i := 0; i < 2; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				result, err := products.Save(ctx, tenant, key, candidate)
				results <- result
				failures <- err
			}()
		}
		wg.Wait()
		close(results)
		close(failures)
		for err := range failures {
			if err != nil {
				t.Fatal(err)
			}
		}
		var version uuid.UUID
		for result := range results {
			if version != uuid.Nil && version != result.VersionID {
				t.Fatal("concurrent idempotency created duplicate versions")
			}
			version = result.VersionID
		}
	})
	t.Run("pagination and deletion", func(t *testing.T) {
		seen := map[uuid.UUID]bool{}
		cursor := ""
		for i := 0; i < 20; i++ {
			page, err := products.List(ctx, viewer, cursor, 1)
			if err != nil {
				t.Fatal(err)
			}
			for _, product := range page.Items {
				if seen[product.ID] {
					t.Fatal("cursor repeated a product")
				}
				seen[product.ID] = true
			}
			cursor = page.NextCursor
			if cursor == "" {
				break
			}
		}
		if len(seen) != 2 {
			t.Fatalf("pagination products=%d want 2", len(seen))
		}
		if err := products.Delete(ctx, tenant, first.ID); err != nil {
			t.Fatal(err)
		}
		if err := products.Delete(ctx, tenant, first.ID); err != nil {
			t.Fatalf("duplicate delete=%v", err)
		}
		if _, err := products.Get(ctx, tenant, first.ID); !errors.Is(err, products.ErrUnavailable) {
			t.Fatalf("deleted product readable=%v", err)
		}
		clash.AssetVersionIDs = []uuid.UUID{}
		if _, err := products.Save(ctx, tenant, uuid.NewString(), clash); err != nil {
			t.Fatalf("deleted SKU not released=%v", err)
		}
	})
	if _, err := g.DB().Exec(ctx, "GRANT SELECT,UPDATE ON marketing_products TO bm_tenant_rls_test; GRANT SELECT ON marketing_product_versions,marketing_product_media,marketing_product_sources TO bm_tenant_rls_test"); err != nil {
		t.Fatal(err)
	}
	err = g.DB().Transaction(ctx, func(_ context.Context, tx gdb.TX) error {
		if _, err := tx.Exec("SET LOCAL ROLE bm_tenant_rls_test"); err != nil {
			return err
		}
		if _, err := tx.Exec("SELECT set_config('app.company_id', ?, true)", tenant.CompanyID.String()); err != nil {
			return err
		}
		for _, table := range []string{"marketing_products", "marketing_product_versions", "marketing_product_media", "marketing_product_sources"} {
			count, err := tx.Model(table).Unscoped().Where("company_id", other.CompanyID).Count()
			if err != nil {
				return err
			}
			if count != 0 {
				t.Errorf("RLS exposed cross-company rows in %s", table)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
