package assets

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"billionmail-core/internal/service/antivirus"
	"billionmail-core/internal/service/objectstorage"
	platformqueue "billionmail-core/internal/service/queue"
	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/google/uuid"
	_ "golang.org/x/image/webp"
)

var ErrValidationBusy = errors.New("asset validation is already leased")
var ErrValidationLeaseLost = errors.New("asset validation lease expired or was replaced")
var ErrMediaToolUnavailable = errors.New("media validation tool is unavailable")

type Validator struct {
	Store   objectstorage.Storage
	Scanner antivirus.Scanner
}
type validationRow struct {
	ID                  uuid.UUID  `json:"id"`
	AssetID             uuid.UUID  `json:"asset_id"`
	Status              string     `json:"status"`
	DeclaredContentType string     `json:"declared_content_type"`
	DeclaredSize        int64      `json:"declared_size"`
	QuarantineKey       string     `json:"quarantine_key"`
	JobID               uuid.UUID  `json:"job_id"`
	ValidationUntil     *time.Time `json:"validation_until"`
}

func (v Validator) Handle(ctx context.Context, event platformqueue.Event) error {
	if v.Store == nil || v.Scanner == nil {
		return errors.New("storage and antivirus are required")
	}
	var payload struct {
		VersionID uuid.UUID `json:"version_id"`
	}
	if json.Unmarshal(event.Payload, &payload) != nil || payload.VersionID == uuid.Nil || event.CompanyID == uuid.Nil {
		return errors.New("invalid asset validation event")
	}
	owner := uuid.New()
	var row validationRow
	err := g.DB().Transaction(ctx, func(_ context.Context, tx gdb.TX) error {
		if _, err := tx.Exec("SELECT set_config('app.company_id', ?, true)", event.CompanyID.String()); err != nil {
			return err
		}
		if err := tx.Model("marketing_asset_versions v").InnerJoin("marketing_assets a", "a.company_id=v.company_id AND a.id=v.asset_id").Fields("v.id,v.asset_id,v.status,v.declared_content_type,v.declared_size,v.quarantine_key,v.job_id,v.validation_until").Where("v.company_id", event.CompanyID).Where("v.id", payload.VersionID).WhereNull("a.deleted_at").WhereNull("v.deleted_at").LockUpdate().Scan(&row); err != nil {
			return err
		}
		if row.Status != "processing" {
			return nil
		}
		if row.JobID != event.ID {
			return errors.New("asset validation event does not own this version")
		}
		if row.ValidationUntil != nil && row.ValidationUntil.After(time.Now()) {
			return ErrValidationBusy
		}
		_, err := tx.Model("marketing_asset_versions").Where("company_id", event.CompanyID).Where("id", row.ID).Data(g.Map{"validation_owner": owner, "validation_until": time.Now().UTC().Add(6 * time.Minute), "updated_at": time.Now().UTC()}).Update()
		return err
	})
	if err != nil || row.Status != "processing" {
		return err
	}
	defer func() {
		releaseCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		_ = g.DB().Transaction(releaseCtx, func(_ context.Context, tx gdb.TX) error {
			if _, err := tx.Exec("SELECT set_config('app.company_id', ?, true)", event.CompanyID.String()); err != nil {
				return err
			}
			_, err := tx.Model("marketing_asset_versions").Where("company_id", event.CompanyID).Where("id", row.ID).Where("validation_owner", owner).Data(g.Map{"validation_owner": nil, "validation_until": nil}).Update()
			return err
		})
	}()
	if !strings.HasPrefix(row.QuarantineKey, "companies/"+event.CompanyID.String()+"/quarantine/") {
		return errors.New("quarantine key is outside workspace")
	}
	body, _, err := v.Store.Get(ctx, row.QuarantineKey)
	if err != nil {
		return err
	}
	defer body.Close()
	file, err := os.CreateTemp("", "marketing-upload-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	defer file.Close()
	digest := sha256.New()
	size, err := io.Copy(io.MultiWriter(file, digest), io.LimitReader(body, MaxUploadBytes+1))
	if err != nil {
		return err
	}
	if size < 1 || size > MaxUploadBytes || size != row.DeclaredSize {
		return v.reject(ctx, event.CompanyID, row.ID, owner, row.QuarantineKey, "File size does not match the approved upload limit")
	}
	contentType, width, height, err := validateFile(ctx, file, row.DeclaredContentType)
	if err != nil {
		if errors.Is(err, ErrMediaToolUnavailable) {
			return err
		}
		return v.reject(ctx, event.CompanyID, row.ID, owner, row.QuarantineKey, "File type or media structure is invalid")
	}
	if _, err = file.Seek(0, io.SeekStart); err != nil {
		return err
	}
	if err = v.Scanner.Scan(ctx, file); err != nil {
		if errors.Is(err, antivirus.ErrInfected) {
			return v.reject(ctx, event.CompanyID, row.ID, owner, row.QuarantineKey, "File failed antivirus validation")
		}
		return err
	}
	if _, err = file.Seek(0, io.SeekStart); err != nil {
		return err
	}
	objectKey := "companies/" + event.CompanyID.String() + "/assets/" + row.AssetID.String() + "/" + row.ID.String() + "/" + owner.String()
	if err = v.Store.Put(ctx, objectKey, file, contentType); err != nil {
		return err
	}
	err = g.DB().Transaction(ctx, func(_ context.Context, tx gdb.TX) error {
		if _, err := tx.Exec("SELECT set_config('app.company_id', ?, true)", event.CompanyID.String()); err != nil {
			return err
		}
		result, err := tx.Model("marketing_asset_versions").Where("company_id", event.CompanyID).Where("id", row.ID).Where("validation_owner", owner).Where("validation_until > NOW()").Where("status", "processing").Data(g.Map{"status": "ready", "object_key": objectKey, "sha256": hex.EncodeToString(digest.Sum(nil)), "byte_size": size, "content_type": contentType, "width": width, "height": height, "validation_owner": nil, "validation_until": nil, "updated_at": time.Now().UTC()}).Update()
		if err != nil {
			return err
		}
		changed, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if changed != 1 {
			return ErrValidationLeaseLost
		}
		return nil
	})
	if err != nil {
		cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		_ = v.Store.Delete(cleanup, objectKey)
		return err
	}
	_ = v.Store.Delete(ctx, row.QuarantineKey)
	return nil
}

func (v Validator) reject(ctx context.Context, companyID, versionID, owner uuid.UUID, key, reason string) error {
	err := g.DB().Transaction(ctx, func(_ context.Context, tx gdb.TX) error {
		if _, err := tx.Exec("SELECT set_config('app.company_id', ?, true)", companyID.String()); err != nil {
			return err
		}
		result, err := tx.Model("marketing_asset_versions").Where("company_id", companyID).Where("id", versionID).Where("validation_owner", owner).Where("validation_until > NOW()").Where("status", "processing").Data(g.Map{"status": "rejected", "rejection_reason": reason, "validation_owner": nil, "validation_until": nil, "updated_at": time.Now().UTC()}).Update()
		if err != nil {
			return err
		}
		changed, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if changed != 1 {
			return ErrValidationLeaseLost
		}
		return nil
	})
	if err != nil {
		return err
	}
	_ = v.Store.Delete(ctx, key)
	return nil
}

func validateFile(ctx context.Context, file *os.File, expected string) (string, int, int, error) {
	header := make([]byte, 512)
	count, err := file.ReadAt(header, 0)
	if err != nil && err != io.EOF {
		return "", 0, 0, err
	}
	detected := http.DetectContentType(header[:count])
	if detected != expected || Kind(detected) == "" {
		return "", 0, 0, errors.New("unsupported or mismatched file content")
	}
	if Kind(detected) == "image" {
		if _, err = file.Seek(0, io.SeekStart); err != nil {
			return "", 0, 0, err
		}
		config, _, err := image.DecodeConfig(file)
		if err != nil || config.Width < 1 || config.Height < 1 || config.Width > 8192 || config.Height > 8192 || int64(config.Width)*int64(config.Height) > 25_000_000 {
			return "", 0, 0, errors.New("invalid image dimensions")
		}
		if _, err = file.Seek(0, io.SeekStart); err != nil {
			return "", 0, 0, err
		}
		if _, _, err = image.Decode(file); err != nil {
			return "", 0, 0, err
		}
		return detected, config.Width, config.Height, nil
	}
	if detected == "video/mp4" {
		probeCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
		defer cancel()
		command := exec.CommandContext(probeCtx, "ffprobe", "-v", "error", "-show_entries", "format=duration:stream=codec_type,width,height", "-of", "json", file.Name())
		output := &limitedOutput{limit: 64 * 1024}
		command.Stdout = output
		if err = command.Run(); err != nil {
			if errors.Is(err, exec.ErrNotFound) || probeCtx.Err() != nil {
				return "", 0, 0, ErrMediaToolUnavailable
			}
			return "", 0, 0, err
		}
		var info struct {
			Streams []struct {
				CodecType string `json:"codec_type"`
				Width     int    `json:"width"`
				Height    int    `json:"height"`
			} `json:"streams"`
			Format struct {
				Duration string `json:"duration"`
			} `json:"format"`
		}
		if json.Unmarshal(output.Bytes(), &info) != nil {
			return "", 0, 0, errors.New("invalid video metadata")
		}
		duration, err := strconv.ParseFloat(info.Format.Duration, 64)
		if err != nil || duration <= 0 || duration > 3600 {
			return "", 0, 0, errors.New("invalid video duration")
		}
		for _, stream := range info.Streams {
			if stream.CodecType == "video" && stream.Width > 0 && stream.Height > 0 && stream.Width <= 8192 && stream.Height <= 8192 {
				return detected, stream.Width, stream.Height, nil
			}
		}
		return "", 0, 0, errors.New("video stream missing")
	}
	return detected, 0, 0, nil
}

type limitedOutput struct {
	bytes.Buffer
	limit int
}

func (b *limitedOutput) Write(data []byte) (int, error) {
	if len(data) > b.limit-b.Len() {
		return 0, fmt.Errorf("media metadata exceeds limit")
	}
	return b.Buffer.Write(data)
}
