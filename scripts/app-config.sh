#!/usr/bin/env bash
# app-config.sh — reads or replaces one of the app's settings through core-domain's admin API
# (GET/PUT /admin/config/{key}). There is no settings screen: the team keeps each environment's
# values in a JSON file and sets them with this script.
#
# Usage:
#   scripts/app-config.sh list
#   scripts/app-config.sh get starting_points
#   scripts/app-config.sh set starting_points path/to/starting-points.json
#
# Environment:
#   ADMIN_TOKEN       an admin's Clerk session token (required). In the SPA, signed in as an admin,
#                     run `await Clerk.session.getToken()` in the browser console; it lasts about a
#                     minute, so copy it right before running this.
#   CORE_DOMAIN_URL   core-domain's base URL (default http://localhost:8080).
#
# A value the key refuses is answered 400 with every problem listed; nothing is stored.
set -euo pipefail

base="${CORE_DOMAIN_URL:-http://localhost:8080}"
: "${ADMIN_TOKEN:?set ADMIN_TOKEN to a Clerk session token of an admin, see the header of this script}"

usage() {
	sed -n '5,9p' "$0" | sed 's/^# \{0,1\}//'
	exit 2
}

call() {
	curl --silent --show-error --fail-with-body \
		-H "Authorization: Bearer ${ADMIN_TOKEN}" \
		-H "Accept: application/json" \
		"$@"
	echo
}

case "${1:-}" in
list)
	call "${base}/admin/config"
	;;
get)
	[[ $# -eq 2 ]] || usage
	call "${base}/admin/config/$2"
	;;
set)
	[[ $# -eq 3 ]] || usage
	[[ -f "$3" ]] || { echo "no such file: $3" >&2; exit 1; }
	call -X PUT -H "Content-Type: application/json" --data-binary "@$3" "${base}/admin/config/$2"
	;;
*)
	usage
	;;
esac
