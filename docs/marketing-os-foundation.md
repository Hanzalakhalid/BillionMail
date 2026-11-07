# Marketing OS implementation status

This branch contains the first foundation slice of the Autonomous Marketing OS roadmap. It does not yet send marketing campaigns or activate external providers.

## Implemented in this slice

- Migration `00006_marketing_foundation.sql` adds tenant-scoped integration, business profile, project, strategy, and approval-manifest records. Row-level security and composite tenant foreign keys protect new relationships.
- `/api/v1/integrations` supports listing, creating, rotating, and disconnecting provider connections. Credential versions are envelope encrypted; API responses expose metadata only. New connections remain `unverified` until provider-specific verification exists.
- The Vue Integrations page exposes those operations to workspace admins.
- The object-storage package defines S3-compatible operations, workspace-prefixed keys, signed URLs, and a health check.
- Compose starts the existing Asynq worker by default and includes MinIO plus bucket initialization.
- Migration `00007` and the scheduler binary add leased schedule claims, transactionally paired occurrence/outbox records, five-field cron in IANA timezones, and a readiness endpoint. Missed recurring runs coalesce into one occurrence; the next run is calculated after the current time. Only registered internal events are accepted; the initial registry contains the harmless `platform.schedule.probe` operation.
- Migration `00008` adds dispatch generations. Workers record terminal failures in PostgreSQL, and the tenant-authorized replay service creates an audited generation while retaining the original event identity. Stale generations are ignored, and workers load command content from PostgreSQL rather than trusting Redis payload content. An operator API/UI for replay is still pending.
- Identity, invitation, and refresh-token hashes use explicit PostgreSQL binary values. Refresh-token reuse commits revocation of the token family before returning a rejection.
- Workers and scheduler wait for the API to apply all migrations embedded in their release. They do not rerun legacy database bootstrap or administrator setup on restart.
- Migration `00009` adds the tenant asset library, immutable validated versions, and idempotent upload requests. The Vue library supports uploads, validation status, previews, cursor pagination, and removal. Storage policies enforce declared MIME type and size; files remain quarantined until byte/type/media checks and antivirus scanning complete.
- The media worker consumes its own queue and uses fenced validation leases. Each attempt writes a unique permanent object, so a stale attempt cannot replace a published version. Hourly durable cleanup removes expired quarantine objects and expires abandoned uploads while respecting live leases.
- The media image contains Playwright/Chromium, FFmpeg, and ImageMagick. These tools are absent from the API and scheduler images. A ClamAV service supplies streaming malware checks; scanner outages keep files private and retryable.
- Migration `00010` and `/api/v1/business/profile` add the workspace business profile and Vue editor, with brand voice, language, timezone, transactional idempotency receipts, audit entries, and optimistic revisions.
- Migration `00011` and `/api/v1/products` add manual product profiles, immutable versions, source provenance, verified image-version links, cursor pagination and the Vue catalog/history editor. URL extraction, catalog-file import, separate brand profiles, and domain-brand migration remain open.
- Migration `00012` and `/api/v1/projects` add saved discovery requirements, immutable user notes/events, revisions, idempotent commands, cursor pagination and reconnectable SSE. The Marketing Projects page supports requirements and history; AI discovery, strategy generation, durable agent runs and approvals remain open. See [project contracts and limitations](marketing-projects.md).

## Reproducible backend checks

On Windows, run `powershell -NoProfile -ExecutionPolicy Bypass -File scripts/test-backend.ps1`. The runner exports only public trusted root certificates to a temporary directory so corporate TLS inspection works without disabling certificate verification. No host Go installation is required. It builds isolated storage services, creates PostgreSQL/Redis/MinIO with disposable data, runs foundation tests, builds the API, marketing worker, scheduler, and media worker, and removes the services. Go module and compiler caches persist for subsequent runs.

On Linux/macOS, run `docker compose -f compose.test.yml run --rm backend`, followed by `docker compose -f compose.test.yml down --remove-orphans`. Run `down` before rerunning after an interrupted check to obtain a fresh database. If a custom certificate authority is needed, set `BM_TEST_CA_DIRECTORY` to a directory containing `ca-bundle.pem` with trusted public root certificates. `BM_TEST_GO_MOD_CACHE` can point to an existing Go module cache to avoid downloading cached modules again.

Use `scripts/test-backend.ps1 -Full`, or `BM_TEST_FULL=1` with Compose, to run the complete backend suite in short mode before building the binaries. The runner copies backend sources into the test container before compiling, so host edits cannot change a build that is already running.

Database tests cover migration reruns, actual RLS enforcement using a restricted PostgreSQL role, tenant credential isolation and revocation, random refresh-token hash roundtrips, token-family reuse revocation, concurrent scheduler instances, live/expired leases, transaction rollback, archived Redis task replay, stale generation rejection, and authoritative command loading. These are foundation checks; they do not prove customer-visible send recovery across providers.

## Building the branch for Compose

Run `docker compose build` before starting the stack with `docker compose up -d`. The core source Dockerfile builds Vue assets and three Go binaries, then layers them onto the existing BillionMail runtime. API, marketing worker, and scheduler share `billionmail/core:marketing-local`. The media worker uses its own source-built image. `BILLIONMAIL_CORE_IMAGE` and `BILLIONMAIL_MEDIA_IMAGE` select release tags built from these sources. Set `BILLIONMAIL_BUILD_CA_FILE` to a trusted public root bundle when build downloads require a custom certificate authority; Compose supplies it as a BuildKit secret, without disabling TLS verification.

The previous public MinIO and `mc` images no longer pull. The default storage image now builds unmodified upstream Community Edition commit `7aac2a2c5b7c882e68c1ce017d8256be2feea27f`, retaining its AGPL license and source/revision labels. The repository-owned storage initializer creates the bucket through the same S3 adapter. Upstream MinIO is archived, so deployment review must account for its maintenance status. [Upstream repository and licensing](https://github.com/minio/minio)

## Configuration

Set `SAAS_LOCAL_MASTER_KEY` to a unique persistent random 32-byte base64 value in `.env` before saving provider credentials in Compose. Keep this key backed up: losing it makes encrypted credentials unreadable. Production uses `SAAS_KMS_KEY_ARN`. The shared development fallback is deliberately rejected for new provider credentials.

Compose uses `MEDIA_BUCKET=marketing-assets`, `MEDIA_ENDPOINT=http://minio:9000`, and its database credentials for the local MinIO instance. Replace these credentials for a deployed instance. `MEDIA_PUBLIC_ENDPOINT` selects the browser-reachable origin used to sign upload/download URLs; its local default is `http://localhost:29000`. Set it to the externally reachable storage origin on a server. Internal object operations retain the private `minio` endpoint. The MinIO API and console bind to localhost ports 29000 and 29001 by default. Signed URLs expire within one hour, and object keys require a canonical workspace UUID.

## Remaining gates

Provider verification, job administration APIs/UI, Customer 360, campaign adapters, execution manifests, and the rest of the roadmap remain to be implemented. Live ClamAV clean/EICAR checks and actual Chromium/FFprobe/ImageMagick runtime checks pass. The media upload implementation still needs process-termination recovery and complete-stack browser acceptance. Scheduler activation for customer-facing operations requires registered handlers and dispatch-time immutable manifest validation. No external-effect path should be enabled until those checks exist.

Focused Go service tests, PostgreSQL/Redis foundation checks, actual MinIO signed-upload policy checks, frontend tests/builds, and `docker compose config` are the checks for this slice. Complete-stack startup, current source-image runtime validation, process-termination recovery, provider-contract checks, and tenant isolation over every new domain are still required before deployment.
