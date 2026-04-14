SERVICE_DIR := $(dir $(abspath $(lastword $(MAKEFILE_LIST))))

.PHONY: docker-up-infra docker-down run build migrate test cli seed query

docker-up-infra:
	cd $(SERVICE_DIR) && docker compose up -d

docker-down:
	cd $(SERVICE_DIR) && docker compose down -v

run:
	cd $(SERVICE_DIR) && go run ./cmd/server

build:
	cd $(SERVICE_DIR) && go build -o bin/market-platform ./cmd/server

migrate:
	cd $(SERVICE_DIR) && go run ./cmd/server migrate

test:
	cd $(SERVICE_DIR) && go test ./... -v -count=1

test-integration:
	cd $(SERVICE_DIR) && go test ./tests/integration/... -v -count=1 -tags=integration

cli:
	cd $(SERVICE_DIR) && go build -o bin/iecli ./cmd/iecli

seed:
	cd $(SERVICE_DIR) && go run ./cmd/iecli seed

query:
	cd $(SERVICE_DIR) && go run ./cmd/iecli query
