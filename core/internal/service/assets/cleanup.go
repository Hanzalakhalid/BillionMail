package assets

import (
	"context"
	"errors"
	"time"

	platformqueue "billionmail-core/internal/service/queue"
	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/google/uuid"
)

// Cleanup removes quarantine objects only after their upload signatures expire.
// Abandoned processing jobs receive a 24-hour grace period and cannot have a
// live validation lease. Permanent asset objects follow workspace retention.
func (v Validator) Cleanup(ctx context.Context, event platformqueue.Event) error {
	if event.CompanyID == uuid.Nil || v.Store == nil {
		return errors.New("workspace and storage are required")
	}
	var rows []struct {
		ID            uuid.UUID `json:"id"`
		QuarantineKey string    `json:"quarantine_key"`
		Status        string    `json:"status"`
	}
	err := g.DB().Transaction(ctx, func(_ context.Context, tx gdb.TX) error {
		if _, err := tx.Exec("SELECT set_config('app.company_id', ?, true)", event.CompanyID.String()); err != nil {
			return err
		}
		result, err := tx.Query(`SELECT id,quarantine_key,status FROM marketing_asset_versions
   WHERE company_id=? AND quarantine_cleaned_at IS NULL AND upload_expires_at<NOW()
   AND (status<>'processing' OR (upload_expires_at<NOW()-INTERVAL '24 hours' AND (validation_until IS NULL OR validation_until<NOW())))
   ORDER BY upload_expires_at,id FOR UPDATE SKIP LOCKED LIMIT 100`, event.CompanyID)
		if err != nil {
			return err
		}
		if err = result.Structs(&rows); err != nil {
			return err
		}
		for _, row := range rows {
			if row.Status == "processing" || row.Status == "awaiting_upload" {
				if _, err := tx.Model("marketing_asset_versions").Where("company_id", event.CompanyID).Where("id", row.ID).Data(g.Map{"status": "failed", "rejection_reason": "Upload expired before validation completed", "validation_owner": nil, "validation_until": nil, "updated_at": time.Now().UTC()}).Update(); err != nil {
					return err
				}
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	var failures []error
	for _, row := range rows {
		expected := "companies/" + event.CompanyID.String() + "/quarantine/" + row.ID.String()
		if row.QuarantineKey != expected {
			failures = append(failures, errors.New("invalid workspace quarantine key"))
			continue
		}
		if err := v.Store.Delete(ctx, expected); err != nil {
			failures = append(failures, err)
			continue
		}
		err := g.DB().Transaction(ctx, func(_ context.Context, tx gdb.TX) error {
			if _, err := tx.Exec("SELECT set_config('app.company_id', ?, true)", event.CompanyID.String()); err != nil {
				return err
			}
			_, err := tx.Model("marketing_asset_versions").Where("company_id", event.CompanyID).Where("id", row.ID).Data(g.Map{"quarantine_cleaned_at": time.Now().UTC(), "updated_at": time.Now().UTC()}).Update()
			return err
		})
		if err != nil {
			failures = append(failures, err)
		}
	}
	return errors.Join(failures...)
}
