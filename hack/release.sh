#!/usr/bin/env bash

set -euo pipefail

: "${IMG:?set IMG to an immutable image reference containing @sha256:}"
: "${VERSION:?set VERSION to the release version}"

if [[ "$IMG" != *@sha256:* ]]; then
  echo "IMG must contain an immutable sha256 digest" >&2
  exit 1
fi

RELEASE_DIR=${RELEASE_DIR:-release}
export RELEASE_DIR VERSION

cd "$(dirname "${BASH_SOURCE[0]}")/.."
make release RELEASE_IMAGE="$IMG"
(
  cd "$RELEASE_DIR"
  sha256sum ./*.yaml > SHA256SUMS
)
