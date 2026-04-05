# migrations/ — Design

## Conventions

- Sequential numbering: `001_`, `002_`, etc.
- Each migration has `.up.sql` and `.down.sql`.
- Down migrations must be safe to run (use `IF EXISTS`).
- `001_initial` creates all five base tables: sellers, listings, transactions, ownership, buy_orders.
- Embedded in binary via Go `embed.FS` in `db.go`.
