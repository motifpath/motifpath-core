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

- [Docker](https://docs.docker.com/get-docker/) with Compose v2 (`docker compose`, not
  `docker-compose`) — runs local dependencies (Postgres, MongoDB, Redpanda, object store) and
  integration tests. On Linux, add yourself to the `docker` group
  (`sudo usermod -aG docker $USER`, then log out and back in) so it runs without `sudo`.
- [mise](https://mise.jdx.dev) — installs the toolchain pinned in `mise.toml`
- [Atlas](https://atlasgo.io) 1.3.2 — the migration CLI, same version as `services/core-domain/Dockerfile`
- `git`, `make` and `curl` — preinstalled on most Linux distributions; on macOS, run
  `xcode-select --install`
- A Clerk development instance — see [Clerk keys](#clerk-keys)

You do **not** need to install Go, Node, golangci-lint, oapi-codegen, wgo, or process-compose
yourself — mise provides all of them, at the exact versions declared in `mise.toml`.

mise does not provide these, so install them separately where they are missing:

| Tool | Needed for | Linux | macOS |
|---|---|---|---|
| GNU `timeout` | the `wait-*` gates in `process-compose.yaml` | preinstalled | GNU coreutils (installs as `gtimeout`; link it as `timeout`) |
| Bash 4+ | `scripts/voice-samples.sh` | preinstalled | the system Bash is 3.2 — install a newer one |
| ffmpeg | rendering voice samples | `apt install ffmpeg` | any package source |

## Getting Started (first time on this machine)

All MotifPath repositories must be cloned **side by side in the same parent directory** —
`make generate` and `make test:bdd` read `../motifpath-specs`, and `mise run full` starts
`../motifpath-web`. The clone commands are in the
[motifpath-specs README](../motifpath-specs/README.md#onboarding).

1. **Install Docker and the other prerequisites** above.
2. **Install mise** and activate it in your shell:
   ```bash
   curl https://mise.run | sh
   # bash:
   echo 'eval "$(~/.local/bin/mise activate bash)"' >> ~/.bashrc && source ~/.bashrc
   # zsh:
   echo 'eval "$(~/.local/bin/mise activate zsh)"' >> ~/.zshrc && source ~/.zshrc
   ```
3. **Install the pinned toolchain**, from the repo root:
   ```bash
   mise trust
   mise install
   go version   # should print the version pinned in mise.toml
   ```
   The first run takes a few minutes: the Go-based tools are compiled locally. Until both
   commands have run, mise puts nothing on your `PATH` — see [Troubleshooting](#troubleshooting).
4. **Install Atlas:**
   ```bash
   curl -sSf https://atlasgo.sh | ATLAS_VERSION=v1.3.2 sh
   ```
   Atlas removes binaries older than about six months; if this version is refused, bump it here
   and in `services/core-domain/Dockerfile` together.
5. **Create the env files** from their templates:
   ```bash
   cp .env.example                             .env
   cp services/core-domain/.env.example       services/core-domain/.env
   cp services/event-ingestion/.env.example   services/event-ingestion/.env
   cp services/aggregation-worker/.env.example services/aggregation-worker/.env
   ```
   Set `CLERK_SECRET_KEY` in each of the three service `.env` files (see [Clerk keys](#clerk-keys)).
   The defaults for everything else already match `make dev`. The root `.env` holds only
   `ADMIN_CLERK_USER_ID`, filled in at step 9.
6. **Start local dependencies:**
   ```bash
   make dev
   ```
   Starts Postgres, MongoDB, Redpanda (local Kafka), and the SeaweedFS object store via
   docker-compose and waits for their healthchecks.
7. **Set up `motifpath-web`** — follow its [README → Setup](../motifpath-web/README.md#setup)
   (`npm install` and `.env.local`).
8. **Start the full stack:**
   ```bash
   mise run full
   ```
   Open http://localhost:5173 and sign in once with Google. This creates your Clerk user and
   registers you as a student.
9. **Make yourself an admin and load the demo content.** Copy your user id (`user_…`) from the
   Clerk dashboard → Users into `ADMIN_CLERK_USER_ID` in the root `.env`, then, in a second
   terminal:
   ```bash
   make db:reset
   ```
   This wipes the local database, re-applies every migration, seeds courses, paths, lessons,
   exercises and diagrams, bootstraps your account as admin, and uploads the voice samples.
   Then run `process-compose process restart event-ingestion` and reload the browser — see
   [Troubleshooting](#troubleshooting).
10. **Check it works:** after the reload you see the admin's course and path in the web app,
    and `make test` passes.

### Clerk keys

Sign-in goes through [Clerk](https://clerk.com). Use a **development** instance with Google
sign-in enabled — ask a maintainer to invite you to the team's instance, or create your own for
solo work. Every repo must point at the **same** instance:

| Value | Where it goes | Where to find it |
|---|---|---|
| Secret key (`sk_test_…`) | `CLERK_SECRET_KEY` in each `services/*/.env` | Clerk dashboard → API keys |
| Publishable key (`pk_test_…`) | `VITE_CLERK_PUBLISHABLE_KEY` in `motifpath-web/.env.local` | Clerk dashboard → API keys |
| Your user id (`user_…`) | `ADMIN_CLERK_USER_ID` in the root `.env` | Clerk dashboard → Users, after your first sign-in |

A `pk_test_…` value in `CLERK_SECRET_KEY` lets the services start, but every authenticated call
then returns 401.

## Local Setup (day to day)

```bash
make dev        # dependency containers (Postgres, MongoDB, Redpanda, object store)
mise run full   # backend + aggregation-worker + web
```

With mise activated in your shell, the pinned tools are on your `PATH` whenever you are inside
the repo — there is no shell to enter.

## Troubleshooting

- **`go: command not found`** (or `oapi-codegen`, `wgo`, `process-compose`) — mise has not
  installed or trusted this repo's `mise.toml`. Run `mise trust && mise install` in the repo,
  then open a new terminal or `cd` out and back in. `mise ls` should list every tool with a
  version.
- **Progress stops saving after `make db:reset`** — `event-ingestion` still holds the user ids
  from before the reset. Run `process-compose process restart event-ingestion` from the repo
  and reload the browser.
- **Diagrams play no sound** — the voice samples aren't in the local object store. Install
  ffmpeg and Bash 4+, then run `scripts/voice-samples.sh` (or `make db:reset`).
- **Sign-in ends at `/welcome/error`, or every API call returns 401** — `CLERK_SECRET_KEY` is
  missing, is a `pk_test_…` key, or belongs to a different Clerk instance than the web app's
  publishable key.

## Running the services locally

The Go services run as `wgo`-reloaded processes managed by
[process-compose](https://github.com/F1bonacc1/process-compose) (installed by
mise). The dependency containers stay in Docker Compose — `make dev` starts
them, process-compose does not touch them (ADR-016).

```bash
# 1. One-time: create the .env files (Getting Started, step 5).

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