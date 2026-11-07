package migrations

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/png"
	"io"
	"sync"
	"testing"
	"time"

	"billionmail-core/internal/service/antivirus"
	"billionmail-core/internal/service/assets"
	"billionmail-core/internal/service/objectstorage"
	platformqueue "billionmail-core/internal/service/queue"
	"billionmail-core/internal/service/tenancy"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/google/uuid"
)

type memoryAssetStore struct {
	mu      sync.Mutex
	objects map[string][]byte
	puts    int
}

func (s *memoryAssetStore) Put(_ context.Context, key string, body io.Reader, _ string) error {
	data, err := io.ReadAll(body)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.objects[key] = data
	s.puts++
	return nil
}
func (s *memoryAssetStore) Get(_ context.Context, key string) (io.ReadCloser, string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, ok := s.objects[key]
	if !ok {
		return nil, "", errors.New("object missing")
	}
	return io.NopCloser(bytes.NewReader(append([]byte(nil), data...))), "image/png", nil
}
func (s *memoryAssetStore) Delete(_ context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.objects, key)
	return nil
}
func (s *memoryAssetStore) PresignGet(_ context.Context, key string, _ time.Duration) (string, error) {
	return "https://storage.test/" + key, nil
}
func (s *memoryAssetStore) PresignPut(_ context.Context, key, _ string, _ time.Duration) (string, error) {
	return "https://storage.test/" + key, nil
}
func (s *memoryAssetStore) PresignUpload(_ context.Context, key, _ string, _ int64, _ time.Duration) (objectstorage.UploadForm, error) {
	return objectstorage.UploadForm{URL: "https://storage.test/upload", Fields: map[string]string{"key": key}}, nil
}
func (s *memoryAssetStore) Health(context.Context) error { return nil }

type assetScanner struct {
	failure error
	started chan struct{}
	release chan struct{}
}

func (s assetScanner) Scan(ctx context.Context, _ io.Reader) error {
	if s.started != nil {
		close(s.started)
		select {
		case <-s.release:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return s.failure
}
func (s assetScanner) Health(context.Context) error { return s.failure }

func testAssetLifecycle(t *testing.T, ctx context.Context, company, otherCompany, user uuid.UUID) {
	t.Helper()
	tenant := tenancy.TenantContext{CompanyID: company, UserID: user, Role: "owner"}
	other := tenancy.TenantContext{CompanyID: otherCompany, UserID: user, Role: "owner"}
	storage := &memoryAssetStore{objects: map[string][]byte{}}
	var imageData bytes.Buffer
	if err := png.Encode(&imageData, image.NewRGBA(image.Rect(0, 0, 8, 6))); err != nil {
		t.Fatal(err)
	}
	input := assets.UploadInput{Title: "Product reference", ContentType: "image/png", ByteSize: int64(imageData.Len())}
	makeUpload := func(t *testing.T, data []byte) (assets.Upload, platformqueue.Event) {
		t.Helper()
		request := input
		request.ByteSize = int64(len(data))
		upload, err := assets.BeginUpload(ctx, tenant, storage, uuid.NewString(), request)
		if err != nil {
			t.Fatal(err)
		}
		storage.mu.Lock()
		storage.objects[upload.Form.Fields["key"]] = append([]byte(nil), data...)
		storage.mu.Unlock()
		if _, err = assets.Download(ctx, tenant, storage, upload.VersionID); err == nil {
			t.Fatal("quarantined upload was downloadable")
		}
		job, err := assets.CompleteUpload(ctx, tenant, upload.VersionID)
		if err != nil {
			t.Fatal(err)
		}
		again, err := assets.CompleteUpload(ctx, tenant, upload.VersionID)
		if err != nil || again.JobID != job.JobID {
			t.Fatalf("completion was not idempotent: %v", err)
		}
		var event platformqueue.Event
		if err = g.DB().Model("outbox_events").Where("company_id", company).Where("id", job.JobID).Scan(&event); err != nil {
			t.Fatal(err)
		}
		return upload, event
	}
	t.Run("idempotent upload form", func(t *testing.T) {
		key := uuid.NewString()
		first, err := assets.BeginUpload(ctx, tenant, storage, key, input)
		if err != nil {
			t.Fatal(err)
		}
		second, err := assets.BeginUpload(ctx, tenant, storage, key, input)
		if err != nil || first.VersionID != second.VersionID {
			t.Fatalf("duplicate upload err=%v", err)
		}
		different := input
		different.Title = "Different product"
		if _, err = assets.BeginUpload(ctx, tenant, storage, key, different); !errors.Is(err, assets.ErrConflict) {
			t.Fatalf("idempotency conflict err=%v", err)
		}
		third, err := assets.BeginUpload(ctx, other, storage, key, input)
		if err != nil || third.VersionID == first.VersionID {
			t.Fatalf("tenant idempotency collision: %v", err)
		}
		if _, err = assets.CompleteUpload(ctx, other, first.VersionID); err == nil {
			t.Fatal("cross-tenant completion allowed")
		}
	})
	t.Run("validated immutable image", func(t *testing.T) {
		upload, event := makeUpload(t, imageData.Bytes())
		validator := assets.Validator{Store: storage, Scanner: assetScanner{}}
		if err := validator.Handle(ctx, event); err != nil {
			t.Fatal(err)
		}
		before := storage.puts
		if err := validator.Handle(ctx, event); err != nil || storage.puts != before {
			t.Fatalf("validation retry rewrote immutable bytes: %v", err)
		}
		if _, err := assets.Download(ctx, tenant, storage, upload.VersionID); err != nil {
			t.Fatal(err)
		}
		if _, err := assets.Download(ctx, other, storage, upload.VersionID); err == nil {
			t.Fatal("cross-tenant download allowed")
		}
		if _, err := g.DB().Model("marketing_asset_versions").Where("id", upload.VersionID).Data(g.Map{"object_key": "companies/elsewhere/overwrite"}).Update(); err == nil {
			t.Fatal("database allowed ready version mutation")
		}
		var row struct {
			Width, Height int
			SHA256        string `json:"sha256"`
		}
		if err := g.DB().Model("marketing_asset_versions").Where("id", upload.VersionID).Scan(&row); err != nil || row.Width != 8 || row.Height != 6 || len(row.SHA256) != 64 {
			t.Fatalf("validated metadata=%+v err=%v", row, err)
		}
	})
	t.Run("invalid content and malware stay private", func(t *testing.T) {
		for _, test := range []struct {
			body    []byte
			scanner assetScanner
		}{{[]byte("not an image"), assetScanner{}}, {imageData.Bytes(), assetScanner{failure: antivirus.ErrInfected}}} {
			upload, event := makeUpload(t, test.body)
			if err := (assets.Validator{Store: storage, Scanner: test.scanner}).Handle(ctx, event); err != nil {
				t.Fatal(err)
			}
			status, err := g.DB().Model("marketing_asset_versions").Where("id", upload.VersionID).Value("status")
			if err != nil || status.String() != "rejected" {
				t.Fatalf("unsafe upload status=%v err=%v", status, err)
			}
			if _, err := assets.Download(ctx, tenant, storage, upload.VersionID); err == nil {
				t.Fatal("rejected file was downloadable")
			}
		}
	})
	t.Run("scanner outage retries without publication", func(t *testing.T) {
		upload, event := makeUpload(t, imageData.Bytes())
		if err := (assets.Validator{Store: storage, Scanner: assetScanner{failure: errors.New("scanner unavailable")}}).Handle(ctx, event); err == nil {
			t.Fatal("scanner outage accepted")
		}
		if _, err := assets.Download(ctx, tenant, storage, upload.VersionID); err == nil {
			t.Fatal("unscanned file published")
		}
		if err := (assets.Validator{Store: storage, Scanner: assetScanner{}}).Handle(ctx, event); err != nil {
			t.Fatalf("scanner recovery failed: %v", err)
		}
	})
	t.Run("expired quarantine cleanup preserves permanent assets", func(t *testing.T) {
		upload, event := makeUpload(t, imageData.Bytes())
		if _, err := g.DB().Model("marketing_asset_versions").Where("id", upload.VersionID).Data(g.Map{"upload_expires_at": time.Now().Add(-time.Hour)}).Update(); err != nil {
			t.Fatal(err)
		}
		validator := assets.Validator{Store: storage, Scanner: assetScanner{}}
		if err := validator.Handle(ctx, event); err != nil {
			t.Fatal(err)
		}
		key := upload.Form.Fields["key"]
		storage.mu.Lock()
		storage.objects[key] = []byte("late upload")
		storage.mu.Unlock()
		if err := validator.Cleanup(ctx, platformqueue.Event{CompanyID: company}); err != nil {
			t.Fatal(err)
		}
		storage.mu.Lock()
		_, exists := storage.objects[key]
		storage.mu.Unlock()
		if exists {
			t.Fatal("expired quarantine object was retained")
		}
		if _, err := assets.Download(ctx, tenant, storage, upload.VersionID); err != nil {
			t.Fatalf("cleanup removed a permanent asset: %v", err)
		}
		pending, pendingEvent := makeUpload(t, imageData.Bytes())
		if _, err := g.DB().Model("marketing_asset_versions").Where("id", pending.VersionID).Data(g.Map{"upload_expires_at": time.Now().Add(-48 * time.Hour), "validation_until": time.Now().Add(time.Hour), "validation_owner": uuid.New()}).Update(); err != nil {
			t.Fatal(err)
		}
		if err := validator.Cleanup(ctx, platformqueue.Event{CompanyID: company}); err != nil {
			t.Fatal(err)
		}
		status, err := g.DB().Model("marketing_asset_versions").Where("id", pending.VersionID).Value("status")
		if err != nil || status.String() != "processing" {
			t.Fatal("cleanup interrupted a live validation lease")
		}
		if _, err := g.DB().Model("marketing_asset_versions").Where("id", pending.VersionID).Data(g.Map{"validation_until": time.Now().Add(-time.Minute)}).Update(); err != nil {
			t.Fatal(err)
		}
		if err := validator.Cleanup(ctx, platformqueue.Event{CompanyID: company}); err != nil {
			t.Fatal(err)
		}
		status, err = g.DB().Model("marketing_asset_versions").Where("id", pending.VersionID).Value("status")
		if err != nil || status.String() != "failed" {
			t.Fatalf("abandoned upload status=%v err=%v", status, err)
		}
		if err := validator.Handle(ctx, pendingEvent); err != nil {
			t.Fatalf("expired upload should not be validated again: %v", err)
		}
	})
	t.Run("cursor pagination and deletion", func(t *testing.T) {
		var tiedIDs []uuid.UUID
		for i := 0; i < 4; i++ {
			upload, err := assets.BeginUpload(ctx, tenant, storage, uuid.NewString(), input)
			if err != nil {
				t.Fatal(err)
			}
			tiedIDs = append(tiedIDs, upload.AssetID)
		}
		if _, err := g.DB().Model("marketing_assets").Where("company_id", company).WhereIn("id", tiedIDs).Data(g.Map{"created_at": time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)}).Update(); err != nil {
			t.Fatal(err)
		}
		var cursor string
		seen := map[uuid.UUID]bool{}
		for pageNumber := 0; pageNumber < 50; pageNumber++ {
			page, err := assets.List(ctx, tenant, cursor, 2)
			if err != nil {
				t.Fatal(err)
			}
			if len(page.Items) > 2 {
				t.Fatal("page size exceeded")
			}
			for _, asset := range page.Items {
				if seen[asset.ID] {
					t.Fatal("cursor repeated an asset")
				}
				seen[asset.ID] = true
			}
			cursor = page.NextCursor
			if cursor == "" {
				break
			}
		}
		expected, err := g.DB().Model("marketing_assets").Where("company_id", company).WhereNull("deleted_at").Count()
		if err != nil || len(seen) != expected {
			t.Fatalf("pagination saw=%d expected=%d err=%v", len(seen), expected, err)
		}
		if _, err := assets.List(ctx, tenant, "invalid-cursor", 2); err == nil {
			t.Fatal("invalid cursor accepted")
		}
		upload, event := makeUpload(t, imageData.Bytes())
		if err := (assets.Validator{Store: storage, Scanner: assetScanner{}}).Handle(ctx, event); err != nil {
			t.Fatal(err)
		}
		if err := assets.Delete(ctx, other, upload.AssetID); err == nil {
			t.Fatal("cross-tenant deletion allowed")
		}
		if err := assets.Delete(ctx, tenant, upload.AssetID); err != nil {
			t.Fatal(err)
		}
		if err := assets.Delete(ctx, tenant, upload.AssetID); err != nil {
			t.Fatalf("duplicate deletion failed: %v", err)
		}
		if _, err := assets.Download(ctx, tenant, storage, upload.VersionID); err == nil {
			t.Fatal("deleted asset created another download URL")
		}
		viewer := tenant
		viewer.Role = "viewer"
		if _, err := assets.BeginUpload(ctx, viewer, storage, uuid.NewString(), input); !errors.Is(err, tenancy.ErrPermissionDenied) {
			t.Fatalf("viewer upload err=%v", err)
		}
		if err := assets.Delete(ctx, viewer, upload.AssetID); !errors.Is(err, tenancy.ErrPermissionDenied) {
			t.Fatalf("viewer delete err=%v", err)
		}
	})
	t.Run("expired owner cannot overwrite published bytes", func(t *testing.T) {
		upload, event := makeUpload(t, imageData.Bytes())
		started, release := make(chan struct{}), make(chan struct{})
		var unblock sync.Once
		defer unblock.Do(func() { close(release) })
		firstResult := make(chan error, 1)
		go func() {
			firstResult <- (assets.Validator{Store: storage, Scanner: assetScanner{started: started, release: release}}).Handle(ctx, event)
		}()
		select {
		case <-started:
		case <-time.After(5 * time.Second):
			t.Fatal("validation never started")
		}
		if err := (assets.Validator{Store: storage, Scanner: assetScanner{}}).Handle(ctx, event); !errors.Is(err, assets.ErrValidationBusy) {
			t.Fatalf("live validation lease not respected: %v", err)
		}
		if _, err := g.DB().Model("marketing_asset_versions").Where("id", upload.VersionID).Data(g.Map{"validation_until": time.Now().Add(-time.Second)}).Update(); err != nil {
			t.Fatal(err)
		}
		if err := (assets.Validator{Store: storage, Scanner: assetScanner{}}).Handle(ctx, event); err != nil {
			t.Fatal(err)
		}
		key, err := g.DB().Model("marketing_asset_versions").Where("id", upload.VersionID).Value("object_key")
		if err != nil {
			t.Fatal(err)
		}
		unblock.Do(func() { close(release) })
		select {
		case err := <-firstResult:
			if !errors.Is(err, assets.ErrValidationLeaseLost) {
				t.Fatalf("expired validator result=%v", err)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("expired validator did not stop")
		}
		storage.mu.Lock()
		data, exists := storage.objects[key.String()]
		storage.mu.Unlock()
		if !exists || !bytes.Equal(data, imageData.Bytes()) {
			t.Fatal("stale owner removed or replaced the published asset")
		}
	})
}
