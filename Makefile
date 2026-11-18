# Hugimethara developer tasks. Run `make help`.
COMPOSE := docker compose -f deploy/compose/docker-compose.yml

.PHONY: help generate check-generated lint test test-integration web-check up down logs
help: ## Show targets
	@grep -E '^[a-z-]+:.*##' $(MAKEFILE_LIST) | awk -F':.*## ' '{printf "  %-18s %s\n", $$1, $$2}'

generate: ## Regenerate Go (oapi-codegen, sqlc) and TS (openapi-typescript) code
	cd backend && go generate ./... && go tool sqlc generate
	cd web && pnpm generate:api

check-generated: generate ## Fail if generated code is out of date
	git diff --exit-code -- backend/internal/api/apigen backend/internal/event/eventdb web/src/lib/api/schema.d.ts

lint: ## Lint backend and web
	cd backend && golangci-lint run ./...
	cd web && pnpm lint

test: ## Backend unit tests
	cd backend && go test -race ./...

test-integration: ## Backend integration tests (needs Docker)
	cd backend && go test -race -tags integration -count=1 ./...

web-check: ## Web typecheck + build
	cd web && pnpm typecheck && pnpm build

up: ## Start the full local stack
	$(COMPOSE) up --build -d

down: ## Stop the local stack (keeps volumes)
	$(COMPOSE) down

logs: ## Tail stack logs
	$(COMPOSE) logs -f api worker
