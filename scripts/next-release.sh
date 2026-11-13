#!/usr/bin/env bash
# Prints the next release tag for the repository in the current directory.
#
# Usage: next-release.sh [LABEL...]
#
# The arguments are the labels of the PR merged into main. Exactly one of them
# must be release:major, release:minor or release:patch; it bumps the newest
# vX.Y.Z tag. Other labels are ignored. Fails, printing why, when there is no
# release label or more than one.
set -euo pipefail

bump=
count=0
for label in "$@"; do
  case "$label" in
    release:major | release:minor | release:patch)
      bump=${label#release:}
      count=$((count + 1))
      ;;
    release:*)
      echo "unknown release label '$label': use release:major, release:minor or release:patch" >&2
      exit 1
      ;;
  esac
done

if ((count == 0)); then
  echo "the PR needs a release label: release:major, release:minor or release:patch" >&2
  exit 1
fi
if ((count > 1)); then
  echo "the PR has $count release labels; it needs exactly one" >&2
  exit 1
fi

latest=$(git tag --list 'v*' | grep -E '^v[0-9]+\.[0-9]+\.[0-9]+$' | sort -V | tail -n 1 || true)
IFS=. read -r major minor patch <<<"${latest#v}"
major=${major:-0} minor=${minor:-0} patch=${patch:-0}

case "$bump" in
  major) echo "v$((major + 1)).0.0" ;;
  minor) echo "v$major.$((minor + 1)).0" ;;
  patch) echo "v$major.$minor.$((patch + 1))" ;;
esac
