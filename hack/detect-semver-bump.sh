#!/usr/bin/env bash
# detect-semver-bump.sh — Detect the semver bump type from a Renovate PR.
# Usage: ./hack/detect-semver-bump.sh <pr-body-file> [pr-title]
#
# Analyzes version changes in the PR body/title and outputs one of:
#   major, minor, patch, digest, unknown
#
# Exits 0 on success, 2 on usage error.

set -euo pipefail

if [[ $# -lt 1 ]]; then
    echo "Usage: $0 <pr-body-file> [pr-title]" >&2
    exit 2
fi

PR_BODY_FILE="$1"
PR_TITLE="${2:-}"

if [[ ! -f "${PR_BODY_FILE}" ]]; then
    echo "Error: PR body file not found: ${PR_BODY_FILE}" >&2
    exit 2
fi

# Combine body and title for analysis
COMBINED=$(cat "${PR_BODY_FILE}")
if [[ -n "${PR_TITLE}" ]]; then
    COMBINED="${PR_TITLE}\n${COMBINED}"
fi

echo -e "${COMBINED}" | python3 -c '
import sys, re

text = sys.stdin.read()

# Try three-component semver first: v1.5.0 -> v1.9.0
version_pairs_3 = re.findall(
    r"`?v?(\d+)\.(\d+)\.(\d+)[^`]*`?\s*->\s*`?v?(\d+)\.(\d+)\.(\d+)",
    text
)

# Also try two-component versions (e.g., docker tags: v9.5-build -> v9.7-build)
version_pairs_2 = re.findall(
    r"`?v?(\d+)\.(\d+)(?:[-.][^`\s]*)?`?\s*->\s*`?v?(\d+)\.(\d+)(?:[-.][^`\s]*)?`?",
    text
)

if not version_pairs_3 and not version_pairs_2:
    # Check for digest updates: `054e65f` -> `716be56`
    if re.search(r"`[0-9a-f]{7,}`\s*->\s*`[0-9a-f]{7,}`", text):
        print("digest")
    else:
        print("unknown")
    sys.exit(0)

# Determine the highest bump across all version pairs.
highest = "patch"

for old_maj, old_min, old_pat, new_maj, new_min, new_pat in version_pairs_3:
    if int(new_maj) != int(old_maj):
        highest = "major"
        break
    elif int(new_min) != int(old_min):
        highest = "minor"

if highest != "major":
    for old_maj, old_min, new_maj, new_min in version_pairs_2:
        if int(new_maj) != int(old_maj):
            highest = "major"
            break
        elif int(new_min) != int(old_min):
            highest = "minor"

print(highest)
'
