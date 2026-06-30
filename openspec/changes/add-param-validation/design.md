## Context

The tekton-kueue webhook intercepts PipelineRun creation and applies CEL-based mutations (resource requests, priorities, labels). CEL expressions are configured in a ConfigMap deployed from infra-deployments. The expressions call `.map()` and `.filter()` on `build-platforms` param values, assuming they are arrays. When a PipelineRun submits `build-platforms` as a string, the CEL runtime crashes with HTTP 500 (`EvaluationError`). The webhook uses `failurePolicy: Fail`, so this blocks PipelineRun creation entirely.

The tekton-kueue CEL environment includes `cel.StdLib()`, which provides the `type()` function. After JSON marshaling (how PipelineRun params are passed to CEL), string params become CEL `string` and array params become CEL `list(dyn)`. This means `type(x) == list` reliably distinguishes the two cases.

## Goals / Non-Goals

**Goals:**
- Prevent HTTP 500 by adding `type()` guards to CEL expressions that call `.map()`/`.filter()` on param values
- Provide operator visibility via a validation-error annotation when a param has the wrong type
- Deny malformed PipelineRuns during the no-skip-kueue rollout via ValidatingAdmissionPolicy
- Keep tekton-kueue code unchanged — all fixes are config-level in infra-deployments

**Non-Goals:**
- Modifying tekton-kueue Go code (controller, webhook, CEL engine)
- Validating param types beyond `build-platforms` (can be extended later with the same pattern)
- Providing user-facing error propagation through PaC (PaC doesn't propagate admission errors)

## Decisions

### Config-only fix using CEL `type()` guards
The CEL expressions themselves are modified to check `type(value) == list` before calling `.map()`. When the type is wrong, the expression returns `[]` (no resource request). This is preferred over a Go-level validation layer because it requires zero code changes to tekton-kueue, keeping the project simple. The existing `type()` function from `cel.StdLib()` already supports this.

### Validation-error annotation for observability
A separate CEL expression detects wrong-typed params and adds a `tekton-kueue.konflux-ci.dev/validation-error` annotation. This leaves a debug trail on the PipelineRun object, making it easy to find and diagnose affected runs. The `annotation()` function is already available in the CEL environment.

### ValidatingAdmissionPolicy for denial (temporary)
During the no-skip-kueue rollout, PipelineRuns with wrong-typed params must be denied — otherwise they'd be admitted with no resource requests, bypassing the queue. A Kubernetes-native `ValidatingAdmissionPolicy` reads the validation-error annotation and denies the request. This is decoupled from the webhook and can be removed independently once the rollout completes.

After the rollout, the annotation-only approach is sufficient: the PipelineRun is admitted (queued correctly for other resources) with an explanatory annotation, which is a better UX than outright rejection since PaC doesn't propagate admission errors to users.

### CEL expression structure
The type guard is added inline to each expression that accesses `.value.map()` or `.value.filter()`:

```cel
has(pipelineRun.spec.params) &&
pipelineRun.spec.params.exists(p, p.name == 'build-platforms') &&
type(pipelineRun.spec.params.filter(p, p.name == 'build-platforms')[0].value) == list ?
pipelineRun.spec.params.filter(p, p.name == 'build-platforms')[0]
.value.map(p, resource(...)) : []
```

The error annotation expression uses the inverse check (`!= list`):
```cel
has(pipelineRun.spec.params) &&
pipelineRun.spec.params.exists(p, p.name == 'build-platforms') &&
type(pipelineRun.spec.params.filter(p, p.name == 'build-platforms')[0].value) != list ?
annotation('tekton-kueue.konflux-ci.dev/validation-error',
  'build-platforms must be an array of platform strings') : []
```

## Risks / Trade-offs

- **CEL expression complexity** — Adding `type()` guards makes the expressions longer and harder to read. → Acceptable trade-off for zero code changes; the expressions are already complex.
- **ValidatingAdmissionPolicy lifecycle** — Must be removed after no-skip-kueue rollout or it will continue denying requests unnecessarily. → Document the removal step; the policy is isolated in its own kustomize resource.
- **Pattern not generalized** — Each new type-sensitive param needs manual guards added to the CEL expressions. → Acceptable for now; `build-platforms` is the only known case. If more arise, a Go-level validation layer can be reconsidered.
- **`type()` comparison syntax** — Need to verify that `type(x) == list` compiles and evaluates correctly in the tekton-kueue CEL environment (confirmed: `cel.StdLib()` provides `type()`, and JSON-unmarshaled arrays become CEL `list(dyn)` which matches the `list` type).
