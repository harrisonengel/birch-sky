# The Information Exchange

A data marketplace where buyer agents search, trust, and purchase information
from sellers without breaching Arrow's Information Paradox. See
[`CLAUDE.md`](CLAUDE.md) for the project vision.

## Local end-to-end demo (docker compose)

```bash
# optional — buyer-agent + prepper LLM calls need this; demo still boots without it
export ANTHROPIC_API_KEY=sk-ant-...

docker compose up --build
```

Wait until the `seed` service exits `0`, then open
[http://localhost:5173](http://localhost:5173).

The compose stack runs:

| Port   | Service          | Notes                                     |
| ------ | ---------------- | ----------------------------------------- |
| 5173   | frontend         | Vite dev server                           |
| 8080   | market-platform  | Go marketplace API                        |
| 8081   | harness          | Python buyer-agent runner (internal 8000) |
| 8082   | agent-prepper    | Python clarification service (internal 8002) |
| 8088   | market-platform MCP | internal 8081                          |
| 9200   | opensearch       |                                           |
| 5432   | postgres         |                                           |
| 9000/1 | minio            |                                           |

### Auth

The frontend container runs with `VITE_AUTH_MODE=local`, which bypasses the
Cognito gate and stamps a stub `buyer-local-*` ID into sessionStorage. Do not
use this mode outside local development — production deployments must leave
`VITE_AUTH_MODE` unset so Cognito enforcement stays on.

### Reseeding

`iecli seed` inserts sellers and listings via the HTTP API. Re-running
`docker compose up` without wiping volumes will fail the seed step on
duplicate-email conflict. For a clean slate:

```bash
docker compose down -v
docker compose up --build
```

## Tests

```bash
npm test           # Vitest unit tests
npm run test:e2e   # Playwright end-to-end (uses mocked HTTP; no stack required)
```
