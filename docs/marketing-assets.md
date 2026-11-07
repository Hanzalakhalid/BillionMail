# Asset library and upload validation

The Asset Library stores workspace-owned product references and campaign media. A file is available for preview only after validation finishes. New provider generation adapters will create additional versions through this same library.

## API

All paths are under `/api/v1/assets` and require an authenticated workspace membership. Owners, administrators, and managers can upload and remove assets. Viewers can list and download validated files. Access is checked again in each service operation.

| Method and path | Result |
| --- | --- |
| `POST /uploads` | Direct upload resource containing `asset_id`, `version_id`, status, expiry, and a signed multipart POST form. Requires `Idempotency-Key`. Reusing a key with different requirements returns HTTP 409. |
| `POST /uploads/complete` | HTTP 202 with `job_id`, `resource_id`, job `status`, and `resource_status`. Repeating completion returns the same job. |
| `GET /list?cursor=...&limit=30` | Direct `{items,next_cursor}` page, ordered by creation time and ID. Page size is 1–100. |
| `GET /download?version_id=...` | Direct `{url}` response with a five-minute signed URL. Quarantined, rejected, and removed assets cannot obtain new URLs. |
| `POST /delete` | Removes the asset from the library. Repeating the operation is harmless. Existing signed links expire within five minutes; permanent object retention is separate. |

Upload input contains `title`, `content_type`, and `byte_size`. The browser sends every returned form field, followed by the `file` part, directly to storage. Application authentication tokens must not be sent to storage. After storage accepts the file, the browser calls `/uploads/complete` with `version_id`. Mutation requests accept an `Idempotency-Key`; completion and removal also enforce idempotency through their resource transitions.

Current upload limits are 20 MiB, PNG/JPEG/WebP images, MP4 video, and PDF documents. Images are limited to 8192 pixels per dimension and 25 million pixels total; video is limited to one hour. Generated video export and larger-file quota handling remain part of the general media studio milestone.

## Validation and storage

The storage-verified POST policy binds the exact quarantine key, MIME type, maximum byte size, and expiry. The media worker then checks actual size and content, decodes images, inspects videos with FFprobe, and streams bytes to ClamAV. Scanner errors cause retries; detected malware and invalid files remain unavailable. The scanner implementation follows the [documented ClamD streaming protocol](https://docs.clamav.net/manual/Usage/ClamdProtocol.html).

Validated bytes are copied to a unique key for the validation attempt before the database publishes the immutable version. A lease-owner check prevents late workers from publishing their attempts, and stale attempts remove only their own objects. PostgreSQL prevents changes to a ready version's content and provenance fields. Asset removal is recorded in the audit log and hides the asset immediately from new API access.

Hourly schedules remove quarantine objects after their upload signatures expire. Incomplete processing gets a 24-hour grace period and is skipped while its validation lease is live. Permanent objects retain administrator-controlled storage retention during the legacy workspace releases.

`MEDIA_ENDPOINT` is the internal S3 origin. `MEDIA_PUBLIC_ENDPOINT` is the browser-reachable origin used in signed forms and download URLs. Compose defaults to private `http://minio:9000` operations and public `http://localhost:29000` signatures. Set the latter to the deployment's reachable storage origin. `CLAMD_ADDR` is private to the media worker and defaults in Compose to `clamav:3310`.

## Evidence and remaining gates

Database tests exercise duplicate upload/completion/removal, cursor pagination, workspace access, rejected content, scanner outage recovery, ready-version immutability, quarantine cleanup, and validation fencing. Storage tests use real MinIO to prove signed MIME/size policies, upload/download parity, and deletion. Scanner unit tests validate wire framing and failure handling against a repository-owned protocol fixture.

The live ClamAV clean-stream and EICAR fixture checks pass via `docker compose -f compose.test.yml -p billionmail-av-test --profile antivirus run --rm antivirus-check`. This uses definitions supplied by the official image for repeatable tests; deployed ClamAV retains automatic definition updates. `core/test/media-runtime.cjs` verifies actual browser rendering, screenshots, FFprobe and ImageMagick in the media image.

Browser acceptance against the complete stack, worker process-kill recovery, generation adapters, version editing, and media exports remain unverified or unimplemented. No provider publication or marketing send is authorized by uploading an asset.
