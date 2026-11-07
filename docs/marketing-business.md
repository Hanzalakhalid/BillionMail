# Business profile

The Business Profile screen saves workspace context for marketing projects: business name, website, industry, description, brand voice, default language, and IANA timezone. It does not fetch the supplied website or authorize any customer-facing action.

`GET /api/v1/business/profile` returns `{profile: ...}` or `{profile: null}` when no profile exists. `PUT /api/v1/business/profile` accepts the profile fields and its `revision`, and requires `Idempotency-Key`. A first save uses revision zero. Every successful change increments the revision; a stale revision returns HTTP 409. Retrying the same key and input returns the original resource response without a second mutation or audit entry. Reusing the key with different input also returns 409.

Owners, administrators, and managers can edit. Viewers can read. Every database query scopes the company explicitly and uses transaction-local tenant context for RLS. A transaction advisory lock serializes profile updates, and the mutation receipt commits atomically with the profile and audit entry.

Migration `00010_business_profiles.sql` extends the initial business table and adds tenant-scoped mutation receipts. Existing business rows retain their data and begin at revision one. Websites must be HTTP URLs without embedded credentials; timezone and field lengths are validated server-side. Product imports and website fetching will need their own URL resolution and SSRF controls before any external request occurs.

The profile is one input to the planned AI workspace. Separate brand records, product catalogs, domain-brand migration, conversational discovery and strategy generation remain part of the roadmap.
