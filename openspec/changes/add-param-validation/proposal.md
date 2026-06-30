## Why

The webhook's CEL expressions assume certain PipelineRun parameters have specific types (e.g., `build-platforms` is an array). When a PipelineRun submits a param with the wrong type, the CEL `.map()` call crashes at runtime, returning HTTP 500 (InternalServerError). This was observed in production on kflux-prd-rh02 (2026-06-09). Additionally, during the no-skip-kueue rollout, a malformed PipelineRun that passes without resource requests would effectively skip the queue.

## What Changes

All changes are in infra-deployments — no tekton-kueue code changes required.

- Add `type()` guards to the build-platforms CEL expressions so they return `[]` instead of crashing when the value is a string. This eliminates the HTTP 500.
- Add a new CEL expression that detects wrong-typed `build-platforms` and sets a `tekton-kueue.konflux-ci.dev/validation-error` annotation with an error message. This provides operator visibility.
- Add a ValidatingAdmissionPolicy that denies PipelineRuns carrying the validation-error annotation. This is needed during the no-skip-kueue rollout to prevent queue bypass; it can be removed once the rollout completes.
- Add test cases to `hack/test-tekton-kueue-config.py` for string-typed `build-platforms`.

## Capabilities

### New Capabilities

- `cel-type-guard`: CEL `type()` guards on build-platforms expressions and a validation-error annotation expression, all in infra-deployments config.yaml.
- `validating-admission-policy`: A Kubernetes ValidatingAdmissionPolicy that denies PipelineRuns with the validation-error annotation (temporary, for no-skip-kueue rollout).

### Modified Capabilities

## Impact

- `components/kueue/production/base/tekton-kueue/config.yaml` — modified CEL expressions + new annotation expression
- `components/kueue/production/base/` — new ValidatingAdmissionPolicy resource
- `hack/test-tekton-kueue-config.py` — new test case for string-typed build-platforms
- No changes to tekton-kueue Go code
