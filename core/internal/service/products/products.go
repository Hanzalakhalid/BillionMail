package products

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/url"
	"regexp"
	"strings"
	"time"

	"billionmail-core/internal/service/tenancy"
	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/google/uuid"
)

var ErrConflict = errors.New("product revision or idempotency key conflicts with an existing change")
var ErrUnavailable = errors.New("product is unavailable in this workspace")
var pricePattern = regexp.MustCompile(`^(0|[1-9][0-9]{0,14})(\.[0-9]{1,4})?$`)
var currencyPattern = regexp.MustCompile(`^[A-Z]{3}$`)

type Input struct {
	ID              uuid.UUID   `json:"id,omitempty"`
	Revision        int64       `json:"revision"`
	Name            string      `json:"name"`
	SKU             string      `json:"sku"`
	Description     string      `json:"description"`
	Benefits        []string    `json:"benefits"`
	URL             string      `json:"url"`
	Price           string      `json:"price"`
	Currency        string      `json:"currency"`
	AssetVersionIDs []uuid.UUID `json:"asset_version_ids"`
}
type Product struct {
	Input
	VersionID uuid.UUID `json:"version_id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
type Page struct {
	Items      []Product `json:"items"`
	NextCursor string    `json:"next_cursor"`
}
type Version struct {
	ID        uuid.UUID `json:"id"`
	Version   int64     `json:"version"`
	Product   Product   `json:"product"`
	CreatedAt time.Time `json:"created_at"`
}

func Validate(input Input) error {
	if strings.TrimSpace(input.Name) == "" || len(input.Name) > 200 || len(input.SKU) > 160 || len(input.Description) > 16000 || len(input.URL) > 2048 || input.Revision < 0 || len(input.Benefits) > 30 || len(input.AssetVersionIDs) > 20 {
		return errors.New("valid product name, revision and bounded product details are required")
	}
	if (input.Price == "") != (input.Currency == "") || (input.Price != "" && (!pricePattern.MatchString(input.Price) || !currencyPattern.MatchString(input.Currency))) {
		return errors.New("price must be a nonnegative decimal with up to four places and a three-letter currency")
	}
	if input.URL != "" {
		parsed, err := url.Parse(input.URL)
		if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Hostname() == "" || parsed.User != nil {
			return errors.New("product URL must use HTTP without credentials")
		}
	}
	for _, benefit := range input.Benefits {
		if strings.TrimSpace(benefit) == "" || len(benefit) > 500 {
			return errors.New("product benefits must contain 1 to 500 characters")
		}
	}
	seen := map[uuid.UUID]bool{}
	for _, id := range input.AssetVersionIDs {
		if id == uuid.Nil || seen[id] {
			return errors.New("asset versions must be unique nonempty UUIDs")
		}
		seen[id] = true
	}
	return nil
}

func tenantTx(ctx context.Context, company uuid.UUID, run func(gdb.TX) error) error {
	if company == uuid.Nil {
		return tenancy.ErrTenantContextMissing
	}
	return g.DB().Transaction(ctx, func(_ context.Context, tx gdb.TX) error {
		if _, err := tx.Exec("SELECT set_config('app.company_id', ?, true)", company.String()); err != nil {
			return err
		}
		return run(tx)
	})
}

func Get(ctx context.Context, tenant tenancy.TenantContext, id uuid.UUID) (Product, error) {
	if err := tenancy.Authorize(tenant, "product.read"); err != nil {
		return Product{}, err
	}
	var result Product
	err := tenantTx(ctx, tenant.CompanyID, func(tx gdb.TX) error {
		var row struct {
			Snapshot json.RawMessage `json:"snapshot"`
		}
		err := tx.Model("marketing_products p").InnerJoin("marketing_product_versions v", "v.id=p.current_version_id AND v.company_id=p.company_id").Fields("v.snapshot").Where("p.company_id", tenant.CompanyID).Where("p.id", id).WhereNull("p.deleted_at").Scan(&row)
		if errors.Is(err, sql.ErrNoRows) || (err == nil && len(row.Snapshot) == 0) {
			return ErrUnavailable
		}
		if err != nil {
			return err
		}
		return json.Unmarshal(row.Snapshot, &result)
	})
	return result, err
}

func Save(ctx context.Context, tenant tenancy.TenantContext, key string, input Input) (Product, error) {
	if err := tenancy.Authorize(tenant, "product.manage"); err != nil {
		return Product{}, err
	}
	input.Name = strings.TrimSpace(input.Name)
	input.SKU = strings.TrimSpace(input.SKU)
	input.URL = strings.TrimSpace(input.URL)
	if input.Benefits == nil {
		input.Benefits = []string{}
	}
	if input.AssetVersionIDs == nil {
		input.AssetVersionIDs = []uuid.UUID{}
	}
	if err := Validate(input); err != nil {
		return Product{}, err
	}
	if key == "" || len(key) > 128 || tenant.UserID == uuid.Nil {
		return Product{}, errors.New("actor and idempotency key are required")
	}
	request, _ := json.Marshal(input)
	digest := sha256.Sum256(request)
	hash := hex.EncodeToString(digest[:])
	var result Product
	err := tenantTx(ctx, tenant.CompanyID, func(tx gdb.TX) error {
		// Serialize a workspace's catalog mutations, including SKU claims and
		// receipt lookup, without serializing other workspaces' catalogs.
		if _, err := tx.Exec("SELECT pg_advisory_xact_lock(hashtextextended(?,0))", tenant.CompanyID.String()+":products"); err != nil {
			return err
		}
		count, err := tx.Model("marketing_mutation_receipts").Where("company_id", tenant.CompanyID).Where("operation", "product.save").Where("idempotency_key", key).Count()
		if err != nil {
			return err
		}
		if count > 0 {
			var receipt struct {
				RequestHash string          `json:"request_hash"`
				Response    json.RawMessage `json:"response"`
			}
			if err := tx.Model("marketing_mutation_receipts").Where("company_id", tenant.CompanyID).Where("operation", "product.save").Where("idempotency_key", key).Scan(&receipt); err != nil {
				return err
			}
			if receipt.RequestHash != hash {
				return ErrConflict
			}
			return json.Unmarshal(receipt.Response, &result)
		}
		now := time.Now().UTC()
		created := now
		if input.ID != uuid.Nil {
			var current struct {
				Revision  int64     `json:"revision"`
				CreatedAt time.Time `json:"created_at"`
			}
			err := tx.Model("marketing_products").Where("company_id", tenant.CompanyID).Where("id", input.ID).WhereNull("deleted_at").LockUpdate().Scan(&current)
			if errors.Is(err, sql.ErrNoRows) {
				return ErrUnavailable
			}
			if err != nil {
				return err
			}
			if current.Revision != input.Revision {
				return ErrConflict
			}
			created = current.CreatedAt
		} else if input.Revision != 0 {
			return ErrConflict
		}
		if input.SKU != "" {
			query := tx.Model("marketing_products").Where("company_id", tenant.CompanyID).Where("LOWER(sku)=?", strings.ToLower(input.SKU)).WhereNull("deleted_at")
			if input.ID != uuid.Nil {
				query = query.WhereNot("id", input.ID)
			}
			count, err := query.Count()
			if err != nil {
				return err
			}
			if count > 0 {
				return ErrConflict
			}
		}
		if len(input.AssetVersionIDs) > 0 {
			// Lock parent assets as well as versions, so deletion cannot race a new
			// reference. Each immutable version keeps the exact reviewed product image.
			var references []struct {
				ID uuid.UUID `json:"id"`
			}
			err := tx.Model("marketing_asset_versions v").InnerJoin("marketing_assets a", "a.company_id=v.company_id AND a.id=v.asset_id").Fields("v.id").Where("v.company_id", tenant.CompanyID).WhereIn("v.id", input.AssetVersionIDs).Where("v.status", "ready").Where("a.kind", "image").WhereNull("a.deleted_at").WhereNull("v.deleted_at").Order("v.id").LockUpdate().Scan(&references)
			if err != nil {
				return err
			}
			if len(references) != len(input.AssetVersionIDs) {
				return errors.New("all product references must be validated images in this workspace")
			}
		}
		newProduct := input.ID == uuid.Nil
		if newProduct {
			input.ID = uuid.New()
		}
		input.Revision++
		result = Product{Input: input, VersionID: uuid.New(), CreatedAt: created, UpdatedAt: now}
		if newProduct {
			if _, err := tx.Model("marketing_products").Data(g.Map{"id": input.ID, "company_id": tenant.CompanyID, "name": input.Name, "sku": input.SKU, "revision": input.Revision, "created_by": tenant.UserID, "updated_by": tenant.UserID, "created_at": now, "updated_at": now}).Insert(); err != nil {
				return err
			}
		}
		snapshot, err := json.Marshal(result)
		if err != nil {
			return err
		}
		if _, err := tx.Model("marketing_product_versions").Data(g.Map{"id": result.VersionID, "company_id": tenant.CompanyID, "product_id": input.ID, "version": input.Revision, "snapshot": string(snapshot), "created_by": tenant.UserID}).Insert(); err != nil {
			return err
		}
		for position, id := range input.AssetVersionIDs {
			if _, err := tx.Model("marketing_product_media").Data(g.Map{"id": uuid.New(), "company_id": tenant.CompanyID, "product_version_id": result.VersionID, "asset_version_id": id, "position": position, "created_by": tenant.UserID}).Insert(); err != nil {
				return err
			}
		}
		if _, err := tx.Model("marketing_product_sources").Data(g.Map{"id": uuid.New(), "company_id": tenant.CompanyID, "product_version_id": result.VersionID, "kind": "manual", "created_by": tenant.UserID}).Insert(); err != nil {
			return err
		}
		if input.URL != "" {
			if _, err := tx.Model("marketing_product_sources").Data(g.Map{"id": uuid.New(), "company_id": tenant.CompanyID, "product_version_id": result.VersionID, "kind": "url", "url": input.URL, "created_by": tenant.UserID}).Insert(); err != nil {
				return err
			}
		}
		if _, err := tx.Model("marketing_products").Where("company_id", tenant.CompanyID).Where("id", input.ID).Data(g.Map{"name": input.Name, "sku": input.SKU, "revision": input.Revision, "current_version_id": result.VersionID, "updated_at": now, "updated_by": tenant.UserID}).Update(); err != nil {
			return err
		}
		if _, err := tx.Model("marketing_mutation_receipts").Data(g.Map{"company_id": tenant.CompanyID, "operation": "product.save", "idempotency_key": key, "request_hash": hash, "response": string(snapshot), "created_by": tenant.UserID}).Insert(); err != nil {
			return err
		}
		_, err = tx.Model("audit_logs").Data(g.Map{"company_id": tenant.CompanyID, "actor_user_id": tenant.UserID, "action": "product.saved", "resource_type": "product", "resource_id": input.ID.String(), "metadata": g.Map{"revision": input.Revision, "version_id": result.VersionID}}).Insert()
		return err
	})
	return result, err
}

func decodeCursor(cursor string) (time.Time, uuid.UUID, error) {
	if cursor == "" {
		return time.Time{}, uuid.Nil, nil
	}
	if len(cursor) > 256 {
		return time.Time{}, uuid.Nil, errors.New("invalid product cursor")
	}
	decoded, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return time.Time{}, uuid.Nil, errors.New("invalid product cursor")
	}
	parts := strings.Split(string(decoded), "|")
	if len(parts) != 2 {
		return time.Time{}, uuid.Nil, errors.New("invalid product cursor")
	}
	timestamp, err := time.Parse(time.RFC3339Nano, parts[0])
	if err != nil {
		return time.Time{}, uuid.Nil, errors.New("invalid product cursor")
	}
	id, err := uuid.Parse(parts[1])
	if err != nil || id == uuid.Nil {
		return time.Time{}, uuid.Nil, errors.New("invalid product cursor")
	}
	return timestamp, id, nil
}

func List(ctx context.Context, tenant tenancy.TenantContext, cursor string, limit int) (Page, error) {
	if err := tenancy.Authorize(tenant, "product.read"); err != nil {
		return Page{}, err
	}
	if limit == 0 {
		limit = 30
	}
	if limit < 1 || limit > 100 {
		return Page{}, errors.New("product page size must be between 1 and 100")
	}
	timestamp, id, err := decodeCursor(cursor)
	if err != nil {
		return Page{}, err
	}
	page := Page{Items: []Product{}}
	err = tenantTx(ctx, tenant.CompanyID, func(tx gdb.TX) error {
		var rows []struct {
			Snapshot  json.RawMessage `json:"snapshot"`
			CreatedAt time.Time       `json:"created_at"`
			ID        uuid.UUID       `json:"id"`
		}
		query := tx.Model("marketing_products p").InnerJoin("marketing_product_versions v", "v.id=p.current_version_id AND v.company_id=p.company_id").Fields("p.id,p.created_at,v.snapshot").Where("p.company_id", tenant.CompanyID).WhereNull("p.deleted_at")
		if cursor != "" {
			query = query.Where("(p.created_at,p.id)<(?,?)", timestamp, id)
		}
		if err := query.Order("p.created_at DESC,p.id DESC").Limit(limit + 1).Scan(&rows); err != nil {
			return err
		}
		for i, row := range rows {
			if i == limit {
				last := rows[limit-1]
				page.NextCursor = base64.RawURLEncoding.EncodeToString([]byte(last.CreatedAt.UTC().Format(time.RFC3339Nano) + "|" + last.ID.String()))
				break
			}
			var product Product
			if err := json.Unmarshal(row.Snapshot, &product); err != nil {
				return err
			}
			page.Items = append(page.Items, product)
		}
		return nil
	})
	return page, err
}

func Versions(ctx context.Context, tenant tenancy.TenantContext, id uuid.UUID, before int64) ([]Version, error) {
	if err := tenancy.Authorize(tenant, "product.read"); err != nil {
		return nil, err
	}
	result := make([]Version, 0)
	err := tenantTx(ctx, tenant.CompanyID, func(tx gdb.TX) error {
		var rows []struct {
			ID        uuid.UUID       `json:"id"`
			Version   int64           `json:"version"`
			Snapshot  json.RawMessage `json:"snapshot"`
			CreatedAt time.Time       `json:"created_at"`
		}
		query := tx.Model("marketing_product_versions v").InnerJoin("marketing_products p", "p.company_id=v.company_id AND p.id=v.product_id").Fields("v.id,v.version,v.snapshot,v.created_at").Where("v.company_id", tenant.CompanyID).Where("v.product_id", id).WhereNull("p.deleted_at")
		if before > 0 {
			query = query.WhereLT("v.version", before)
		}
		if err := query.OrderDesc("v.version").Limit(30).Scan(&rows); err != nil {
			return err
		}
		for _, row := range rows {
			var product Product
			if err := json.Unmarshal(row.Snapshot, &product); err != nil {
				return err
			}
			result = append(result, Version{ID: row.ID, Version: row.Version, Product: product, CreatedAt: row.CreatedAt})
		}
		return nil
	})
	return result, err
}

func Delete(ctx context.Context, tenant tenancy.TenantContext, id uuid.UUID) error {
	if err := tenancy.Authorize(tenant, "product.manage"); err != nil {
		return err
	}
	return tenantTx(ctx, tenant.CompanyID, func(tx gdb.TX) error {
		var row struct {
			ID        uuid.UUID  `json:"id"`
			DeletedAt *time.Time `json:"deleted_at"`
		}
		err := tx.Model("marketing_products").Unscoped().Where("company_id", tenant.CompanyID).Where("id", id).LockUpdate().Scan(&row)
		if errors.Is(err, sql.ErrNoRows) || (err == nil && row.ID == uuid.Nil) {
			return ErrUnavailable
		}
		if err != nil {
			return err
		}
		if row.DeletedAt != nil {
			return nil
		}
		if _, err := tx.Model("marketing_products").Where("company_id", tenant.CompanyID).Where("id", id).Data(g.Map{"deleted_at": time.Now().UTC(), "updated_at": time.Now().UTC(), "updated_by": tenant.UserID}).Update(); err != nil {
			return err
		}
		_, err = tx.Model("audit_logs").Data(g.Map{"company_id": tenant.CompanyID, "actor_user_id": tenant.UserID, "action": "product.deleted", "resource_type": "product", "resource_id": id.String()}).Insert()
		return err
	})
}
