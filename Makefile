SERVICES := services/core-domain services/event-ingestion services/aggregation-worker
SPECS_DIR := ../motifpath-specs

.PHONY: generate migrate\:diff test test\:bdd test\:int lint dev

generate:
	@mkdir -p .bundled
	npx --yes @redocly/cli bundle $(SPECS_DIR)/openapi/event-ingestion-service.yaml \
		-o .bundled/event-ingestion-service.yaml
	oapi-codegen -config services/event-ingestion/oapi-codegen.yaml \
		.bundled/event-ingestion-service.yaml
	npx --yes @redocly/cli bundle $(SPECS_DIR)/openapi/core-domain-service.yaml \
		-o .bundled/core-domain-service.yaml
	oapi-codegen -config services/core-domain/oapi-codegen.yaml \
		.bundled/core-domain-service.yaml

migrate\:diff:
	@if [ -z "$(name)" ]; then echo "Usage: make migrate:diff name=<description>"; exit 1; fi
	atlas migrate diff $(name) \
		--dir "file://services/core-domain/internal/adapters/repo/ent/migrate/migrations" \
		--to "ent://services/core-domain/internal/adapters/repo/ent/schema" \
		--dev-url "docker://postgres/16/dev?search_path=public"

test:
	go test ./services/event-ingestion/... ./services/core-domain/... ./services/aggregation-worker/...

test\:bdd:
	go test -v -tags integration ./services/event-ingestion/internal/bdd/...
	go test -v -tags integration ./services/core-domain/internal/bdd/...

test\:int:
	go test -v -tags integration ./services/event-ingestion/internal/adapters/...
	go test -v -tags integration ./services/core-domain/internal/adapters/...
	go test -v -tags integration ./services/aggregation-worker/internal/adapters/...

lint:
	golangci-lint run ./services/event-ingestion/... ./services/core-domain/... ./services/aggregation-worker/...

dev:
	docker compose up -d
	@echo "Waiting for services to be healthy..."
	@docker compose ps
