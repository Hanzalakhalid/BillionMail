# Execution ledger — Autonomous Marketing OS roadmap

The user-approved roadmap in this conversation remains the complete objective. This ledger records evidence and sequencing; it does not replace or reduce the requested scope.

## Current checkpoint

- Existing feature branch: `feat/autonomus-marketing-agent`. Continue in the user-selected checkout with its existing changes. Do not relocate or discard the in-progress implementation.
- Phase 0 is in progress: scheduler, dispatch generations/replay service, source builds, asset quarantine/validation, library UI, and isolated PostgreSQL/Redis/MinIO tests exist. Process-kill acceptance, whole-stack browser coverage, replay administration, telemetry and legacy credential migration remain.
- Phase 1 is in progress: business profile and manual product persistence/APIs/editors exist. Product edits preserve immutable versions and exact validated image references, with source records, workspace SKU uniqueness, idempotency receipts, audits, revision conflicts and pagination. Project discovery requirements, notes, durable events, revision/idempotency handling, API/SSE and an initial editor have been added and tested. Product URL extraction/catalog-file import, separate brand profiles, domain-brand migration, AI discovery, strategy/copy/image generation, resumable agent runs, review packages and planning-only approvals remain.
- Phases 2–4 remain open in full, including four-channel manifest-gated campaigns, durable workflows/inbox, Customer 360, events/analytics/reports, pages, commerce, social/ads, billing/compliance and AWS parity.

## Verified evidence

- `scripts/test-backend.ps1 -Full`: complete backend short-mode suite and API/worker/scheduler compilation passed before the asset milestone.
- `scripts/test-backend.ps1`: latest asset HTTP contract, database validation/recovery/cleanup/pagination tests, MinIO signed-policy tests, and API/worker/scheduler/media-worker compilation passed.
- Frontend: latest full run passed 65 files / 487 tests, including project request identities/event cursors, development/production URL composition and token-refresh retries. The production frontend build and targeted lint passed. Production source image build passed before the business/product/project milestones and needs rebuilding for current sources.
- Both current core and media Docker images built. `core/test/media-runtime.cjs` verified actual Chromium rendering/screenshot, FFprobe, and ImageMagick in the non-root media image.
- Live `clamav/clamav:1.5` contract passed clean-stream and benign EICAR detection. Isolated scanner containers were removed afterward.
- Business profile unit/database and direct HTTP revision/idempotency checks passed. A direct restricted-role PostgreSQL transaction confirmed profile/receipt RLS and blocked cross-workspace updates. The latest full short-mode suite and all four runtime binary builds passed.
- Business profile API client tests passed (2 tests), along with the asset/direct-response regression set (13 tests total); the frontend production build with the profile editor passed. Core/media source images were built before the business-profile milestone and must be rebuilt to include it.
- Integration metadata regression: a credential nested inside arrays-of-arrays was accepted before the fix, then rejected after recursive validation; the integration unit package passed after the fix.
- Product HTTP regression: GoFrame conversion of UUID arrays failed before controller dispatch. Request DTOs now use strings with explicit UUID parsing; the binding regression, controller validation and actual HTTP create/retry/revision/history/delete contract passed.
- The full backend short-mode suite and four binary builds passed with the project workspace. A subsequent focused run passed the final UUID normalization, pagination and populated cross-workspace RLS checks, plus immutable history, receipt retries, message role assignment and SSE `Last-Event-ID` recovery. All four runtime binaries compiled again, and isolated test services were cleaned up successfully.
- A read-only project workspace review found no remaining important/critical issues after UUID normalization. Its minor inline-error feedback was also addressed.
- Local 404 repair: the development proxy's `/api/api/...` browser URLs are intentional; the API process on 8090 was an outdated Windows executable. Rebuilt it from current sources, applied migrations after taking a database backup, and replaced/restarted the same executable path. Actual authenticated frontend-proxy checks returned HTTP 200 JSON for business/products/assets/projects/integrations, and dummy integration create/disconnect passed. The local provider key is now supplied from the existing `.env`. A separate token-refresh retry regression was fixed so an already-prefixed URL is not prefixed again. See `docs/local-development.md`.

## Decisions

- Ruling: retain MinIO by building final upstream Community Edition commit `7aac2a2c5b7c882e68c1ce017d8256be2feea27f` — old public MinIO/mc images no longer pull — archived upstream requires deployment maintenance review. Keep AGPL notices/source metadata.
- Ruling: continue the approved plan inline in the current feature checkout — the user explicitly requested implementation on this branch and repeated autonomous continuation — no new worktree or renewed design approval is needed for already specified work.
- Ruling: uploaded source files remain private until byte/type/media/antivirus validation finishes — signed storage policies alone do not validate bytes — scanner outages retry without publication.
- Ruling: new domain APIs return direct resource JSON with 202 for queued work, while legacy envelopes remain compatible through explicit client response handling.

## Next implementation tasks

1. Persist business/brand/product context with tenant permissions, audit metadata, idempotent mutations and verified asset references.
   - Business editor and manual product catalog implemented. Separate brands, domain migration, URL extraction and catalog files remain.
2. Add adaptive AI discovery, business/product context snapshots and durable agent tool/task contracts to the persistent project workspace, followed by controlled AI adapters and review. The current history UI uses explicit refresh; the backend SSE contract exists, while its streaming frontend client remains open.
3. Continue Phase 0 acceptance gaps alongside those vertical flows; never claim a phase gate from tables, route presence or narrow tests.

Full acceptance must be audited against every item in the original roadmap before the active goal can be marked complete. No phase exit gate is claimed by this checkpoint.
