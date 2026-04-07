# api/ — Design

HTTP API layer using chi v5 router.

## Conventions

- All endpoints under `/api/v1` prefix.
- JSON request/response bodies with `Content-Type: application/json`.
- Error responses: `{"error": "message"}`.
- Paginated list endpoints return `{"data": [...], "total": N, "limit": N, "offset": N}`.
- Input validation at handler level before service calls.
- chi URL params for resource IDs, query params for filters/pagination.
- 100MB max for multipart file uploads.

## Structured Request and Response Types

**Every endpoint is backed by named Go structs for both its request and its
response.** A reader should be able to open one file in this directory and
see exactly what an endpoint accepts and returns. This is the same rule
the architect persona enforces (see `.claude/skills/architect/SKILL.md` —
"Structured API Contracts") and it applies uniformly to every API in this
service.

Concretely:

- Request and response types live in `<resource>_types.go` files in this
  package — `listing_types.go`, `search_types.go`, `purchase_types.go`,
  `buyorder_types.go`. They are named (not anonymous structs in the
  handler) so they can be referenced from tests, CLI tools, and docs.
- POST/PUT bodies decode directly into the named request type.
- GET list endpoints have a corresponding `ListXxxRequest` struct populated
  by a `parseListXxxRequest(*http.Request)` helper from query parameters.
  Query strings are still strings, but the typed object is the contract.
- Validation lives on the typed request object via a `Validate() error`
  method (or a `validateXxxRequest` helper for type aliases). Handlers
  call `Validate` before invoking the service.
- Update endpoints use a domain-level update struct with pointer fields
  (`domain.ListingUpdate`) so callers can patch a subset of fields without
  the handler ever seeing a `map[string]interface{}`.
- The only `map[string]interface{}` payloads in the package are the
  `/ready` healthcheck output (a deliberate diagnostic response) and the
  pagination wrapper's `Data` field, which is filled with a typed slice
  by the handler.

This rule is non-negotiable. The API surface is what buyers, sellers,
internal services, and CLI tools program against — inconsistency here
compounds, and stringly-typed payloads block the CLI work that needs to
follow this PR.

## Error Codes

| Status | Meaning |
|--------|---------|
| 400 | Invalid input (missing fields, bad format) |
| 404 | Resource not found |
| 409 | Conflict (duplicate, invalid state transition) |
| 403 | Forbidden (e.g., downloading unowned data) |
| 500 | Internal server error |

## Open Follow-Ups

- **Buyer authentication.** `buyer_id` is currently trusted from the
  request body. A real auth flow (OAuth or similar) is required before
  MVP — tracked as a follow-up issue.
- **CORS / security posture.** A `security.md` should be added to this
  directory documenting how CORS is configured, what we have today, and
  what is intentionally missing — tracked as a follow-up issue.
