SERVICES := services/core-domain services/event-ingestion services/aggregation-worker
SPECS_DIR := ../motifpath-specs

.PHONY: generate generate\:ent migrate\:diff test test\:bdd test\:int lint dev db\:reset db\:full-reset

generate:
	@mkdir -p .bundled
	npx --yes @redocly/cli bundle $(SPECS_DIR)/openapi/event-ingestion-service.yaml \
		-o .bundled/event-ingestion-service.yaml
	oapi-codegen -config services/event-ingestion/oapi-codegen.yaml \
		.bundled/event-ingestion-service.yaml
	npx --yes @redocly/cli bundle $(SPECS_DIR)/openapi/core-domain-service.yaml \
		-o .bundled/core-domain-service.yaml
	@# oapi-codegen 2.4 doesn't parse OpenAPI 3.1's "type: [<type>, null]" nullable
	@# shorthand (any single type), which redocly's bundler
	@# re-serializes as a two-item YAML list (only this local, gitignored bundle is
	@# touched — the spec itself, in motifpath-specs, is untouched and keeps
	@# authoring in 3.1 style).
	@perl -0pi -e 's/^([ \t]*)type:\n[ \t]*- (string|integer|number|boolean|array|object)\n[ \t]*- .null.\n/$$1type: $$2\n$$1nullable: true\n/mg' .bundled/core-domain-service.yaml
	oapi-codegen -config services/core-domain/oapi-codegen.yaml \
		.bundled/core-domain-service.yaml

migrate\:diff:
	@if [ -z "$(name)" ]; then echo "Usage: make migrate:diff name=<description>"; exit 1; fi
	cd services/core-domain && go run ./cmd/entmigrate $(name)

# Regenerates the ent ORM client (internal/adapters/repo/ent/*) from the
# schema files under ent/schema/ — a separate step from `generate` above,
# since that one only covers the OpenAPI-derived HTTP layer.
generate\:ent:
	cd services/core-domain && go generate ./...

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

# Wipes the local dev Postgres, re-migrates from scratch, and repopulates
# with cmd/seed-full's full combination matrix. Hard-refuses to run against
# anything but localhost — see scripts/db-reset.sh. NEVER run in production.
db\:reset:
	./scripts/db-reset.sh

# One-shot catch-up after editing the OpenAPI spec and/or an ent schema file
# (internal/adapters/repo/ent/schema/*): regenerates the HTTP layer and the
# ent client, verifies the result still builds and passes, generates the
# matching Atlas migration, then wipes and reseeds the local dev database
# with it applied. Each step aborts the chain on failure, same as running
# them by hand in order. Requires Docker running and a migration name:
#   make db:full-reset name=<description>
db\:full-reset:
	@if [ -z "$(name)" ]; then echo "Usage: make db:full-reset name=<description>"; exit 1; fi
	docker compose up -d
	$(MAKE) generate
	$(MAKE) generate:ent
	cd services/core-domain && go build ./... && go test ./...
	$(MAKE) migrate:diff name=$(name)
	$(MAKE) db:reset
