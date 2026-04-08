# Dependency Update Triage Policy

This document describes how dependency update PRs (from Renovate/Mintmaker) are classified, labeled, and triaged in this repository.

## Classification

### Direct vs. Transitive

Dependencies are classified using Renovate's native `matchDepTypes`:

- **Direct** (`dependency/direct` label): Packages listed in `go.mod` without `// indirect`. These are imported directly by our source code.
- **Transitive** (`dependency/transitive` label): Packages listed in `go.mod` with `// indirect`. These are pulled in as dependencies of our direct dependencies.

### Semver Bump Type

Every dependency PR is labeled with the semver bump type:

- `semver/patch` — Patch updates (e.g., 1.2.3 → 1.2.4). Bug fixes only, no new features or breaking changes per semver contract. Non-gomod digest updates (e.g., container images) are also labeled as patch.
- `semver/minor` — Minor updates (e.g., 1.2.0 → 1.3.0). New features, backwards-compatible. **Gomod digest/pseudo-version bumps** (e.g., `v0.0.0-20250910...` → `v0.0.0-20260330...`) are also labeled as minor because pseudo-versions have no semver guarantees and may contain breaking changes.
- `semver/major` — Major updates (e.g., 1.0.0 → 2.0.0). May contain breaking changes.

### Semver Label Fallback

Semver labels are primarily applied by Renovate via `packageRules`. As a fallback, a GitHub Action workflow (`.github/workflows/dep-semver-label.yaml`) detects the bump type from the PR body/title and applies the label if Renovate hasn't. This handles edge cases like two-component docker tags (`v9.5` → `v9.7`) and digest-only updates.

## Auto-approve Rules

| Update Type | Direct | Transitive | Action |
|-------------|--------|------------|--------|
| Patch | Auto-approve | Auto-approve | `approved`/`lgtm` labels added after CI passes |
| Gomod digest | Manual review | Manual review | Labeled `semver/minor` — pseudo-versions have no semver guarantees |
| Minor | Manual review | Manual review | Requires human approval |
| Major | Manual review | Manual review | Requires human approval |
| Konflux references (Tekton tasks) | N/A | N/A | Auto-approve after CI passes |
| Dockerfile updates | N/A | N/A | Auto-approve after CI passes |
| Go toolchain (go-toolset) | N/A | N/A | **Manual review required** |
| RPM lockfile updates | N/A | N/A | Auto-approve after CI passes |

Auto-approve is handled by a post-CI GitHub Actions workflow (`.github/workflows/auto-approve.yaml`). It triggers after the "Tests", "Lint", and "E2E Tests" workflows complete, verifies all check runs passed, and only then applies `approved` and `lgtm` labels. This ensures PRs with failing tests are never labeled as approved.

### Rationale

- **Patch bumps** are low-risk by semver convention (bug fixes only). Combined with CI validation, auto-approving reduces review burden without meaningful risk.
- **Gomod digest/pseudo-version bumps** use `v0.0.0-timestamp-hash` versions with no semver guarantees. Breaking API changes have been observed in k8s.io pseudo-version bumps, so these require manual review with AI impact analysis.
- **Minor/major bumps** may introduce new behavior or breaking changes and benefit from human review.
- **Konflux reference updates** are digest-only updates to build pipeline task references — they are validated by the Tekton pipeline CI.
- **Go toolchain updates** (`go-toolset` image) are excluded from auto-approve because they frequently require coordinated changes to build infrastructure (Tekton task images, Dockerfile base images) and have historically caused CI failures.

## PR Grouping

Go module updates are individual PRs (no grouping) to allow independent merging and failure isolation. One failing update does not block other safe updates.

- Patch bumps — Individual PRs per package
- Digest bumps — Individual PRs per package
- Minor bumps — Individual PRs per package
- Major bumps — Individual PRs per package
- `go-modules k8s` — Exception: Kubernetes-related packages (`k8s.io/*`, `sigs.k8s.io/controller-runtime`) are grouped together for minor/major/digest updates because they are tightly coupled and must be upgraded together

## Import Usage Comments

For PRs updating direct dependencies, a GitHub Action posts a comment listing which source files import the package. This helps reviewers assess the impact of non-auto-merged updates (minor/major bumps).

## Override Procedure

To exclude a specific package from auto-approve, add a `packageRules` entry in `renovate.json` to force it to a non-patch label:

```json
{
  "description": "Require manual review for package-name",
  "matchPackageNames": ["github.com/example/package-name"],
  "addLabels": ["semver/minor"]
}
```

This ensures the auto-approve workflow skips the PR since it only acts on `semver/patch` labels.

## Rollback Procedure

If an auto-approved dependency update causes a regression:

1. **Revert the PR**: Create a revert PR for the problematic merge.
2. **Disable auto-approve temporarily** (if needed): Disable the `auto-approve.yaml` workflow or add the package to the override list.
3. **Investigate**: Determine if the issue is in the dependency or in our usage of it.
4. **Re-enable**: Once resolved, remove the override.

## Configuration

All policies are configured in [`.github/renovate.json`](../.github/renovate.json). Mintmaker's base config (`inheritConfig: true`) is merged with this repo-level config.
