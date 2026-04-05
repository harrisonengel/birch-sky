# internal/ — Design

All non-main application code lives here, following Go convention for unexported packages.

## Package Layout

| Package | Purpose |
|---------|---------|
| `config` | Environment-based configuration |
| `domain` | Pure domain types, no external dependencies |
| `storage/postgres` | PostgreSQL repositories (sqlx) |
| `storage/objectstore` | S3-compatible object storage (MinIO) |
| `search` | OpenSearch client, embedder, indexer, hybrid search |
| `payments` | Stripe payment processing |
| `service` | Business logic orchestration layer |
| `api` | HTTP handlers, router, middleware |
| `mcp` | MCP SSE server and tool definitions |

## Dependency Direction

`api` / `mcp` → `service` → `storage` + `search` + `payments` → `domain`

Domain types have no dependencies. Services depend on interfaces, not implementations.
