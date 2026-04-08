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

### Semver Label Application

Semver labels are applied by the [deptriage](https://github.com/konflux-ci/deptriage) action during the classification phase. It detects the bump type from the PR body/title and applies the appropriate `semver/*` label, handling edge cases like two-component docker tags (`v9.5` → `v9.7`) and digest-only updates.

## Auto-merge Rules

| Update Type | Direct | Transitive | Action |
|-------------|--------|------------|--------|
| Patch | Auto-merge | Auto-merge | Merged automatically when CI passes |
| Gomod digest | Manual review | Manual review | Labeled `semver/minor` — pseudo-versions have no semver guarantees |
| Minor | Manual review | Manual review | Requires human approval |
| Major | Manual review | Manual review | Requires human approval |
| Konflux references (Tekton tasks) | N/A | N/A | Auto-merge when CI passes |
| Dockerfile updates | N/A | N/A | Auto-merge when CI passes |
| Go toolchain (go-toolset) | N/A | N/A | Auto-merge when CI passes (deferred approval) |
| RPM lockfile updates | N/A | N/A | Auto-merge when CI passes |

Auto-merge is handled by the [deptriage](https://github.com/konflux-ci/deptriage) action via two workflows:

- **dep-triage.yaml** — triggers on PR open/sync, classifies the bump type, runs AI-assisted risk analysis, and auto-approves eligible PRs with `approved`/`lgtm` labels and a formal APPROVE review.
- **auto-merge.yaml** — triggers on `check_suite: completed`, evaluates merge eligibility (labels + CI status), submits an APPROVE review via a GitHub App token, and merges when all conditions are met.

### Rationale

- **Patch bumps** are low-risk by semver convention (bug fixes only). Combined with CI validation, auto-approving reduces review burden without meaningful risk.
- **Gomod digest/pseudo-version bumps** use `v0.0.0-timestamp-hash` versions with no semver guarantees. Breaking API changes have been observed in k8s.io pseudo-version bumps, so these require manual review with AI impact analysis.
- **Minor/major bumps** may introduce new behavior or breaking changes and benefit from human review.
- **Konflux reference updates** are digest-only updates to build pipeline task references — they are validated by the Tekton pipeline CI.
- **Go toolchain updates** (`go-toolset` image) are flagged with risk hints and assessed as MEDIUM risk. Approval is deferred until the Konflux CI pipeline passes, proving the build is safe with the new toolchain.
- **AI risk level** is informational — MEDIUM risk does NOT block merge. Only HIGH risk prevents auto-merge and requires human review.

## PR Grouping

Go module updates are individual PRs (no grouping) to allow independent merging and failure isolation. One failing update does not block other safe updates.

- Patch bumps — Individual PRs per package
- Digest bumps — Individual PRs per package
- Minor bumps — Individual PRs per package
- Major bumps — Individual PRs per package
- `go-modules k8s` — Exception: Kubernetes-related packages (`k8s.io/*`, `sigs.k8s.io/controller-runtime`) are grouped together for minor/major/digest updates because they are tightly coupled and must be upgraded together

## Import Usage and Impact Analysis

For dependency PRs, the deptriage action gathers code-level usage context (via `go mod why`, `go mod graph`, and source scanning) and runs AI-assisted impact analysis. The analysis is posted as a PR comment, helping reviewers assess the impact of non-auto-merged updates.

## Override Procedure

To exclude a specific package from auto-merge, the deptriage action's risk detection will flag packages with known risk patterns. For additional overrides, add a `packageRules` entry in `renovate.json` with a label that deptriage recognizes as blocking (e.g., `risk/high`):

```json
{
  "description": "Require manual review for package-name",
  "matchPackageNames": ["github.com/example/package-name"],
  "addLabels": ["risk/high"]
}
```

## Rollback Procedure

If an auto-merged dependency update causes a regression:

1. **Revert the PR**: Create a revert PR for the problematic merge.
2. **Investigate**: Determine if the issue is in the dependency or in our usage of it.
3. **Resume**: Once resolved, the next dependency update PR will be triaged normally.

## Configuration

All policies are configured in [`.github/renovate.json`](../.github/renovate.json). Mintmaker's base config (`inheritConfig: true`) is merged with this repo-level config.
