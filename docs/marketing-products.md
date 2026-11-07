# Product catalog

The Products screen saves business-owned product facts and links to validated reference images. It supports creation, editing, version history, cursor pagination, and removal. Product descriptions, benefits, prices and URLs are supplied by the user; saving a URL does not fetch or analyze its contents.

## Records and provenance

Migration `00011_products.sql` creates tenant-scoped products, immutable versions, media links, and source records. Every edit creates a new version with a complete snapshot. The product's current-version pointer changes atomically with its revision, response receipt and audit log. Old versions retain their descriptions and exact asset-version references.

SKU uniqueness is case-insensitive within a workspace and applies to active products. Removal hides the product but retains version history for future campaign provenance. A removed SKU can be used again. Product images must be ready, nonremoved image assets belonging to the same workspace. Uploads still awaiting validation and another workspace's versions are rejected. The service locks reference rows while saving to avoid racing an asset removal.

Prices use decimal strings in the named currency's major unit, without floating-point conversion, exponents or negative amounts. The current record accepts up to four decimal places and a three-letter uppercase currency code; it does not infer taxes, discounts or provider cost estimates. A missing price and currency means price data is unavailable.

Manual changes create a manual source record. A supplied URL also creates an unverified URL source with no verification timestamp. URL extraction, catalog-file import, commerce synchronization, and separate brand profiles remain part of subsequent work.

## APIs

All endpoints require the authenticated workspace. Owners, administrators and managers can edit; viewers can read. Responses are direct JSON resources.

| Method and path | Result |
| --- | --- |
| `GET /api/v1/products/list?cursor=...&limit=30` | `{items,next_cursor}` with creation-time/ID cursor ordering. |
| `GET /api/v1/products/get?id=...` | Current product resource. Missing or removed products return HTTP 404. |
| `POST /api/v1/products/save` | Creates a product or new immutable version. Requires `Idempotency-Key`. New products omit `id` and use revision zero; edits include `id` and the current revision. |
| `GET /api/v1/products/versions?id=...&before=...` | `{items,next_before}` with up to 30 versions ordered newest first. Use the last returned version as `before` to retrieve older versions. |
| `POST /api/v1/products/delete` | Removes a product. Repeating the operation is harmless. |

Save input fields: `name`, `sku`, `description`, `benefits`, `url`, `price`, `currency`, `asset_version_ids`, and `revision`; updates also include `id`. Stale revisions, duplicate active SKUs, and idempotency-key reuse with different content return HTTP 409. Repeating the original key/input returns its original version, including after a subsequent edit. Completed mutations have one audit entry each.

## Verification

Unit tests cover field validation, prices, unsupported URLs, duplicate reference IDs and malformed pagination cursors. PostgreSQL tests exercise creation/update history, immutable snapshots, idempotent concurrent retries, stale revisions, SKU scoping, image readiness, cross-workspace read/write/reference rejection, pagination, removal/retry, and restricted-role RLS across all product tables. HTTP contract tests exercise the real GoFrame resource binding and status codes.

The catalog is internal planning context. It does not authorize customer-facing sends or provider spending. Product context still needs to be connected to the persistent AI project workspace and immutable approval manifests.
