# Marketing OS implementation status

This branch contains the first foundation slice of the Autonomous Marketing OS roadmap. It does not yet send marketing campaigns or activate external providers.

## Implemented in this slice

- Migration `00006_marketing_foundation.sql` adds tenant-scoped integration, business profile, project, strategy, and approval-manifest records. Row-level security and composite tenant foreign keys protect new relationships.
- `/api/v1/integrations` supports listing, creating, rotating, and disconnecting provider connections. Credential versions are envelope encrypted; API responses expose metadata only. New connections remain `unverified` until provider-specific verification exists.
- The Vue Integrations page exposes those operations to workspace admins.
- The object-storage package defines S3-compatible operations, workspace-prefixed keys, signed URLs, and a health check.
- Compose starts the existing Asynq worker by default and includes MinIO plus bucket initialization.

## Configuration

Set `SAAS_LOCAL_MASTER_KEY` to a unique persistent random 32-byte base64 value in `.env` before saving provider credentials in Compose. Keep this key backed up: losing it makes encrypted credentials unreadable. Production uses `SAAS_KMS_KEY_ARN`. The shared development fallback is deliberately rejected for new provider credentials.

Compose uses `MEDIA_BUCKET=marketing-assets`, `MEDIA_ENDPOINT=http://minio:9000`, and its database credentials for the local MinIO instance. Replace these credentials for a deployed instance. The MinIO API and console bind to localhost ports 29000 and 29001 by default.

## Remaining gates

The storage adapter is not yet exposed through validated asset upload APIs. Provider verification, jobs, scheduler and media worker, Customer 360, campaign adapters, execution manifests, and the rest of the roadmap remain to be implemented. No external-effect path should be enabled until immutable manifests are checked at dispatch.

Focused Go service tests, frontend lint/build, and `docker compose config` are the checks for this slice. Full database migration, integration, crash-recovery, provider-contract, and tenant-isolation tests are still required before deployment.
