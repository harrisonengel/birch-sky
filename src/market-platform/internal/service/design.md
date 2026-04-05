# service/ — Design

Business logic orchestration layer. Services coordinate between repositories, external integrations, and domain logic.

## Patterns

- Constructor injection: services receive dependencies as constructor arguments.
- Services own multi-step workflows (e.g., purchase: check ownership → create payment → create transaction).
- Non-fatal side-effects (like search indexing after listing creation) are logged but don't fail the primary operation.
- Services return domain types, not HTTP-specific types.

## Services

| Service | Responsibility |
|---------|---------------|
| `ListingService` | CRUD, file upload, triggers search indexing |
| `SearchService` | Hybrid search orchestration (text + vector + RRF fusion) |
| `PurchaseService` | Payment initiation, confirmation, ownership management |
| `BuyOrderService` | Buy order lifecycle (create, fill, cancel) |
