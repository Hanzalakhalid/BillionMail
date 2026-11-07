# Local frontend and backend routing

The frontend development server targets `http://127.0.0.1:8090` through the `localhost` entry in `core/frontend/build/config.ts`. It proxies `/api` and strips that first prefix. The client separately prefixes backend routes with `/api`.

| Environment | Browser request | Backend route |
|---|---|---|
| Development | `/api/api/v1/business/profile` | `/api/v1/business/profile` |
| Production | `/api/v1/business/profile` | `/api/v1/business/profile` |

The two prefixes in the development request are intentional. Removing one causes the request to reach the backend's static-page fallback instead of its API group. Token refresh retries must preserve the already-prefixed route; the request interceptor now applies its prefix idempotently.

## Missing routes after source changes

Adding a Go controller or migration does not update a running Windows executable. Rebuild and restart the API serving port 8090 whenever backend code changes; restarting only the frontend cannot register new backend routes. Source builds include migrations inside the binary and apply them during API startup.

The local Windows instance was upgraded from the stale `core/billionmail-chatbot-dev-next.exe` to the current sources on November 7, 2026. The existing executable path was retained for its launcher. A prior binary copy is retained as `core/billionmail-chatbot-dev-next.previous.exe` (ignored by Git), and a pre-upgrade PostgreSQL backup was saved in the user's temporary directory. The configured database/Redis connections were preserved; the local provider encryption key comes from the existing root `.env` value and must remain stable.

For container deployments, use the source-build instructions in [marketing-os-foundation.md](marketing-os-foundation.md). Rebuilding an image alone does not replace an already running container; recreate the API/worker/scheduler/media containers using the same release.

Authenticated checks through the frontend proxy verified business, product, asset, project and integration reads, plus creating and disconnecting a dummy integration. These checks do not certify asset processing or live provider calls; the complete storage/media stack and credentials remain separate runtime prerequisites.
