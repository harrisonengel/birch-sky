SERVICE_DIR := $(dir $(abspath $(lastword $(MAKEFILE_LIST))))

.PHONY: docker-up-infra docker-down run build migrate seed test

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

seed:
	cd $(SERVICE_DIR) && go run ./cmd/seed

test:
	cd $(SERVICE_DIR) && go test ./... -v -count=1

test-integration:
	cd $(SERVICE_DIR) && go test ./tests/integration/... -v -count=1 -tags=integration
