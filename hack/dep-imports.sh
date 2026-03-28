#!/usr/bin/env bash
# dep-imports.sh — List source files that import a given Go package.
# Usage: ./hack/dep-imports.sh <package-name>
# Example: ./hack/dep-imports.sh github.com/onsi/gomega
#
# Exits 0 if imports found, 1 if no imports found, 2 on usage error.

set -euo pipefail

if [[ $# -lt 1 ]]; then
    echo "Usage: $0 <package-name>" >&2
    exit 2
fi

PACKAGE="$1"
ROOT="${2:-.}"

# Search Go source files for import statements matching the package.
# Uses fixed-string matching (-F) because Go module paths contain dots
# that would be regex metacharacters.
MATCHES=$(grep -rnF --include='*.go' "\"${PACKAGE}" "${ROOT}" |
    grep -v '_test.go' |
    grep -v '/vendor/' |
    grep -v '/hack/' ||
    true)

if [[ ! "${MATCHES}" ]]; then
    echo "No imports found for package: ${PACKAGE}"
    exit 1
fi

echo "Files importing ${PACKAGE}:"
echo "${MATCHES}"
exit 0
