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

## Error Codes

| Status | Meaning |
|--------|---------|
| 400 | Invalid input (missing fields, bad format) |
| 404 | Resource not found |
| 409 | Conflict (duplicate, invalid state transition) |
| 403 | Forbidden (e.g., downloading unowned data) |
| 500 | Internal server error |
