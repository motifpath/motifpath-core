#!/usr/bin/env bash
# Fails when this branch changes or deletes a migration that base already
# has. Production runs base's migrations, and Atlas never runs an applied
# migration again, so an edit to one silently never reaches it: a change to
# the schema or the reference data goes in a new migration.
#
# Usage: scripts/applied-migrations.sh [base] [migrations dir]
#   base defaults to origin/main; the comparison is from where this branch
#   left base, so a migration base gained since isn't this branch's change.
#   atlas.sum is rehashed for every new migration, so it may change.
set -euo pipefail

base="${1:-origin/main}"
dir="${2:-services/core-domain/internal/adapters/repo/ent/migrate/migrations}"

changed=$(git diff --name-status --no-renames "$base"...HEAD -- "$dir" |
  awk '($1 == "M" || $1 == "D") && $2 !~ /\/atlas\.sum$/ { print "  " $1 " " $2 }')

if [[ -n "$changed" ]]; then
  echo "These migrations are already on $base, so production may have applied them:"
  echo "$changed"
  echo "Put them back as they are on $base, and make the change in a new migration."
  exit 1
fi
