# api — public HTTP gateway

The API gateway is the only thing buyers reach directly. It owns:

- Brief schema validation
- Job creation and queueing
- Job status / verdict retrieval
- Objective discovery
- Per-job audit retrieval (helpful for the demo, locked down in
  prod)

The MVP uses `net/http` with the standard library router. Auth is a
single static API key checked by middleware. Swapping in JWT
verification is one middleware change.

## Endpoints

| Method | Path                  | Purpose                              |
|--------|-----------------------|--------------------------------------|
| POST   | /v1/briefs            | Submit a brief, return `{job_id}`    |
| GET    | /v1/jobs/{id}         | Status + verdict (when complete)     |
| GET    | /v1/objectives        | List registered templates            |
| GET    | /v1/audit/{job_id}    | Audit trail for one job              |
| GET    | /healthz              | Liveness                             |
