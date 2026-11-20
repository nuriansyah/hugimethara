# Hugimethara developer tasks. Run `make help`.
COMPOSE := docker compose -f deployments/compose/docker-compose.yml

.PHONY: help generate check-generated lint test test-integration web-check up down logs
help: ## Show targets
	@grep -E '^[a-z-]+:.*##' $(MAKEFILE_LIST) | awk -F':.*## ' '{printf "  %-18s %s\n", $$1, $$2}'

generate: ## Regenerate Go (oapi-codegen) and TS (openapi-typescript) API code
	go generate ./...
	pnpm --filter hugimethara-web generate:api

check-generated: generate ## Fail if generated code is out of date
	git diff --exit-code -- internal/transport/http/apigen apps/web/src/lib/api/schema.d.ts

lint: ## Lint backend and web
	golangci-lint run ./...
	pnpm --filter hugimethara-web lint

test: ## Backend unit tests
	go test -race ./...

# Integration tests need a disposable Postgres (superuser URL) and Redis:
#   HUGI_TEST_POSTGRES_URL=postgres://hugi_owner@127.0.0.1:5432/postgres?sslmode=disable
#   HUGI_TEST_REDIS_URL=redis://127.0.0.1:6379/0
# Each test creates and drops its own database. NATS runs in-process.
test-integration: ## Backend integration tests (see variables above)
	go test -race -tags integration -count=1 ./...

web-check: ## Web typecheck + build
	pnpm --filter hugimethara-web typecheck
	pnpm --filter hugimethara-web build

up: ## Start the full local stack
	$(COMPOSE) up --build -d

down: ## Stop the local stack (keeps volumes)
	$(COMPOSE) down

logs: ## Tail stack logs
	$(COMPOSE) logs -f api worker
