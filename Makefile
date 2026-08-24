SERVICES := services/core-domain services/event-ingestion
SPECS_DIR := ../motifpath-specs

.PHONY: generate migrate\:diff test test\:bdd test\:int lint dev

generate:
	oapi-codegen -config services/event-ingestion/oapi-codegen.yaml \
		$(SPECS_DIR)/openapi/event-ingestion-service.yaml
	oapi-codegen -config services/core-domain/oapi-codegen.yaml \
		$(SPECS_DIR)/openapi/core-domain-service.yaml

migrate\:diff:
	@if [ -z "$(name)" ]; then echo "Usage: make migrate:diff name=<description>"; exit 1; fi
	atlas migrate diff $(name) \
		--dir "file://services/core-domain/internal/adapters/repo/ent/migrate/migrations" \
		--to "ent://services/core-domain/internal/adapters/repo/ent/schema" \
		--dev-url "docker://postgres/16/dev?search_path=public"

test:
	go test ./services/event-ingestion/... ./services/core-domain/...

test\:bdd:
	go test -v -tags integration ./services/event-ingestion/internal/bdd/...
	go test -v -tags integration ./services/core-domain/internal/bdd/...

test\:int:
	go test -v -tags integration ./services/event-ingestion/internal/adapters/...
	go test -v -tags integration ./services/core-domain/internal/adapters/...

lint:
	golangci-lint run ./services/event-ingestion/... ./services/core-domain/...

dev:
	docker compose up -d
	@echo "Waiting for services to be healthy..."
	@docker compose ps
