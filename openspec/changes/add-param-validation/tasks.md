## 1. CEL Type Guards (infra-deployments)

- [ ] 1.1 Add `type()` guard to the build-platforms resource request expression (lines 5-14 of config.yaml): check `type(value) == list` before `.map()`
- [ ] 1.2 Add `type()` guard to the build-platforms AWS IP expression (lines 17-37 of config.yaml): check `type(value) == list` before `.filter()`
- [ ] 1.3 Add new CEL expression for validation-error annotation: detect `build-platforms` present but `type(value) != list`, set `tekton-kueue.konflux-ci.dev/validation-error` annotation

## 2. ValidatingAdmissionPolicy (infra-deployments)

- [ ] 2.1 Create ValidatingAdmissionPolicy resource that denies PipelineRuns with `tekton-kueue.konflux-ci.dev/validation-error` annotation
- [ ] 2.2 Create ValidatingAdmissionPolicyBinding to apply the policy to PipelineRun resources
- [ ] 2.3 Add both resources to the kustomization.yaml

## 3. Tests (infra-deployments)

- [ ] 3.1 Add PipelineRun test case in `hack/test-tekton-kueue-config.py` with `build-platforms` as a string param
- [ ] 3.2 Define expected results: validation-error annotation present, no platform resource request annotations
- [ ] 3.3 Run `python hack/test-tekton-kueue-config.py` — all tests pass including new string-typed build-platforms case

## 4. Verification

- [ ] 4.1 Verify the modified CEL expressions compile by running the test script against the production base config
- [ ] 4.2 Verify existing array-typed build-platforms tests still pass (no regression)
