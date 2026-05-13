# motifpath-core

Go monorepo containing the two backend services for the MotifPath platform.

| Service | Description | Database |
|---------|-------------|----------|
| `services/core-domain/` | Learning graph, student paths, threshold logic | PostgreSQL via ent ORM |
| `services/event-ingestion/` | Domain event collection and storage | MongoDB Atlas |

## Prerequisites

- [devbox](https://www.jetify.com/devbox) — manages Go, tools, and local dependencies
- Docker (for local Postgres and MongoDB via `make dev`)

## Getting Started

```bash
devbox shell          # enter the dev environment
make dev              # start Postgres + MongoDB via docker-compose
make generate         # generate oapi-codegen stubs from motifpath-specs
make test             # run unit tests
make test:bdd         # run Gherkin BDD tests via godog
make test:int         # run integration tests via testcontainers
```

## Repository Structure

```
services/
  core-domain/
    cmd/                      entry point and dependency wiring
    internal/
      domain/                 entities, value objects, domain errors (zero deps)
      application/            use cases and service layer
      ports/                  repository, service, and event interfaces
      adapters/http/          HTTP handlers (generated — do not edit)
      adapters/repo/          PostgreSQL repository implementations
  event-ingestion/
    cmd/
    internal/
      domain/
      application/
      ports/
      adapters/http/          (generated — do not edit)
      adapters/repo/          MongoDB repository implementations
```

## Architecture

Both services follow **Hexagonal Architecture** — dependency direction is always inward:

```
adapters → application → domain
           ports ↗
```

Business logic lives exclusively in `internal/domain/` and `internal/application/`. HTTP handlers call application services and do nothing else.

## Domain Model (core-domain)

- **Node** — a concept in the learning graph with prerequisites and a default accuracy threshold
- **StudentPath** — tracks node states (`locked` / `unlocked` / `in_progress`) per student
- **ThresholdOverride** — a teacher-set custom threshold for a specific student+node pair

Threshold rule: `ThresholdOverride` takes precedence over the Node's default threshold when one exists for the student+node pair.

## Makefile Reference

```
make generate     regenerate oapi-codegen stubs from spec
make test         run unit tests
make test:bdd     run godog BDD tests
make test:int     run integration tests (testcontainers)
make lint         run golangci-lint
make dev          start local dependencies via docker-compose
```

## Code Quality

- **golangci-lint** enforced via `.golangci.yml` — run `make lint` before committing
- **Coverage gate**: 80% on `internal/application/` packages
- Errors must be handled explicitly — no blank identifier `_` on error returns
- No `interface{}` / `any` — define explicit types
