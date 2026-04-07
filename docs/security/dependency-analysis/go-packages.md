# Go Package Dependencies — Security Analysis

Date: 2026-04-05

## Summary

12 packages rated **Low risk**, 1 package rated **Medium risk** (replaced).

| Package | Version | Risk | Notes |
|---------|---------|------|-------|
| `go-chi/chi/v5` | v5.2.5 | Low | 17K+ importers, MIT, no CVEs |
| `jmoiron/sqlx` | v1.4.0 | Low | 25K+ importers, thin wrapper on database/sql |
| `golang-migrate/migrate/v4` | v4.19.1 | Low | Well-established migration tool |
| `opensearch-project/opensearch-go/v4` | v4.6.0 | Low | Official AWS-backed client, Apache 2.0 |
| `minio/minio-go/v7` | v7.0.100 | Low | Official MinIO SDK, actively maintained |
| `stripe/stripe-go/v76` | v76.25.0 | Low | Official Stripe SDK (note: v76 is behind latest v85) |
| `google/uuid` | v1.6.0 | Low | 113K+ importers, Google-maintained |
| `anthropics/anthropic-sdk-go` | v1.30.0 | Low | Official Anthropic SDK, MIT |
| `aws/aws-sdk-go-v2` | v1.41.5 | Low | Official AWS SDK, Apache 2.0 |
| `aws/aws-sdk-go-v2/config` | v1.32.14 | Low | AWS SDK sub-module |
| `aws/aws-sdk-go-v2/service/bedrockruntime` | v1.50.4 | Low | AWS SDK sub-module |
| `testcontainers/testcontainers-go` | v0.41.0 | Low | Test-only, Docker/AtomicJar-backed |
| ~~`mark3labs/mcp-go`~~ | ~~v0.47.0~~ | **Medium** | **Replaced** — pre-v1.0, smaller maintainer, competing SDK had CVEs |
| `modelcontextprotocol/go-sdk` | v1.4.1 | Low | Official MCP SDK, replaced mark3labs/mcp-go |

## Medium Risk Decision

`mark3labs/mcp-go` was replaced with the official `modelcontextprotocol/go-sdk` (v1.4.1) per user decision. The official SDK is maintained by the MCP specification authors, has reached v1.x stability, and has patched known CVEs (CVE-2026-33252, GHSA-q382-vc8q-7jhj).

## Recommendations

- Monitor `stripe-go` for upgrade to v82+ (current v76 is 9 major versions behind)
- Run `govulncheck ./...` periodically for transitive dependency CVEs
