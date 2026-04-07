# storage/ — Design

Storage layer provides persistence interfaces and implementations.

## Sub-packages

- `postgres/` — relational data via sqlx with named parameters
- `objectstore/` — S3-compatible file storage via MinIO client

## Interface Contracts

Services depend on repository interfaces defined at the service layer. Storage implementations satisfy those interfaces. This allows test doubles without mocking frameworks.
