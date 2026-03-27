#!/usr/bin/env bash
# gather-dep-context.sh — Gather context about dependency updates for AI analysis.
# Usage: ./hack/gather-dep-context.sh <pr-body-file> [pr-title]
#
# Reads a Renovate/Mintmaker PR body from a file, extracts package names,
# and gathers import usage, source snippets, and test coverage for each package.
# Outputs structured JSON to stdout.
#
# Exits 0 on success, 2 on usage error.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

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

PR_BODY=$(cat "${PR_BODY_FILE}")

# --- Extract package names from PR body ---
# Renovate lists packages in markdown table rows in two formats:
#   Bare:   | github.com/foo/bar | 1.0.0 -> 1.0.1 | ... |
#   Linked: | [github.com/foo/bar](https://...) | `v1.0.0` -> `v1.0.1` | ... |
# We handle both by first stripping markdown links and backticks, then extracting module paths.
PACKAGES=$(echo "${PR_BODY}" \
    | sed 's/\[\([^]]*\)\]([^)]*)/\1/g' \
    | sed 's/`//g' \
    | grep -oP '(?<=\| )[a-zA-Z0-9._-]+\.[a-zA-Z]{2,}/[a-zA-Z0-9._/-]+(?= \|)' \
    | sort -u \
    || true)

if [[ ! "${PACKAGES}" && "${PR_TITLE}" ]]; then
    # Fallback: extract from PR title
    PACKAGES=$(echo "${PR_TITLE}" \
        | grep -oP '[a-zA-Z0-9._-]+\.[a-zA-Z]{2,}/[a-zA-Z0-9._/-]+' \
        || true)
fi

if [[ ! "${PACKAGES}" ]]; then
    echo '{"packages":[]}'
    exit 0
fi

# --- Extract changelog from PR body ---
extract_changelog() {
    local pkg="$1"

    # Try to extract the release notes section for this specific package.
    local changelog
    changelog=$(echo "${PR_BODY}" \
        | sed -n "/### .*${pkg//\//\\/}/,/### \[*[a-zA-Z]/p" \
        | head -100 \
        || true)

    if [[ -z "${changelog}" ]]; then
        # Fallback: use the PR body up to the first "---" separator or
        # "### Configuration" section (strips Renovate boilerplate)
        changelog=$(echo "${PR_BODY}" \
            | sed '/^### Configuration$/,$d' \
            | sed '/^---$/,$d' \
            | head -100)
    fi

    echo "${changelog}"
}

# --- Extract source snippets around import usage ---
extract_snippets() {
    local file="$1"
    local pkg="$2"
    local result=""

    # Show 5 lines of context around each usage of the package
    while IFS=: read -r line_num _; do
        local start=$((line_num - 5))
        [[ ${start} -lt 1 ]] && start=1
        local end=$((line_num + 5))
        local chunk
        chunk=$(sed -n "${start},${end}p" "${file}" 2>/dev/null || true)
        if [[ -n "${result}" ]]; then
            result+=$'\n---\n'
        fi
        result+="${chunk}"
    done < <(grep -n -F "${pkg}" "${file}" 2>/dev/null || true)

    echo "${result}"
}

# --- Check for test file ---
has_test_file() {
    local file="$1"
    local dir
    dir=$(dirname "${file}")
    local base
    base=$(basename "${file}" .go)
    if [[ -f "${dir}/${base}_test.go" ]]; then
        return 0
    fi
    ls "${dir}"/*_test.go &>/dev/null
}

# --- Strip Renovate boilerplate from PR body to get the useful content ---
# Remove everything from "### Configuration" onward (schedule, automerge, rebasing, etc.)
PR_BODY_CLEAN=$(echo "${PR_BODY}" \
    | sed '/^### Configuration$/,$d' \
    | sed '/^<!--renovate-debug:/,$d')

# --- Detect high-risk patterns ---
RISK_HINTS=""

# Go version/toolchain update (e.g., go-toolset image bump, go directive change)
if echo "${PR_TITLE}" | grep -qiE 'go-toolset|golang.*docker|docker.*golang'; then
    RISK_HINTS+="GO_TOOLCHAIN_UPDATE: This PR updates the Go build toolchain image. "
    RISK_HINTS+="This often requires coordinated changes to the build pipeline (e.g., Tekton task images, Dockerfile base images) "
    RISK_HINTS+="and can cause build failures if the new Go version is incompatible with the current build infrastructure. "
    RISK_HINTS+="These updates are historically HIGH risk in this project.\n"
fi

# Check if go.mod go directive is being changed
if echo "${PR_BODY}" | grep -qiE 'go\s+1\.\d+.*->.*go\s+1\.\d+|update.*go.*directive'; then
    RISK_HINTS+="GO_VERSION_BUMP: The Go language version directive in go.mod may be changing. "
    RISK_HINTS+="This can introduce new language features that require a matching Go toolchain version in CI, "
    RISK_HINTS+="and may break builds if the CI build image uses an older Go version.\n"
fi

# Container image / Dockerfile base image updates
if echo "${PR_TITLE}" | grep -qiE 'docker|container|image|registry\.(access\.)?redhat'; then
    RISK_HINTS+="CONTAINER_IMAGE_UPDATE: This PR updates a container base image. "
    RISK_HINTS+="Base image changes can affect build behavior, available system libraries, and binary compatibility.\n"
fi

# --- Gather data for each package, then use python3 to build valid JSON ---
# Write intermediate data to temp files so python3 can assemble it cleanly.
TMPDIR=$(mktemp -d)
trap 'rm -rf "${TMPDIR}"' EXIT

# Save the cleaned PR body for inclusion in the output
echo "${PR_BODY_CLEAN}" > "${TMPDIR}/pr_body"

# Save risk hints
echo -e "${RISK_HINTS}" > "${TMPDIR}/risk_hints"

pkg_index=0
while IFS= read -r pkg; do
    [[ -z "${pkg}" ]] && continue

    pkg_dir="${TMPDIR}/${pkg_index}"
    mkdir -p "${pkg_dir}"

    echo "${pkg}" > "${pkg_dir}/name"

    # Changelog
    extract_changelog "${pkg}" > "${pkg_dir}/changelog"

    # Import usage — dep-imports.sh may not exist if Phase 1 hasn't been merged
    DEP_IMPORTS="${SCRIPT_DIR}/dep-imports.sh"
    if [[ ! -x "${DEP_IMPORTS}" ]]; then
        echo "true" > "${pkg_dir}/no_imports"
        echo "dep-imports.sh not found, skipping import analysis" >&2
        pkg_index=$((pkg_index + 1))
        continue
    fi

    import_output=$("${DEP_IMPORTS}" "${pkg}" 2>&1) || true

    if echo "${import_output}" | grep -q "No imports found"; then
        echo "true" > "${pkg_dir}/no_imports"
    else
        echo "false" > "${pkg_dir}/no_imports"

        # Parse file paths (format: "file:line: content")
        files=$(echo "${import_output}" | grep -v "^Files importing" | cut -d: -f1 | sort -u || true)

        file_index=0
        while IFS= read -r file; do
            [[ -z "${file}" ]] && continue

            file_dir="${pkg_dir}/files/${file_index}"
            mkdir -p "${file_dir}"

            echo "${file}" > "${file_dir}/path"
            extract_snippets "${file}" "${pkg}" > "${file_dir}/snippet"

            if has_test_file "${file}"; then
                echo "true" > "${file_dir}/has_test"
            else
                echo "false" > "${file_dir}/has_test"
            fi

            file_index=$((file_index + 1))
        done <<< "${files}"
    fi

    pkg_index=$((pkg_index + 1))
done <<< "${PACKAGES}"

# --- Use python3 to build valid JSON from the gathered data ---
python3 - "${TMPDIR}" << 'PYEOF'
import json
import os
import sys

tmpdir = sys.argv[1]

# Read the cleaned PR body (release notes, changelogs, version diffs)
pr_body = open(os.path.join(tmpdir, "pr_body")).read().strip()

# Read risk hints (known high-risk patterns detected)
risk_hints = open(os.path.join(tmpdir, "risk_hints")).read().strip()

packages = []

pkg_index = 0
while True:
    pkg_dir = os.path.join(tmpdir, str(pkg_index))
    if not os.path.isdir(pkg_dir):
        break

    name = open(os.path.join(pkg_dir, "name")).read().strip()
    changelog = open(os.path.join(pkg_dir, "changelog")).read().strip()
    no_imports = open(os.path.join(pkg_dir, "no_imports")).read().strip() == "true"

    pkg_data = {
        "name": name,
        "changelog": changelog,
        "imports": [],
    }

    if no_imports:
        pkg_data["noDirectImports"] = True
    else:
        files_dir = os.path.join(pkg_dir, "files")
        if os.path.isdir(files_dir):
            file_index = 0
            while True:
                file_dir = os.path.join(files_dir, str(file_index))
                if not os.path.isdir(file_dir):
                    break

                file_path = open(os.path.join(file_dir, "path")).read().strip()
                snippet = open(os.path.join(file_dir, "snippet")).read().strip()
                has_test = open(os.path.join(file_dir, "has_test")).read().strip() == "true"

                pkg_data["imports"].append({
                    "file": file_path,
                    "hasTest": has_test,
                    "snippet": snippet,
                })

                file_index += 1

    packages.append(pkg_data)
    pkg_index += 1

output = {"prBody": pr_body, "packages": packages}
if risk_hints:
    output["riskHints"] = risk_hints
print(json.dumps(output, indent=2))
PYEOF
