#!/usr/bin/env bash
# voice-samples.sh — renders every voice's samples and uploads them to the
# LOCAL dev object store, where the voice list says players download them from
# (audio/voices/{voice_id}/{midi}.mp3 under the content-media bucket).
#
# Which pitches a voice has is read from the local database's voices table,
# which the migrations fill, so this script never keeps its own copy of it.
# Each pitch is rendered from the nearest recording in tonejs-instruments
# (CC BY 3.0, github.com/nbrosowsky/tonejs-instruments), re-pitched to match,
# then trimmed to 3 s with a 0.6 s fade, mono, MP3 96 kbps.
#
# Rendering needs ffmpeg and Bash 4+ (neither is provided by mise; see the
# README). Rendered files are cached in .voice-samples/
# (gitignored), so later runs upload without ffmpeg or network access.
#
# Usage: scripts/voice-samples.sh   (make db:reset runs it after seeding)
set -euo pipefail

CACHE_DIR=".voice-samples"
BUCKET="motifpath-content-media"
FFMPEG="${FFMPEG:-ffmpeg}"

# The tonejs-instruments folder each voice is rendered from.
declare -A UPSTREAM=(
  [acoustic-guitar]="guitar-acoustic"
  [electric-bass]="bass-electric"
  [piano]="piano"
)

# midi_of NAME prints the MIDI pitch of a tonejs sample name such as "As2"
# (A sharp 2), or nothing when NAME isn't one.
midi_of() {
  local name="$1" letter sharp octave base
  [[ "$name" =~ ^([A-G])(s?)(-?[0-9])$ ]] || return 0
  letter="${BASH_REMATCH[1]}" sharp="${BASH_REMATCH[2]}" octave="${BASH_REMATCH[3]}"
  case "$letter" in C) base=0 ;; D) base=2 ;; E) base=4 ;; F) base=5 ;; G) base=7 ;; A) base=9 ;; B) base=11 ;; esac
  [ -n "$sharp" ] && base=$((base + 1))
  echo $(((octave + 1) * 12 + base))
}

# download UPSTREAM DIR fetches every tonejs-instruments recording of
# UPSTREAM into DIR. The recordings land in a staging folder first and only
# replace DIR, marked complete, once every one of them has arrived, so an
# interrupted download is retried on the next run instead of being cached.
download() {
  local upstream="$1" dir="$2" staging="$2.partial" listing urls
  echo "   downloading $upstream recordings"
  rm -rf "$staging"
  mkdir -p "$staging"
  if ! listing="$(curl -fsS "https://api.github.com/repos/nbrosowsky/tonejs-instruments/contents/samples/$upstream")"; then
    echo "   couldn't list the $upstream recordings; will retry on the next run" >&2
    rm -rf "$staging"
    return 1
  fi
  urls="$(grep -o '"download_url": *"[^"]*\.mp3"' <<<"$listing" | sed -E 's/.*"(https[^"]+)"/\1/')"
  if [ -z "$urls" ] || ! (cd "$staging" && xargs -P 8 -n 1 curl -fsSO <<<"$urls"); then
    echo "   couldn't download every $upstream recording; will retry on the next run" >&2
    rm -rf "$staging"
    return 1
  fi
  touch "$staging/.complete"
  rm -rf "$dir"
  mv "$staging" "$dir"
}

# render VOICE PITCHES... writes CACHE_DIR/VOICE/PITCH.mp3 for every pitch
# not rendered yet, and fails if any of them couldn't be. It runs as the
# left side of `||`, where bash ignores `set -e`, so every failure is checked
# explicitly.
render() {
  local voice="$1"; shift
  local upstream="${UPSTREAM[$voice]:-}" originals="$CACHE_DIR/.originals/$voice"
  if [ -z "$upstream" ]; then
    echo "   no recordings known for voice '$voice'; skipping it" >&2
    return 0
  fi
  mkdir -p "$CACHE_DIR/$voice"

  local missing=()
  for pitch in "$@"; do
    [ -f "$CACHE_DIR/$voice/$pitch.mp3" ] || missing+=("$pitch")
  done
  [ "${#missing[@]}" -eq 0 ] && return 0

  if ! command -v "$FFMPEG" >/dev/null 2>&1; then
    echo "   ffmpeg not found: can't render ${#missing[@]} sample(s) of '$voice'." >&2
    echo "   Run this script again inside e.g. \`nix shell nixpkgs#ffmpeg-headless\`." >&2
    return 1
  fi

  if [ ! -f "$originals/.complete" ]; then
    download "$upstream" "$originals" || return 1
  fi

  # Every recording by pitch, to find the nearest one to each target.
  local -A recording=()
  local file name midi
  for file in "$originals"/*.mp3; do
    name="$(basename "$file" .mp3)"
    midi="$(midi_of "$name")"
    [ -n "$midi" ] && recording[$midi]="$file"
  done
  if [ "${#recording[@]}" -eq 0 ]; then
    echo "   no $upstream recordings to render '$voice' from" >&2
    return 1
  fi

  local pitch nearest distance best shift output partial rendered=0 failed=0
  for pitch in "${missing[@]}"; do
    best=""
    for midi in "${!recording[@]}"; do
      distance=$((midi > pitch ? midi - pitch : pitch - midi))
      if [ -z "$best" ] || [ "$distance" -lt "$best" ]; then best="$distance" nearest="$midi"; fi
    done
    shift=$((pitch - nearest))
    # Rendered under a temporary name, so a failed or interrupted render never
    # leaves a broken file that later runs would take as done.
    output="$CACHE_DIR/$voice/$pitch.mp3" partial="$CACHE_DIR/$voice/$pitch.partial.mp3"
    if "$FFMPEG" -v error -y -i "${recording[$nearest]}" \
      -af "aresample=44100,asetrate=44100*pow(2\,${shift}/12),aresample=44100,atrim=0:3,afade=t=out:st=2.4:d=0.6" \
      -ac 1 -b:a 96k "$partial" && mv "$partial" "$output"; then
      rendered=$((rendered + 1))
    else
      rm -f "$partial"
      echo "   failed to render sample $pitch of '$voice'" >&2
      failed=$((failed + 1))
    fi
  done
  echo "   rendered $rendered sample(s) of '$voice'"
  [ "$failed" -eq 0 ]
}

echo "==> Rendering voice samples"
failed=0
while IFS='|' read -r voice pitches; do
  [ -n "$voice" ] || continue
  # shellcheck disable=SC2086 # pitches is a space-separated list on purpose
  render "$voice" $pitches || failed=1
done < <(docker compose exec -T postgres psql -U motifpath -d core_domain -tA \
  -c "SELECT id || '|' || string_agg(p::text, ' ') FROM voices, jsonb_array_elements_text(pitches) AS p GROUP BY id")

echo "==> Uploading voice samples to the local object store"
docker compose up -d objectstore >/dev/null
docker compose run --rm -v "$(pwd)/$CACHE_DIR:/samples:ro" --entrypoint sh objectstore-init -c "
  aws s3api head-bucket --bucket $BUCKET 2>/dev/null || aws s3 mb s3://$BUCKET >/dev/null
  aws s3 sync /samples s3://$BUCKET/audio/voices --exclude '.originals/*' --exclude '*.partial.mp3' --only-show-errors
"
echo "==> voice samples done"
exit "$failed"
