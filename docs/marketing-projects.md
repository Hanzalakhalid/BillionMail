# Persistent marketing project discovery

The Marketing Projects page saves a workspace-owned objective, structured discovery requirements and user notes. It is the persistence foundation for the planned discovery/planner/executor/evaluator pipeline. It does not currently call a model or generate strategies, assets or customer-facing actions.

Migration `00012_project_workspace.sql` adds project revisions and an ordered durable event ledger. Conversation and event payloads cannot be edited in place. Every mutation commits its state, event, exact idempotency response and audit entry together. All queries include the company, with PostgreSQL RLS as an additional boundary. Owners, administrators and managers can write; viewers can read.

## API

All routes use the existing authenticated `/api` group and return direct resource JSON.

| Method | Route | Purpose |
|---|---|---|
| GET | `/api/v1/projects/list?cursor=...&limit=30` | Cursor-paginated projects |
| GET | `/api/v1/projects/get?id=...` | Current requirements and missing fields |
| POST | `/api/v1/projects/save` | Create or revise discovery requirements |
| POST | `/api/v1/projects/messages` | Append a user message with `id` and `content` |
| GET | `/api/v1/projects/events?id=...&after=0&limit=100` | Recover durable history |
| GET | `/api/v1/projects/stream?id=...&after=0` | SSE events with durable IDs |

Both POST routes require `Idempotency-Key`. Updates require the current revision; stale revisions and changed reuse of a key conflict. User message roles are assigned by the service, never accepted from the caller. Editing requirements is currently limited to projects in discovery; later state transitions must implement strategy supersession and approval invalidation before enabling edits in those states.

The stream honors `Last-Event-ID`, overriding `after`, and closes each connection after 25 seconds. Reconnect after the last received ID to recover committed events, including after an API restart. It emits `project.updated` and `message.created`; polling the events route uses the same source. The initial page uses explicit history refresh rather than a streaming client. Event IDs are monotonic database IDs, not counts, and may have gaps.

Discovery guidance is deterministic: audience, channels, languages and success measure are shown only when missing. This is not AI extraction from messages. Language tags and channel names are validated. Selecting a channel records intent and does not assert that its execution adapter is available.

## Remaining work

Model-backed adaptive discovery, product/business context snapshots, registered tool schemas, durable agent tasks/checkpoints, strategy versions, comments, planning approval and content generation remain open. No project state currently authorizes a send, publication or spend. Project archival/erasure and retention controls also remain part of the full roadmap.
