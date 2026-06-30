## ADDED Requirements

### Requirement: CEL type guards prevent crashes on wrong param types
The build-platforms CEL expressions in config.yaml SHALL include a `type()` guard that checks `type(value) == list` before calling `.map()` or `.filter()` on the param value. When the value is not a list, the expression SHALL return `[]` instead of crashing.

#### Scenario: String build-platforms does not crash
- **WHEN** a PipelineRun has param `build-platforms` with type `string`
- **THEN** the resource request expression SHALL return `[]` (no resource requests)
- **AND** the webhook SHALL NOT return HTTP 500

#### Scenario: Array build-platforms works as before
- **WHEN** a PipelineRun has param `build-platforms` with type `array` containing `["linux/amd64", "linux/arm64"]`
- **THEN** the resource request expression SHALL return resource requests for each platform
- **AND** the AWS IP expression SHALL return AWS IP requests for eligible platforms

#### Scenario: Absent build-platforms works as before
- **WHEN** a PipelineRun does not have a `build-platforms` param
- **THEN** the existing `exists()` guard SHALL cause the expression to return `[]`

### Requirement: Validation-error annotation on wrong param type
A CEL expression SHALL detect when `build-platforms` is present but not a list, and set the annotation `tekton-kueue.konflux-ci.dev/validation-error` with an error message describing the type mismatch.

#### Scenario: String build-platforms gets error annotation
- **WHEN** a PipelineRun has param `build-platforms` with type `string`
- **THEN** the PipelineRun SHALL have annotation `tekton-kueue.konflux-ci.dev/validation-error` set
- **AND** the annotation value SHALL describe that build-platforms must be an array

#### Scenario: Array build-platforms gets no error annotation
- **WHEN** a PipelineRun has param `build-platforms` with type `array`
- **THEN** the PipelineRun SHALL NOT have a `tekton-kueue.konflux-ci.dev/validation-error` annotation

#### Scenario: Absent build-platforms gets no error annotation
- **WHEN** a PipelineRun does not have a `build-platforms` param
- **THEN** the PipelineRun SHALL NOT have a `tekton-kueue.konflux-ci.dev/validation-error` annotation

### Requirement: ValidatingAdmissionPolicy denies annotated PipelineRuns
A Kubernetes ValidatingAdmissionPolicy SHALL deny PipelineRun creation when the `tekton-kueue.konflux-ci.dev/validation-error` annotation is present. This policy is temporary and SHALL be removed after the no-skip-kueue rollout completes.

#### Scenario: PipelineRun with validation-error annotation is denied
- **WHEN** a PipelineRun has the `tekton-kueue.konflux-ci.dev/validation-error` annotation
- **THEN** the ValidatingAdmissionPolicy SHALL deny the request

#### Scenario: PipelineRun without validation-error annotation is allowed
- **WHEN** a PipelineRun does not have the `tekton-kueue.konflux-ci.dev/validation-error` annotation
- **THEN** the ValidatingAdmissionPolicy SHALL allow the request

### Requirement: Test coverage for string-typed build-platforms
The test script `hack/test-tekton-kueue-config.py` SHALL include test cases for PipelineRuns with string-typed `build-platforms` params, verifying that the validation-error annotation is set and no resource requests are generated.

#### Scenario: Test validates string build-platforms produces error annotation
- **WHEN** the test submits a PipelineRun with `build-platforms` as a string param
- **THEN** the expected results SHALL include the `tekton-kueue.konflux-ci.dev/validation-error` annotation
- **AND** the expected results SHALL NOT include any `kueue.konflux-ci.dev/requests-*` annotations for platforms
