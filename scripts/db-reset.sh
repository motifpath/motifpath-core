#!/usr/bin/env bash
# db-reset.sh — wipes the LOCAL dev Postgres, re-applies every Atlas
# migration from scratch, and repopulates it with `seed-full`'s broad
# combination matrix (every CourseStatus, every CourseEnrollmentStatus,
# standalone paths current and archived, every ExerciseType). Also clears
# the local dev Mongo's completion aggregates collection, since seed-full
# writes fresh ones.
#
# HARD, NON-OVERRIDABLE PRODUCTION GUARD: refuses to run unless both
# DATABASE_URL and MONGO_URI resolve to localhost/127.0.0.1. There is no
# flag to bypass this — if you need to point this at something else, you
# are holding the wrong tool. This script only exists for local dev/CI
# throwaway databases; it must never run against a real environment.
#
# Usage:
#   make db:reset                                    # local defaults
#   ADMIN_CLERK_USER_ID=user_xxx make db:reset        # also bootstrap your own admin
#
# A repo-root .env (gitignored, see .env.example) is loaded automatically if
# present, so ADMIN_CLERK_USER_ID only has to be set once instead of typed on
# every invocation. A variable already set in the shell environment wins over
# the .env value.
set -euo pipefail

if [ -f .env ]; then
  while IFS='=' read -r key value; do
    [[ "$key" =~ ^[A-Za-z_][A-Za-z0-9_]*$ ]] || continue
    if [ -z "${!key:-}" ]; then
      export "$key=$value"
    fi
  done < <(grep -v '^\s*#' .env | grep -v '^\s*$')
fi

DATABASE_URL="${DATABASE_URL:-postgres://motifpath:motifpath@localhost:5432/core_domain?sslmode=disable}"
MONGO_URI="${MONGO_URI:-mongodb://motifpath:motifpath@localhost:27017/?authSource=admin}"
MONGO_DATABASE="${MONGO_DATABASE:-motifpath_events}"

require_local_host() {
  local url="$1" label="$2" host
  # Strips scheme://[user[:pass]@] and everything from the next / ? or :
  # (port) onward, leaving just the host.
  host=$(echo "$url" | sed -E 's#^[a-zA-Z0-9+]+://([^:@]+(:[^@]*)?@)?##; s#[/:?].*$##')
  case "$host" in
    localhost|127.0.0.1) ;;
    *)
      echo "REFUSED: $label resolves to host '$host', not localhost/127.0.0.1." >&2
      echo "db-reset.sh only ever runs against a local dev database — there is no override." >&2
      exit 1
      ;;
  esac
}

require_local_host "$DATABASE_URL" "DATABASE_URL"
require_local_host "$MONGO_URI" "MONGO_URI"

MIGRATIONS_DIR="file://services/core-domain/internal/adapters/repo/ent/migrate/migrations"

echo "==> Resetting local Postgres container"
docker compose stop postgres >/dev/null 2>&1 || true
docker compose rm -f postgres >/dev/null 2>&1 || true
docker volume rm "$(basename "$(pwd)")_postgres_data" >/dev/null 2>&1 || true
docker compose up -d postgres >/dev/null

echo "==> Waiting for Postgres to accept connections"
for _ in $(seq 1 60); do
  if docker compose exec -T postgres pg_isready -U motifpath >/dev/null 2>&1; then
    break
  fi
  sleep 1
done

echo "==> Clearing local Mongo completion aggregates"
docker compose exec -T mongodb mongosh --quiet -u motifpath -p motifpath --authenticationDatabase admin \
  --eval "db.getSiblingDB('${MONGO_DATABASE}').aggregates.deleteMany({})" >/dev/null 2>&1 || true

echo "==> Applying migrations from scratch"
atlas migrate apply --dir "$MIGRATIONS_DIR" --url "$DATABASE_URL"

echo "==> Running seed-full"
( cd services/core-domain && \
  DATABASE_URL="$DATABASE_URL" MONGO_URI="$MONGO_URI" MONGO_DATABASE="$MONGO_DATABASE" \
  go run ./cmd/seed-full )

echo "==> db-reset done"
