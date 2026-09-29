#!/usr/bin/env bash
# voice-samples.sh — renders every voice's samples and uploads them to the
# LOCAL dev MinIO, where the voice list says players download them from
# (audio/voices/{voice_id}/{midi}.mp3 under the content-media bucket).
#
# Which pitches a voice has is read from the local database's voices table,
# which the migrations fill, so this script never keeps its own copy of it.
# Each pitch is rendered from the nearest recording in tonejs-instruments
# (CC BY 3.0, github.com/nbrosowsky/tonejs-instruments), re-pitched to match,
# then trimmed to 3 s with a 0.6 s fade, mono, MP3 96 kbps.
#
# Rendering needs ffmpeg (not part of devbox; e.g. `nix shell
# nixpkgs#ffmpeg-headless`). Rendered files are cached in .voice-samples/
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

# render VOICE PITCHES... writes CACHE_DIR/VOICE/PITCH.mp3 for every pitch
# not rendered yet.
render() {
  local voice="$1"; shift
  local upstream="${UPSTREAM[$voice]:-}" originals="$CACHE_DIR/.originals/$voice"
  if [ -z "$upstream" ]; then
    echo "   no recordings known for voice '$voice'; skipping it" >&2
    return 0
  fi
  mkdir -p "$CACHE_DIR/$voice" "$originals"

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

  if [ -z "$(ls -A "$originals")" ]; then
    echo "   downloading $upstream recordings"
    curl -fsS "https://api.github.com/repos/nbrosowsky/tonejs-instruments/contents/samples/$upstream" \
      | grep -o '"download_url": *"[^"]*\.mp3"' | sed -E 's/.*"(https[^"]+)"/\1/' \
      | (cd "$originals" && xargs -P 8 -n 1 curl -fsSO)
  fi

  # Every recording by pitch, to find the nearest one to each target.
  local -A recording=()
  local file name midi
  for file in "$originals"/*.mp3; do
    name="$(basename "$file" .mp3)"
    midi="$(midi_of "$name")"
    [ -n "$midi" ] && recording[$midi]="$file"
  done

  local pitch nearest distance best shift
  for pitch in "${missing[@]}"; do
    best=""
    for midi in "${!recording[@]}"; do
      distance=$((midi > pitch ? midi - pitch : pitch - midi))
      if [ -z "$best" ] || [ "$distance" -lt "$best" ]; then best="$distance" nearest="$midi"; fi
    done
    shift=$((pitch - nearest))
    "$FFMPEG" -v error -y -i "${recording[$nearest]}" \
      -af "aresample=44100,asetrate=44100*pow(2\,${shift}/12),aresample=44100,atrim=0:3,afade=t=out:st=2.4:d=0.6" \
      -ac 1 -b:a 96k "$CACHE_DIR/$voice/$pitch.mp3"
  done
  echo "   rendered ${#missing[@]} sample(s) of '$voice'"
}

echo "==> Rendering voice samples"
failed=0
while IFS='|' read -r voice pitches; do
  [ -n "$voice" ] || continue
  # shellcheck disable=SC2086 # pitches is a space-separated list on purpose
  render "$voice" $pitches || failed=1
done < <(docker compose exec -T postgres psql -U motifpath -d core_domain -tA \
  -c "SELECT id || '|' || string_agg(p::text, ' ') FROM voices, jsonb_array_elements_text(pitches) AS p GROUP BY id")

echo "==> Uploading voice samples to local MinIO"
docker compose up -d minio >/dev/null
docker compose run --rm --no-deps -v "$(pwd)/$CACHE_DIR:/samples:ro" --entrypoint sh minio-init -c "
  mc alias set local http://minio:9000 motifpath motifpath >/dev/null &&
  mc mb --ignore-existing local/$BUCKET >/dev/null &&
  mc anonymous set download local/$BUCKET >/dev/null &&
  mc mirror --overwrite --exclude '.originals/*' /samples local/$BUCKET/audio/voices
" >/dev/null
echo "==> voice samples done"
exit "$failed"
