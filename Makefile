SERVICES := services/core-domain services/event-ingestion services/aggregation-worker
SPECS_DIR := ../motifpath-specs

.PHONY: generate check-oapi-codegen generate\:ent migrate\:diff test test\:bdd test\:int lint dev db\:reset db\:full-reset

# The committed stubs are only reproducible with the oapi-codegen and
# @redocly/cli versions pinned in mise.toml: generate refuses any other
# oapi-codegen on the PATH and runs the pinned redocly through npx.
mise_pin = $(shell sed -n 's|^"$(1)" = "\(.*\)"$$|\1|p' mise.toml)
OAPI_CODEGEN_VERSION := $(call mise_pin,go:github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen)
REDOCLY_CLI_VERSION := $(call mise_pin,npm:@redocly/cli)
# The pin is deliberate, so its "update available" banner is noise.
REDOCLY := REDOCLY_SUPPRESS_UPDATE_NOTICE=true npx --yes @redocly/cli@$(REDOCLY_CLI_VERSION)

check-oapi-codegen:
	@actual=$$(oapi-codegen -version 2>/dev/null | tail -1); \
	if [ "$$actual" != "v$(OAPI_CODEGEN_VERSION)" ]; then \
		echo "oapi-codegen $${actual:-not found} on the PATH, v$(OAPI_CODEGEN_VERSION) required (mise.toml)."; \
		echo "Run: mise install && mise exec -- make generate"; \
		exit 1; \
	fi

generate: check-oapi-codegen
	@test -n "$(REDOCLY_CLI_VERSION)" || { echo "@redocly/cli pin not found in mise.toml."; exit 1; }
	@mkdir -p .bundled
	$(REDOCLY) bundle $(SPECS_DIR)/openapi/event-ingestion-service.yaml \
		-o .bundled/event-ingestion-service.yaml
	oapi-codegen -config services/event-ingestion/oapi-codegen.yaml \
		.bundled/event-ingestion-service.yaml
	$(REDOCLY) bundle $(SPECS_DIR)/openapi/core-domain-service.yaml \
		-o .bundled/core-domain-service.yaml
	@# The script types the few tri-state fields the spec alone can't express;
	@# it rewrites only this local, gitignored bundle (see its comments).
	@perl scripts/openapi-compat.pl .bundled/core-domain-service.yaml
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
	go test -v -tags integration ./services/aggregation-worker/internal/bdd/...

test\:int:
	go test -v -tags integration ./services/event-ingestion/internal/adapters/...
	go test -v -tags integration ./services/core-domain/internal/adapters/...
	go test -v -tags integration ./services/aggregation-worker/internal/adapters/...

lint:
	golangci-lint run ./services/event-ingestion/... ./services/core-domain/... ./services/aggregation-worker/...

# --remove-orphans: a container left over from a service that was removed
# from docker-compose.yml keeps running and holding its host ports, which
# blocks a replacement service published on the same port.
dev:
	docker compose up -d --remove-orphans
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
