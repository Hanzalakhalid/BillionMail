package assets

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"billionmail-core/internal/service/objectstorage"
	"billionmail-core/internal/service/tenancy"
	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/google/uuid"
)

const MaxUploadBytes int64 = 20 * 1024 * 1024

var ErrConflict = errors.New("idempotency key was used with different upload requirements")

type UploadInput struct {
	Title       string `json:"title"`
	ContentType string `json:"content_type"`
	ByteSize    int64  `json:"byte_size"`
}
type Upload struct {
	AssetID   uuid.UUID                 `json:"asset_id"`
	VersionID uuid.UUID                 `json:"version_id"`
	Status    string                    `json:"status"`
	ExpiresAt time.Time                 `json:"expires_at"`
	Form      *objectstorage.UploadForm `json:"form,omitempty"`
}
type Asset struct {
	ID              uuid.UUID `json:"id"`
	Title           string    `json:"title"`
	Kind            string    `json:"kind"`
	VersionID       uuid.UUID `json:"version_id"`
	Status          string    `json:"status"`
	ContentType     string    `json:"content_type"`
	ByteSize        int64     `json:"byte_size"`
	Width           int       `json:"width"`
	Height          int       `json:"height"`
	RejectionReason string    `json:"rejection_reason"`
	CreatedAt       time.Time `json:"created_at"`
}
type Job struct {
	JobID          uuid.UUID `json:"job_id"`
	ResourceID     uuid.UUID `json:"resource_id"`
	Status         string    `json:"status"`
	ResourceStatus string    `json:"resource_status"`
}

type Page struct {
	Items      []Asset `json:"items"`
	NextCursor string  `json:"next_cursor"`
}

func Kind(contentType string) string {
	switch contentType {
	case "image/png", "image/jpeg", "image/webp":
		return "image"
	case "video/mp4":
		return "video"
	case "application/pdf":
		return "document"
	default:
		return ""
	}
}

func BeginUpload(ctx context.Context, tenant tenancy.TenantContext, store objectstorage.Storage, key string, input UploadInput) (Upload, error) {
	if err := tenancy.Authorize(tenant, "asset.manage"); err != nil {
		return Upload{}, err
	}
	input.Title = strings.TrimSpace(input.Title)
	input.ContentType = strings.ToLower(strings.TrimSpace(input.ContentType))
	if tenant.CompanyID == uuid.Nil || tenant.UserID == uuid.Nil || store == nil || key == "" || len(key) > 128 || input.Title == "" || len(input.Title) > 200 || Kind(input.ContentType) == "" || input.ByteSize < 1 || input.ByteSize > MaxUploadBytes {
		return Upload{}, errors.New("valid workspace, idempotency key, title, supported file type and size up to 20 MiB are required")
	}
	encoded, _ := json.Marshal(input)
	digest := sha256.Sum256(encoded)
	fingerprint := hex.EncodeToString(digest[:])
	result := Upload{}
	var quarantineKey string
	err := g.DB().Transaction(ctx, func(_ context.Context, tx gdb.TX) error {
		if _, err := tx.Exec("SELECT set_config('app.company_id', ?, true)", tenant.CompanyID.String()); err != nil {
			return err
		}
		if _, err := tx.Exec("SELECT pg_advisory_xact_lock(hashtextextended(?,0))", tenant.CompanyID.String()+":asset-upload:"+key); err != nil {
			return err
		}
		existing, err := tx.Model("marketing_asset_upload_requests").Where("company_id", tenant.CompanyID).Where("idempotency_key", key).Count()
		if err != nil {
			return err
		}
		if existing > 0 {
			var row struct {
				RequestHash     string    `json:"request_hash"`
				VersionID       uuid.UUID `json:"version_id"`
				AssetID         uuid.UUID `json:"asset_id"`
				Status          string    `json:"status"`
				QuarantineKey   string    `json:"quarantine_key"`
				UploadExpiresAt time.Time `json:"upload_expires_at"`
			}
			if err := tx.Model("marketing_asset_upload_requests r").InnerJoin("marketing_asset_versions v", "v.company_id=r.company_id AND v.id=r.version_id").Fields("r.request_hash,v.id version_id,v.asset_id,v.status,v.quarantine_key,v.upload_expires_at").Where("r.company_id", tenant.CompanyID).Where("r.idempotency_key", key).Scan(&row); err != nil {
				return err
			}
			if row.RequestHash != fingerprint {
				return ErrConflict
			}
			result = Upload{AssetID: row.AssetID, VersionID: row.VersionID, Status: row.Status, ExpiresAt: row.UploadExpiresAt}
			quarantineKey = row.QuarantineKey
			return nil
		}
		result = Upload{AssetID: uuid.New(), VersionID: uuid.New(), Status: "awaiting_upload", ExpiresAt: time.Now().UTC().Add(10 * time.Minute)}
		quarantineKey = "companies/" + tenant.CompanyID.String() + "/quarantine/" + result.VersionID.String()
		if _, err := tx.Model("marketing_assets").Data(g.Map{"id": result.AssetID, "company_id": tenant.CompanyID, "title": input.Title, "kind": Kind(input.ContentType), "created_by": tenant.UserID}).Insert(); err != nil {
			return err
		}
		if _, err := tx.Model("marketing_asset_versions").Data(g.Map{"id": result.VersionID, "company_id": tenant.CompanyID, "asset_id": result.AssetID, "version": 1, "declared_content_type": input.ContentType, "declared_size": input.ByteSize, "quarantine_key": quarantineKey, "upload_expires_at": result.ExpiresAt, "created_by": tenant.UserID}).Insert(); err != nil {
			return err
		}
		if _, err := tx.Model("marketing_assets").Where("company_id", tenant.CompanyID).Where("id", result.AssetID).Data(g.Map{"current_version_id": result.VersionID}).Update(); err != nil {
			return err
		}
		if _, err = tx.Model("marketing_asset_upload_requests").Data(g.Map{"company_id": tenant.CompanyID, "idempotency_key": key, "request_hash": fingerprint, "version_id": result.VersionID, "created_by": tenant.UserID}).Insert(); err != nil {
			return err
		}
		if _, err = tx.Exec(`INSERT INTO marketing_schedules(company_id,name,event_type,payload,cron_expression,timezone,next_run_at,source)
          VALUES(?,'Expired upload cleanup','asset.quarantine.cleanup','{}'::jsonb,'0 * * * *','UTC',NOW()+INTERVAL '1 hour','system')
          ON CONFLICT(company_id) WHERE event_type='asset.quarantine.cleanup' AND source='system' AND deleted_at IS NULL DO NOTHING`, tenant.CompanyID); err != nil {
			return err
		}
		_, err = tx.Model("audit_logs").Data(g.Map{"company_id": tenant.CompanyID, "actor_user_id": tenant.UserID, "action": "asset.upload.created", "resource_type": "asset", "resource_id": result.AssetID.String(), "metadata": g.Map{"version_id": result.VersionID, "content_type": input.ContentType, "byte_size": input.ByteSize}}).Insert()
		return err
	})
	if err != nil {
		return Upload{}, err
	}
	if result.Status == "awaiting_upload" {
		remaining := time.Until(result.ExpiresAt)
		if remaining < time.Second {
			return result, errors.New("upload request expired; create another upload with a new idempotency key")
		}
		form, err := store.PresignUpload(ctx, quarantineKey, input.ContentType, input.ByteSize, remaining)
		if err != nil {
			return Upload{}, err
		}
		result.Form = &form
	}
	return result, nil
}

func CompleteUpload(ctx context.Context, tenant tenancy.TenantContext, versionID uuid.UUID) (Job, error) {
	if err := tenancy.Authorize(tenant, "asset.manage"); err != nil {
		return Job{}, err
	}
	result := Job{}
	err := g.DB().Transaction(ctx, func(_ context.Context, tx gdb.TX) error {
		if _, err := tx.Exec("SELECT set_config('app.company_id', ?, true)", tenant.CompanyID.String()); err != nil {
			return err
		}
		var row struct {
			AssetID         uuid.UUID `json:"asset_id"`
			Status          string    `json:"status"`
			JobID           uuid.UUID `json:"job_id"`
			UploadExpiresAt time.Time `json:"upload_expires_at"`
		}
		if err := tx.Model("marketing_asset_versions v").InnerJoin("marketing_assets a", "a.id=v.asset_id AND a.company_id=v.company_id").Fields("v.asset_id,v.status,v.job_id,v.upload_expires_at").Where("v.company_id", tenant.CompanyID).Where("v.id", versionID).WhereNull("a.deleted_at").LockUpdate().Scan(&row); err != nil {
			return err
		}
		result = Job{JobID: row.JobID, ResourceID: row.AssetID, Status: "queued", ResourceStatus: row.Status}
		if row.Status == "ready" || row.Status == "rejected" {
			result.Status = "succeeded"
		} else if row.Status == "failed" {
			result.Status = "failed"
		}
		if row.Status != "awaiting_upload" {
			return nil
		}
		if time.Now().After(row.UploadExpiresAt) {
			return errors.New("upload request expired")
		}
		result.JobID = uuid.New()
		result.Status = "queued"
		result.ResourceStatus = "processing"
		payload, _ := json.Marshal(g.Map{"version_id": versionID})
		if _, err := tx.Model("outbox_events").Data(g.Map{"id": result.JobID, "company_id": tenant.CompanyID, "aggregate_type": "asset_version", "aggregate_id": versionID.String(), "event_type": "asset.upload.validate", "payload": string(payload), "idempotency_key": "asset-validation:" + tenant.CompanyID.String() + ":" + versionID.String()}).Insert(); err != nil {
			return err
		}
		if _, err := tx.Model("marketing_asset_versions").Where("company_id", tenant.CompanyID).Where("id", versionID).Data(g.Map{"status": "processing", "job_id": result.JobID, "updated_at": time.Now().UTC()}).Update(); err != nil {
			return err
		}
		_, err := tx.Model("audit_logs").Data(g.Map{"company_id": tenant.CompanyID, "actor_user_id": tenant.UserID, "action": "asset.validation.requested", "resource_type": "asset_version", "resource_id": versionID.String(), "metadata": g.Map{"job_id": result.JobID}}).Insert()
		return err
	})
	return result, err
}

func List(ctx context.Context, tenant tenancy.TenantContext, cursor string, limit int) (Page, error) {
	if err := tenancy.Authorize(tenant, "asset.read"); err != nil {
		return Page{}, err
	}
	if limit == 0 {
		limit = 30
	}
	if limit < 1 || limit > 100 {
		return Page{}, errors.New("asset page size must be between 1 and 100")
	}
	var cursorTime time.Time
	var cursorID uuid.UUID
	if cursor != "" {
		if len(cursor) > 256 {
			return Page{}, errors.New("invalid asset cursor")
		}
		decoded, err := base64.RawURLEncoding.DecodeString(cursor)
		if err != nil {
			return Page{}, errors.New("invalid asset cursor")
		}
		parts := strings.Split(string(decoded), "|")
		if len(parts) != 2 {
			return Page{}, errors.New("invalid asset cursor")
		}
		cursorTime, err = time.Parse(time.RFC3339Nano, parts[0])
		if err != nil {
			return Page{}, errors.New("invalid asset cursor")
		}
		cursorID, err = uuid.Parse(parts[1])
		if err != nil || cursorID == uuid.Nil {
			return Page{}, errors.New("invalid asset cursor")
		}
	}
	result := make([]Asset, 0)
	err := g.DB().Transaction(ctx, func(_ context.Context, tx gdb.TX) error {
		if _, err := tx.Exec("SELECT set_config('app.company_id', ?, true)", tenant.CompanyID.String()); err != nil {
			return err
		}
		query := tx.Model("marketing_assets a").InnerJoin("marketing_asset_versions v", "v.id=a.current_version_id AND v.company_id=a.company_id").LeftJoin("outbox_events e", "e.id=v.job_id AND e.company_id=v.company_id").Fields("a.id,a.title,a.kind,v.id version_id,CASE WHEN v.status='processing' AND e.dead_lettered_at IS NOT NULL THEN 'failed' ELSE v.status END status,v.content_type,v.byte_size,v.width,v.height,COALESCE(v.rejection_reason,CASE WHEN e.dead_lettered_at IS NOT NULL THEN 'Validation could not complete' ELSE '' END) rejection_reason,a.created_at").Where("a.company_id", tenant.CompanyID).WhereNull("a.deleted_at").WhereNull("v.deleted_at")
		if cursor != "" {
			query = query.Where("(a.created_at,a.id)<(?,?)", cursorTime, cursorID)
		}
		return query.Order("a.created_at DESC,a.id DESC").Limit(limit + 1).Scan(&result)
	})
	if err != nil {
		return Page{}, err
	}
	page := Page{Items: result}
	if len(result) > limit {
		page.Items = result[:limit]
		last := page.Items[limit-1]
		page.NextCursor = base64.RawURLEncoding.EncodeToString([]byte(last.CreatedAt.UTC().Format(time.RFC3339Nano) + "|" + last.ID.String()))
	}
	return page, nil
}

func Delete(ctx context.Context, tenant tenancy.TenantContext, id uuid.UUID) error {
	if err := tenancy.Authorize(tenant, "asset.manage"); err != nil {
		return err
	}
	return g.DB().Transaction(ctx, func(_ context.Context, tx gdb.TX) error {
		if _, err := tx.Exec("SELECT set_config('app.company_id', ?, true)", tenant.CompanyID.String()); err != nil {
			return err
		}
		var row struct {
			ID        uuid.UUID  `json:"id"`
			DeletedAt *time.Time `json:"deleted_at"`
		}
		if err := tx.Model("marketing_assets").Unscoped().Where("company_id", tenant.CompanyID).Where("id", id).LockUpdate().Scan(&row); err != nil {
			return err
		}
		if row.DeletedAt != nil {
			return nil
		}
		now := time.Now().UTC()
		if _, err := tx.Model("marketing_assets").Where("company_id", tenant.CompanyID).Where("id", id).Data(g.Map{"deleted_at": now, "updated_at": now}).Update(); err != nil {
			return err
		}
		_, err := tx.Model("audit_logs").Data(g.Map{"company_id": tenant.CompanyID, "actor_user_id": tenant.UserID, "action": "asset.deleted", "resource_type": "asset", "resource_id": id.String()}).Insert()
		return err
	})
}

func Download(ctx context.Context, tenant tenancy.TenantContext, store objectstorage.Storage, versionID uuid.UUID) (string, error) {
	if err := tenancy.Authorize(tenant, "asset.read"); err != nil {
		return "", err
	}
	var objectKey string
	err := g.DB().Transaction(ctx, func(_ context.Context, tx gdb.TX) error {
		if _, err := tx.Exec("SELECT set_config('app.company_id', ?, true)", tenant.CompanyID.String()); err != nil {
			return err
		}
		value, err := tx.Model("marketing_asset_versions v").InnerJoin("marketing_assets a", "a.company_id=v.company_id AND a.id=v.asset_id").Where("v.company_id", tenant.CompanyID).Where("v.id", versionID).Where("v.status", "ready").WhereNull("a.deleted_at").WhereNull("v.deleted_at").Value("v.object_key")
		if err != nil {
			return err
		}
		if value == nil || value.IsNil() || value.String() == "" {
			return errors.New("validated asset version is unavailable")
		}
		objectKey = value.String()
		return nil
	})
	if err != nil {
		return "", err
	}
	if !strings.HasPrefix(objectKey, "companies/"+tenant.CompanyID.String()+"/") {
		return "", fmt.Errorf("asset object is outside workspace")
	}
	return store.PresignGet(ctx, objectKey, 5*time.Minute)
}
