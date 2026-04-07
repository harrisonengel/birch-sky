# Market Platform — Test Plan

## Test Infrastructure

All integration tests use `testcontainers-go` to spin up real PostgreSQL 16, OpenSearch 3.0, and MinIO containers. No mocks for storage — only the Stripe payment processor is mocked.

Build tag: `//go:build integration`

Run: `go test ./tests/integration/... -v -count=1 -tags=integration`

## Test Matrix

### Sellers (`listing_test.go`)
| # | Test Case | Expected |
|---|-----------|----------|
| 1 | Create seller | 201, valid UUID |
| 2 | Duplicate email | 409 conflict |
| 3 | Get seller by ID | 200 |

### Listings (`listing_test.go`)
| # | Test Case | Expected |
|---|-----------|----------|
| 4 | Create listing | 201, valid UUID |
| 5 | Missing required fields | 400 |
| 6 | Get listing by ID | 200, full metadata |
| 7 | Get nonexistent listing | 404 |
| 8 | List listings (paginated) | 200, correct pagination |
| 9 | List filtered by category | only matching |
| 10 | Update metadata | 200, fields changed |
| 11 | Soft-delete | subsequent GET → 404 |
| 12 | Upload file | 200, data_ref populated |
| 13 | Negative price | 400 |
| 14 | Empty title | 400 |

### Search (`search_test.go`)
| # | Test Case | Expected |
|---|-----------|----------|
| 15 | Empty index search | 0 results |
| 16 | Search by exact title | top result matches |
| 17 | Category filter | only matching |
| 18 | Max price filter | no results above |
| 19 | Mode=text | results returned |
| 20 | Mode=vector | results returned |
| 21 | Mode=hybrid | RRF-fused results |
| 22 | Empty query | 400 |
| 23 | Deleted listings excluded | not in results |
| 24 | CSV content searchable | column names match |

### Purchases (`purchase_test.go`)
| # | Test Case | Expected |
|---|-----------|----------|
| 25 | Initiate purchase | 201, client_secret + txn_id |
| 26 | Confirm purchase | 200, ownership recorded |
| 27 | Get purchase status | completed |
| 28 | List ownership | buyer's listings |
| 29 | Download owned data | presigned URL |
| 30 | Download unowned data | 403 |
| 31 | Purchase nonexistent listing | 404 |
| 32 | Already-owned listing | already_owned: true |

### Buy Orders (`buyorder_test.go`)
| # | Test Case | Expected |
|---|-----------|----------|
| 33 | Create buy order | 201 |
| 34 | List buy orders | paginated |
| 35 | Get buy order | 200 |
| 36 | Fill with listing | status → filled |
| 37 | Fill already-filled | 409 |
| 38 | Cancel buy order | 204 |
| 39 | Fill cancelled | 409 |
| 40 | Max price = 0 | 400 |
| 41 | Filter by buyer_id | correct results |
