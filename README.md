# motifpath-core

Go monorepo for MotifPath backend services.

## Services

| Service | Path | Database | Purpose |
|---|---|---|---|
| core-domain | `services/core-domain/` | PostgreSQL (ent ORM) | Learning graph, student paths, threshold logic |
| event-ingestion | `services/event-ingestion/` | MongoDB Atlas | Domain event collection and storage |
| aggregation-worker | `services/aggregation-worker/` | MongoDB Atlas | Kafka consumer deriving node-completion state (ADR-011) |

All three services follow hexagonal architecture. Specs live in
[motifpath-specs](../motifpath-specs) — check there before implementing any feature.

## Onboarding

First-time machine setup is handled from `motifpath-specs` — see its
[README](../motifpath-specs/README.md#onboarding) for the full setup steps
(global CLAUDE.md + Claude skill installation).

## Branching Model

```
main  (protected — production releases only)
dev   (protected — integration branch, target for all feature PRs)
```

Branch naming — task code is mandatory:

```
feat/MTP-001/short-description    ← branches from dev
fix/BUG-042/short-description     ← branches from dev
hotfix/BUG-099/short-description  ← branches from main (critical production fixes only)
```

After any merge to `main`, the `sync-main-to-dev` workflow opens a PR
from `main` to `dev` automatically. Review and merge it promptly.

## Prerequisites

- [Docker](https://docs.docker.com/get-docker/) — runs local dependencies (Postgres, MongoDB, Redpanda, object store) and integration tests
- [mise](https://mise.jdx.dev) — installs the toolchain pinned in `mise.toml`
- [Atlas](https://atlasgo.io) 1.3.2 — the migration CLI, same version as `services/core-domain/Dockerfile`

You do **not** need to install Go, Node, golangci-lint, oapi-codegen, wgo, or process-compose
yourself — mise provides all of them, at the exact versions declared in `mise.toml`.

mise does not provide these, so install them separately where they are missing:

| Tool | Needed for | Linux | macOS |
|---|---|---|---|
| GNU `timeout` | the `wait-*` gates in `process-compose.yaml` | preinstalled | GNU coreutils (installs as `gtimeout`; link it as `timeout`) |
| Bash 4+ | `scripts/voice-samples.sh` | preinstalled | the system Bash is 3.2 — install a newer one |
| ffmpeg | rendering voice samples | `apt install ffmpeg` | any package source |

## Getting Started (first time on this repo)

1. **Install Docker** if you don't already have it (link above).
2. **Install mise** and activate it in your shell (zsh shown; see the mise docs for others):
   ```bash
   curl https://mise.run | sh
   echo 'eval "$(~/.local/bin/mise activate zsh)"' >> ~/.zshrc && source ~/.zshrc
   ```
3. **Install the pinned toolchain**, from the repo root:
   ```bash
   mise trust
   mise install
   ```
   The first run takes a few minutes: the Go-based tools are compiled locally. `mise ls` then
   lists each tool with the version from `mise.toml`.
4. **Install Atlas:**
   ```bash
   curl -sSf https://atlasgo.sh | ATLAS_VERSION=v1.3.2 sh
   ```
   Atlas removes binaries older than about six months; if this version is refused, bump it here
   and in `services/core-domain/Dockerfile` together.
5. **Start local dependencies:**
   ```bash
   make dev
   ```
   Starts Postgres, MongoDB, Redpanda (local Kafka), and the SeaweedFS object store via
   docker-compose and waits for their healthchecks.
6. **Sanity-check the toolchain:**
   ```bash
   go work sync
   ```
   Should complete with no output or errors.

You're ready to build at that point.

## Local Setup (day to day)

```bash
# Start local dependencies (Postgres, MongoDB, Redpanda, object store)
make dev
```

With mise activated in your shell, the pinned tools are on your `PATH` whenever you are inside
the repo — there is no shell to enter.

## Running the services locally

The Go services run as `wgo`-reloaded processes managed by
[process-compose](https://github.com/F1bonacc1/process-compose) (installed by
mise). The dependency containers stay in Docker Compose — `make dev` starts
them, process-compose does not touch them (ADR-016).

```bash
# 1. One-time: copy the env templates and set a real Clerk secret key in each
#    (Clerk dashboard → API keys — the same instance the SPA uses).
cp services/core-domain/.env.example       services/core-domain/.env
cp services/event-ingestion/.env.example   services/event-ingestion/.env
cp services/aggregation-worker/.env.example services/aggregation-worker/.env

# 2. Start the dependency containers (Postgres, MongoDB, Redpanda, object store).
make dev

# 3. Backend inner loop — core-domain (:8080) + event-ingestion (:8081),
#    each rebuilding on save. core-domain applies Atlas migrations on startup.
mise run services
```

`mise run services` is `process-compose up`. `mise.toml` sets `PC_PORT_NUM=8099` because
process-compose's own API otherwise listens on 8080, the same port as `core-domain`, and the
readiness probe then reaches the wrong server. If you call `process-compose` outside mise, pass
`-p 8099`.

`.env` files are gitignored. Each service reads its own `services/<name>/.env`.

### Full stack (adds aggregation-worker + the web SPA)

`aggregation-worker` (:8082, health probes only) and `web` are defined but not
started by the bare command — name them to bring them up:

```bash
mise run full
```

`web` runs `npm run dev` in `../motifpath-web`, so that repo must be checked out
as a sibling with dependencies installed (`cd ../motifpath-web && npm install`).

Every service exposes `GET /healthz` (liveness) and `GET /readyz` (readiness —
dependency reachability); process-compose gates start-up on `/readyz`.

### Raw `go run` (fallback, no process-compose)

```bash
cd services/core-domain
set -a && . ./.env && set +a
go run ./cmd
```

### Browser (CORS) access

Both services send CORS headers for the origins in `CORS_ALLOWED_ORIGINS`
(comma-separated). It defaults to `http://localhost:5173` — the Vite dev server —
so `motifpath-web` works against a local build with no extra configuration.
Deployed environments set this explicitly.

### Voice samples (diagram playback)

`GET /voices` lists each voice's samples at
`{MEDIA_PUBLIC_BASE_URL}/audio/voices/{voice_id}/{midi}.mp3`. The voices and
their pitches come from the migrations; the audio files don't, so a diagram
stays silent wherever they haven't been uploaded.

- **Local:** `make db:reset` runs `scripts/voice-samples.sh`, which renders the
  samples into `.voice-samples/` (gitignored) and uploads them to the local object store.
  Rendering needs ffmpeg and Bash 4+, which mise doesn't provide (see Prerequisites).
  Later runs upload from the cache without ffmpeg.
- **Deployed environments:** before (or with) the first deploy that includes
  a new voice or new pitches, render locally as above and copy the files to
  the environment's media bucket under the same path, with long-lived caching
  since a sample never changes:
  `aws s3 sync .voice-samples/ s3://<media-bucket>/audio/voices/ --exclude '.originals/*' --cache-control 'public, max-age=31536000, immutable'`

## Commands

```bash
# Regenerate oapi-codegen stubs from spec (run after spec changes)
make generate

# Build the three service images and smoke them against throwaway deps
# (mirrors the image-smoke CI job — catches image-only failures go run hides)
docker compose -f compose.images.yaml up --build --wait --wait-timeout 120
docker compose -f compose.images.yaml down -v

# Run service-layer unit tests with coverage report
make test

# Run BDD tests via godog (requires feature files from motifpath-specs)
make test:bdd

# Run integration tests via testcontainers
make test:int

# Run all tests
make test:all

# Run linter
make lint

# Run linter + all tests (same as CI)
make ci
```

## Architecture

Each service follows hexagonal architecture:

```
internal/
  domain/           → entities, value objects, domain errors (zero dependencies)
  application/      → use cases / service layer (tested here only)
  ports/            → repository and service interfaces
  adapters/
    http/           → HTTP handlers (generated by oapi-codegen — do not edit)
    repo/           → repository implementations
cmd/                → entry point and dependency wiring
```

## Testing Strategy

- **Unit tests** (`make test`): service layer only, table-driven, testify
- **BDD tests** (`make test:bdd`): Gherkin scenarios via godog
- **Integration tests** (`make test:int`): real Postgres/MongoDB via testcontainers
- **Coverage gate**: 80% on `internal/application/` — CI fails below this

## Domain Model

Core concepts:
- **Node** — a concept in the learning graph with prerequisites and an accuracy threshold
- **StudentPath** — tracks node states (locked / unlocked / in_progress) per student
- **ThresholdOverride** — teacher-set custom threshold for a specific student on a specific node

Threshold rule: if a ThresholdOverride exists for a student+node pair,
it takes precedence over the Node's default threshold.

## Related Repositories

| Repository | Purpose |
|---|---|
| [motifpath-specs](../motifpath-specs) | All specs — check here before implementing |
| [motifpath-web](../motifpath-web) | Vue 3 frontend |
| [motifpath-infra](../motifpath-infra) | Terraform infrastructure |