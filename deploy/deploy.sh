#!/usr/bin/env bash
# deploy.sh — runs ON THE VM, as root, from /opt/motifpath/<environment>/,
# where the environment's deploy document unpacked this directory from the
# commit being deployed. Replaces one environment's containers with the given
# image tag and leaves the other environment alone.
#
#   deploy.sh <staging|production> <image-tag>
#
# Rolling back is the same command with the previous tag; the last two tags
# are kept in /etc/motifpath/<environment>.deployed.
#
# Steps:
#   1. render the environment's secrets from SSM Parameter Store
#   2. log in to ECR and pull the three service images for the tag
#   3. publish the voice samples the core-domain image carries to the
#      environment's media bucket
#   4. recreate the environment's containers (migrations run as core-domain
#      starts)
#   5. bring up the edge proxy, or reload it if its Caddyfile changed
#   6. wait until every service answers /healthz and /readyz, and every voice
#      sample answers at the public media URL
set -euo pipefail

usage() { echo "usage: $0 <staging|production> <image-tag>" >&2; exit 2; }

[[ $# -eq 2 ]] || usage
env=$1
tag=$2
[[ $env == staging || $env == production ]] || usage
# Images are tagged with the full commit SHA they were built from.
[[ $tag =~ ^[0-9a-f]{40}$ ]] || { echo "image tag must be a full 40-char commit SHA, got '$tag'" >&2; exit 2; }

bundle=$(cd "$(dirname "$0")" && pwd)
state_dir=/etc/motifpath
edge_dir=/opt/motifpath/edge
caddy_image=$(sed -n 's/^ *image: *\(caddy:[^ ]*\)$/\1/p' "$bundle/edge/compose.yaml")

# One deploy at a time on this host: both environments share the edge proxy
# and the Docker daemon.
mkdir -p "$state_dir"
chmod 700 "$state_dir"
exec 9>/run/lock/motifpath-deploy.lock
flock --wait 900 9 || { echo "another deploy still holds the lock" >&2; exit 1; }

log() { printf '[%s] %s\n' "$(date -u +%H:%M:%S)" "$*"; }

# render_params <ssm-path> <file> — writes every parameter directly under the
# path as NAME='value', in the env-file syntax Compose reads. Single quotes
# keep values literal, so connection strings with &, $ or # survive intact;
# a value that itself contains a single quote or a newline is refused rather
# than mangled.
render_params() {
	local path=$1 out=$2 json
	json=$(aws ssm get-parameters-by-path --path "$path" --with-decryption \
		--region "$AWS_REGION" --output json)
	umask 077
	python3 -c '
import json, re, sys
params = json.load(sys.stdin)["Parameters"]
if not params:
    sys.exit("no parameters under " + sys.argv[1])
for p in sorted(params, key=lambda p: p["Name"]):
    name = p["Name"].rsplit("/", 1)[1]
    value = p["Value"]
    if not re.fullmatch(r"[A-Z][A-Z0-9_]*", name):
        sys.exit("parameter %s: name is not an env var name" % p["Name"])
    if "\x27" in value or "\n" in value:
        sys.exit("parameter %s: value contains a single quote or newline" % p["Name"])
    print("%s=\x27%s\x27" % (name, value))
' "$path" <<<"$json" >"$out.tmp"
	mv "$out.tmp" "$out"
}

env_value() { sed -n "s/^$1=//p" "$2" | tr -d "'"; }

# publish_voice_samples — copies the voice samples out of the core-domain
# image being deployed and syncs them to this environment's media bucket with
# its media key. Nothing is deleted: a rollback keeps newer files, and a URL
# that once played a sound keeps playing it. A sample never changes at its
# path, so comparing sizes is enough (the copy's timestamps are always new),
# and a redeploy uploads nothing. The copy stays in $samples for
# check_voice_samples.
publish_voice_samples() {
	local container
	rm -rf "$samples"
	mkdir -p "$samples"
	container=$(docker create "$registry/motifpath/core-domain:$tag")
	docker cp "$container:/app/voice-samples/." "$samples"
	docker rm "$container" >/dev/null
	AWS_ACCESS_KEY_ID=$(env_value MEDIA_AWS_ACCESS_KEY_ID "$secrets") \
		AWS_SECRET_ACCESS_KEY=$(env_value MEDIA_AWS_SECRET_ACCESS_KEY "$secrets") \
		aws s3 sync "$samples" "s3://$(env_value MEDIA_S3_BUCKET "$secrets")/audio/voices" \
		--region "$AWS_REGION" --size-only --exclude '*' --include '*.mp3' \
		--content-type audio/mpeg --cache-control 'public, max-age=31536000, immutable' \
		--only-show-errors
}

# check_voice_samples — every published sample must answer 200 audio/mpeg at
# the environment's public media URL, where players download it from.
check_voice_samples() {
	local base file rel answer failed=0 checked=0
	base=$(env_value MEDIA_PUBLIC_BASE_URL "$bundle/env/$env.env")
	for file in "$samples"/*/*.mp3; do
		[[ -f $file ]] || continue # the unmatched pattern itself, when there are none
		rel=${file#"$samples"/}
		answer=$(curl -fsSI --max-time 10 --retry 5 --retry-delay 3 --retry-all-errors \
			-o /dev/null -w '%{http_code} %{content_type}' "$base/audio/voices/$rel" || true)
		checked=$((checked + 1))
		if [[ $answer != "200 audio/mpeg" ]]; then
			echo "$env voice sample $rel: got '${answer:-no answer}' from $base" >&2
			failed=$((failed + 1))
		fi
	done
	((checked > 0)) || { echo "$env: the core-domain image carries no voice samples" >&2; return 1; }
	((failed == 0)) || return 1
	log "$env voice samples: $checked answer 200 audio/mpeg"
}

AWS_REGION=$(env_value AWS_REGION "$bundle/env/$env.env")
export AWS_REGION

log "rendering $env secrets"
secrets=$state_dir/$env.secrets.env
samples=$state_dir/$env.voice-samples
render_params "/motifpath/$env/" "$secrets"
render_params "/motifpath/edge/" "$state_dir/edge.secrets.env"

registry=$(env_value ECR_REGISTRY "$secrets")
[[ -n $registry ]] || { echo "ECR_REGISTRY missing under /motifpath/$env/" >&2; exit 1; }
log "logging in to $registry"
aws ecr get-login-password --region "$AWS_REGION" |
	docker login --username AWS --password-stdin "$registry" >/dev/null

# The shell's IMAGE_TAG wins over anything in the env files.
export IMAGE_TAG=$tag
compose=(docker compose -p "$env"
	--env-file "$bundle/env/$env.env" --env-file "$secrets"
	-f "$bundle/compose.yaml")

log "pulling $env images at $tag"
"${compose[@]}" pull --quiet

# Published before the containers are recreated: core-domain applies the
# release's migrations as it starts, and any voice they list must already play.
log "publishing $env voice samples"
publish_voice_samples

log "recreating $env containers"
"${compose[@]}" up --detach --remove-orphans

# The edge config comes from whichever bundle deploys last, so it is checked
# before it replaces the live copy: a broken Caddyfile would otherwise take
# both environments offline at the next restart.
log "updating edge proxy"
docker run --rm -e ACME_EMAIL=validate@example.com \
	-v "$bundle/edge:/etc/caddy:ro" "$caddy_image" \
	caddy validate --config /etc/caddy/Caddyfile --adapter caddyfile >/dev/null
mkdir -p "$edge_dir"
cp "$bundle/edge/compose.yaml" "$bundle/edge/Caddyfile" "$edge_dir/"
edge=(docker compose -p edge --env-file "$state_dir/edge.secrets.env"
	-f "$edge_dir/compose.yaml")
"${edge[@]}" up --detach
# Forced even when the Caddyfile is unchanged: a hostname whose DNS record
# appeared after Caddy started has had its certificate attempts fail and
# backed off, and a reload is what makes Caddy try again right away.
"${edge[@]}" exec -T caddy caddy reload --config /etc/caddy/Caddyfile --adapter caddyfile --force

# core-domain applies pending migrations before it listens, so the first
# probes can see a refused connection for a while.
probe() {
	curl -fsS --max-time 5 --retry 60 --retry-delay 3 --retry-all-errors "$1" >/dev/null
}
for name in CORE_DOMAIN EVENT_INGESTION AGGREGATION_WORKER ENGAGEMENT_MAILER; do
	port=$(env_value "${name}_HOST_PORT" "$bundle/env/$env.env")
	for path in healthz readyz; do
		if ! probe "http://127.0.0.1:$port/$path"; then
			echo "$env ${name,,}: /$path did not answer on :$port" >&2
			"${compose[@]}" ps >&2
			"${compose[@]}" logs --tail 80 >&2
			exit 1
		fi
	done
	log "$env ${name,,}: /healthz + /readyz OK"
done

check_voice_samples
previous=$(head -n1 "$state_dir/$env.deployed" 2>/dev/null || true)
if [[ $previous != "$tag" ]]; then
	printf '%s\n%s\n' "$tag" "$previous" >"$state_dir/$env.deployed"
fi
log "$env now runs $tag${previous:+ (previous: $previous)}"
