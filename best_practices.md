
<b>Pattern 1: When introducing queuing/scheduling behavior for PipelineRuns, ensure the user-visible PipelineRun state reflects it (e.g., set `spec.status` to `Pending` when queued) and keep that behavior consistent across modes/config (e.g., MultiKueue override should not change whether Pending is set).
</b>

Example code before:
```
func (h *Webhook) Default(ctx context.Context, pr *tektonv1.PipelineRun) error {
  if h.cfg.MultiKueueOverride {
    pr.Spec.ManagedBy = "kueue.x-k8s.io/multikueue"
  }
  // queued state not reflected in the PipelineRun
  return nil
}
```

Example code after:
```
func (h *Webhook) Default(ctx context.Context, pr *tektonv1.PipelineRun) error {
  // Always reflect queuing intent in the PipelineRun UX.
  pr.Spec.Status = tektonv1.PipelineRunSpecStatusPending

  if h.cfg.MultiKueueOverride {
    pr.Spec.ManagedBy = "kueue.x-k8s.io/multikueue"
  }
  return nil
}
```

<details><summary>Examples for relevant past discussions:</summary>

- https://github.com/konflux-ci/tekton-kueue/pull/160#discussion_r2582011045
- https://github.com/konflux-ci/tekton-kueue/pull/160#discussion_r2582021227
</details>


___

<b>Pattern 2: When adding helpers/APIs, keep symbol visibility and defensive checks aligned with actual usage: unexport functions not used outside the package and validate assumptions on inputs (e.g., slice length) before indexing.
</b>

Example code before:
```
// Exported but only used internally; can panic if Parameters() is empty.
func ValidateExpressionReturnType(ast *cel.Ast) error {
  t := ast.OutputType()
  elem := t.Parameters()[0]
  _ = elem
  return nil
}
```

Example code after:
```
// Unexported if package-private, and validates before indexing.
func validateExpressionReturnType(ast *cel.Ast) error {
  t := ast.OutputType()
  params := t.Parameters()
  if len(params) == 0 {
    return fmt.Errorf("invalid output type: missing parameters")
  }
  // ...rest of validation...
  return nil
}
```

<details><summary>Examples for relevant past discussions:</summary>

- https://github.com/konflux-ci/tekton-kueue/pull/76#discussion_r2204573813
</details>


___

<b>Pattern 3: In tests, prefer standard library helpers and clearer assertion patterns to reduce custom boilerplate and improve readability (e.g., `maps.Clone` over bespoke map-copy functions, split complex table-driven cases into separate tests when it improves clarity, and use Gomega helpers like `GinkgoWriter` and `.Error()` for multi-return functions where appropriate).
</b>

Example code before:
```
func copyMap(m map[string]string) map[string]string {
  out := make(map[string]string, len(m))
  for k, v := range m {
    out[k] = v
  }
  return out
}

out, err := utils.Run(cmd)
Expect(err).NotTo(HaveOccurred())
_ = out
```

Example code after:
```
cloned := maps.Clone(m)

// For multi-return funcs, assert the trailing error directly.
Expect(utils.Run(cmd)).Error().NotTo(HaveOccurred())
```

<details><summary>Examples for relevant past discussions:</summary>

- https://github.com/konflux-ci/tekton-kueue/pull/76#discussion_r2204587353
- https://github.com/konflux-ci/tekton-kueue/pull/76#discussion_r2204755571
- https://github.com/konflux-ci/tekton-kueue/pull/41#discussion_r2135431945
- https://github.com/konflux-ci/tekton-kueue/pull/41#discussion_r2135448455
</details>


___

<b>Pattern 4: In build/test automation (Makefiles, CI, container builds), avoid hardcoding tools and unstable identifiers: parameterize container tooling via variables (e.g., `$(CONTAINER_TOOL)`, `$(KUBECTL)`), clean up temporary artifacts, keep version sources centralized to prevent duplication, and prefer pinning container base images by digest for reproducible builds.
</b>

Example code before:
```
load-image:
  dir=$$(mktemp -d) && podman save $(IMG) -o $${dir}/img.tar && kind load image-archive $${dir}/img.tar

FROM registry.access.redhat.com/ubi9/go-toolset:9.5 AS builder
```

Example code after:
```
load-image:
  dir=$$(mktemp -d) && $(CONTAINER_TOOL) save $(IMG) -o $${dir}/img.tar && \
    kind load image-archive -n $(KIND_CLUSTER) $${dir}/img.tar && \
    rm -rf "$${dir}"

FROM registry.access.redhat.com/ubi9/go-toolset@sha256:<digest> AS builder
```

<details><summary>Examples for relevant past discussions:</summary>

- https://github.com/konflux-ci/tekton-kueue/pull/21#discussion_r2073136796
- https://github.com/konflux-ci/tekton-kueue/pull/21#discussion_r2073296806
- https://github.com/konflux-ci/tekton-kueue/pull/11#discussion_r2001512882
</details>


___
