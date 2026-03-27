# Dependency Update Triage Policy

This document describes how dependency update PRs (from Renovate/Mintmaker) are classified, labeled, and triaged in this repository.

## Classification

### Direct vs. Transitive

Dependencies are classified using Renovate's native `matchDepTypes`:

- **Direct** (`dependency/direct` label): Packages listed in `go.mod` without `// indirect`. These are imported directly by our source code.
- **Transitive** (`dependency/transitive` label): Packages listed in `go.mod` with `// indirect`. These are pulled in as dependencies of our direct dependencies.

### Semver Bump Type

Every dependency PR is labeled with the semver bump type:

- `semver/patch` — Patch or digest updates (e.g., 1.2.3 → 1.2.4). Bug fixes only, no new features or breaking changes per semver contract.
- `semver/minor` — Minor updates (e.g., 1.2.0 → 1.3.0). New features, backwards-compatible.
- `semver/major` — Major updates (e.g., 1.0.0 → 2.0.0). May contain breaking changes.

### Semver Label Fallback

Semver labels are primarily applied by Renovate via `packageRules`. As a fallback, a GitHub Action workflow (`.github/workflows/dep-semver-label.yaml`) detects the bump type from the PR body/title and applies the label if Renovate hasn't. This handles edge cases like two-component docker tags (`v9.5` → `v9.7`) and digest-only updates.

## Auto-merge Rules

| Update Type | Direct | Transitive | Action |
|-------------|--------|------------|--------|
| Patch/digest | Auto-merge | Auto-merge | Merged automatically when CI passes |
| Minor | Manual review | Manual review | Requires human approval |
| Major | Manual review | Manual review | Requires human approval |
| Konflux references (Tekton tasks) | N/A | N/A | Auto-merge when CI passes |
| Dockerfile updates | N/A | N/A | Auto-merge when CI passes |
| Go toolchain (go-toolset) | N/A | N/A | **Manual review required** |
| RPM lockfile updates | N/A | N/A | Auto-merge when CI passes |

Auto-merge uses GitHub's platform auto-merge (`platformAutomerge`), which requires all required status checks to pass before merging.

### Rationale

- **Patch bumps** are low-risk by semver convention (bug fixes only). Combined with CI validation, auto-merging reduces review burden without meaningful risk.
- **Minor/major bumps** may introduce new behavior or breaking changes and benefit from human review.
- **Konflux reference updates** are digest-only updates to build pipeline task references — they are validated by the Tekton pipeline CI.
- **Go toolchain updates** (`go-toolset` image) are excluded from auto-merge because they frequently require coordinated changes to build infrastructure (Tekton task images, Dockerfile base images) and have historically caused CI failures.

## PR Grouping

Go module updates are grouped to reduce PR volume:

- `go-modules patch` — All patch/digest bumps in one PR
- `go-modules minor` — All minor bumps in one PR
- Major bumps — Individual PRs per package (ungrouped)
- `go-modules k8s` — Kubernetes-related packages (`k8s.io/*`, `sigs.k8s.io/controller-runtime`) grouped together for minor/major/digest updates

## Import Usage Comments

For PRs updating direct dependencies, a GitHub Action posts a comment listing which source files import the package. This helps reviewers assess the impact of non-auto-merged updates (minor/major bumps).

## Override Procedure

To exclude a specific package from auto-merge, add a `packageRules` entry in `renovate.json`:

```json
{
  "description": "Require manual review for package-name",
  "matchPackageNames": ["github.com/example/package-name"],
  "automerge": false
}
```

Place this rule **after** the auto-merge rules so it takes precedence.

## Rollback Procedure

If an auto-merged dependency update causes a regression:

1. **Revert the PR**: Create a revert PR for the problematic merge.
2. **Disable auto-merge temporarily** (if needed): Set `automerge: false` on the relevant `packageRules` in `renovate.json`. This is a single-line change.
3. **Investigate**: Determine if the issue is in the dependency or in our usage of it.
4. **Re-enable**: Once resolved, remove the `automerge: false` override.

## Configuration

All policies are configured in [`.github/renovate.json`](../.github/renovate.json). Mintmaker's base config (`inheritConfig: true`) is merged with this repo-level config.
