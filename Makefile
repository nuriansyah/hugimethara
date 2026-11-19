# Hugimethara developer tasks. Run `make help`.
COMPOSE := docker compose -f deploy/compose/docker-compose.yml

.PHONY: help generate check-generated lint test test-integration web-check up down logs
help: ## Show targets
	@grep -E '^[a-z-]+:.*##' $(MAKEFILE_LIST) | awk -F':.*## ' '{printf "  %-18s %s\n", $$1, $$2}'

generate: ## Regenerate Go (oapi-codegen) and TS (openapi-typescript) API code
	cd backend && go generate ./...
	cd web && pnpm generate:api

check-generated: generate ## Fail if generated code is out of date
	git diff --exit-code -- backend/internal/api/apigen web/src/lib/api/schema.d.ts

lint: ## Lint backend and web
	cd backend && golangci-lint run ./...
	cd web && pnpm lint

test: ## Backend unit tests
	cd backend && go test -race ./...

# Integration tests need a disposable Postgres (superuser URL) and Redis:
#   HUGI_TEST_POSTGRES_URL=postgres://hugi_owner@127.0.0.1:5432/postgres?sslmode=disable
#   HUGI_TEST_REDIS_URL=redis://127.0.0.1:6379/0
# Each test creates and drops its own database. NATS runs in-process.
test-integration: ## Backend integration tests (see variables above)
	cd backend && go test -race -tags integration -count=1 ./...

web-check: ## Web typecheck + build
	cd web && pnpm typecheck && pnpm build

up: ## Start the full local stack
	$(COMPOSE) up --build -d

down: ## Stop the local stack (keeps volumes)
	$(COMPOSE) down

logs: ## Tail stack logs
	$(COMPOSE) logs -f api worker
