/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/konflux-ci/tekton-kueue/internal/cel"
	pkgconfig "github.com/konflux-ci/tekton-kueue/pkg/config"
	"github.com/konflux-ci/tekton-kueue/pkg/common"
	tekv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"gomodules.xyz/jsonpatch/v2"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// +kubebuilder:webhook:path=/mutate-tekton-dev-v1-pipelinerun,mutating=true,failurePolicy=fail,sideEffects=None,groups=tekton.dev,resources=pipelineruns,verbs=create,versions=v1,name=pipelinerun-kueue-defaulter.tekton-kueue.io,admissionReviewVersions=v1

// PipelineRunWebhookHandler handles PipelineRun admission requests using
// explicit JSON patches. This avoids controller-runtime's CustomDefaulter
// pattern which serializes the full Go struct and can leak zero-value fields
// into the patch, interfering with downstream webhook defaulting.
//
// See: https://github.com/tektoncd/pipeline/issues/9647
type PipelineRunWebhookHandler struct {
	configStore *ConfigStore
	decoder     admission.Decoder
}

func NewWebhookHandler(configStore *ConfigStore, decoder admission.Decoder) *PipelineRunWebhookHandler {
	return &PipelineRunWebhookHandler{
		configStore: configStore,
		decoder:     decoder,
	}
}

// Handle builds explicit JSON patches for only the fields we intend to modify.
// No struct round-tripping means no zero-value field leaks.
func (h *PipelineRunWebhookHandler) Handle(ctx context.Context, req admission.Request) admission.Response {
	plr := &tekv1.PipelineRun{}
	if err := h.decoder.Decode(req, plr); err != nil {
		return admission.Errored(http.StatusBadRequest, fmt.Errorf("expected a PipelineRun object: %w", err))
	}

	config, mutators := h.configStore.GetConfigAndMutators()

	var patches []jsonpatch.JsonPatchOperation

	// 1. Set spec.status = PipelineRunPending
	patches = append(patches, jsonpatch.JsonPatchOperation{
		Operation: "add",
		Path:      "/spec/status",
		Value:     string(tekv1.PipelineRunSpecStatusPending),
	})

	// 2. Add queue label
	if plr.Labels == nil {
		patches = append(patches, jsonpatch.JsonPatchOperation{
			Operation: "add",
			Path:      "/metadata/labels",
			Value:     map[string]string{common.QueueLabel: config.QueueName},
		})
	} else if _, exists := plr.Labels[common.QueueLabel]; !exists {
		patches = append(patches, jsonpatch.JsonPatchOperation{
			Operation: "add",
			Path:      "/metadata/labels/" + escapeJSONPointer(common.QueueLabel),
			Value:     config.QueueName,
		})
	}

	// 3. Set managedBy if multiKueueOverride is enabled
	if config.MultiKueueOverride {
		patches = append(patches, jsonpatch.JsonPatchOperation{
			Operation: "add",
			Path:      "/spec/managedBy",
			Value:     common.ManagedByMultiKueueLabel,
		})
	}

	// 4. Apply CEL mutations. CEL only modifies labels, annotations, and
	//    resource annotations — all metadata, no spec fields. We apply them
	//    to the decoded struct then diff only the metadata to build patches.
	if len(mutators) > 0 {
		plrForCEL := plr.DeepCopy()
		for _, mutator := range mutators {
			if err := mutator.Mutate(plrForCEL); err != nil {
				var validationErr *cel.ValidationError
				if errors.As(err, &validationErr) {
					return admission.Errored(http.StatusBadRequest, validationErr)
				}
				var evaluationErr *cel.EvaluationError
				if errors.As(err, &evaluationErr) {
					return admission.Errored(http.StatusInternalServerError, evaluationErr)
				}
				return admission.Errored(http.StatusInternalServerError, err)
			}
		}
		celPatches := metadataDiffPatches(plr, plrForCEL)
		patches = append(patches, celPatches...)
	}

	return admission.Patched("kueue defaults applied", patches...)
}

// metadataDiffPatches computes JSON patches for label and annotation changes
// between original and mutated PipelineRuns. Only metadata is compared —
// spec fields are excluded to prevent zero-value leaks.
func metadataDiffPatches(original, mutated *tekv1.PipelineRun) []jsonpatch.JsonPatchOperation {
	var patches []jsonpatch.JsonPatchOperation

	// Diff labels: additions and modifications
	if mutated.Labels != nil {
		if original.Labels == nil {
			patches = append(patches, jsonpatch.JsonPatchOperation{
				Operation: "add",
				Path:      "/metadata/labels",
				Value:     mutated.Labels,
			})
		} else {
			for k, v := range mutated.Labels {
				if original.Labels[k] != v {
					patches = append(patches, jsonpatch.JsonPatchOperation{
						Operation: "add",
						Path:      "/metadata/labels/" + escapeJSONPointer(k),
						Value:     v,
					})
				}
			}
			// Detect deletions
			for k := range original.Labels {
				if _, exists := mutated.Labels[k]; !exists {
					patches = append(patches, jsonpatch.JsonPatchOperation{
						Operation: "remove",
						Path:      "/metadata/labels/" + escapeJSONPointer(k),
					})
				}
			}
		}
	}

	// Diff annotations: additions and modifications
	if mutated.Annotations != nil {
		if original.Annotations == nil {
			patches = append(patches, jsonpatch.JsonPatchOperation{
				Operation: "add",
				Path:      "/metadata/annotations",
				Value:     mutated.Annotations,
			})
		} else {
			for k, v := range mutated.Annotations {
				if original.Annotations[k] != v {
					patches = append(patches, jsonpatch.JsonPatchOperation{
						Operation: "add",
						Path:      "/metadata/annotations/" + escapeJSONPointer(k),
						Value:     v,
					})
				}
			}
			// Detect deletions
			for k := range original.Annotations {
				if _, exists := mutated.Annotations[k]; !exists {
					patches = append(patches, jsonpatch.JsonPatchOperation{
						Operation: "remove",
						Path:      "/metadata/annotations/" + escapeJSONPointer(k),
					})
				}
			}
		}
	}

	return patches
}

// escapeJSONPointer escapes special characters per RFC 6901.
func escapeJSONPointer(s string) string {
	s = strings.ReplaceAll(s, "~", "~0")
	s = strings.ReplaceAll(s, "/", "~1")
	return s
}

// SetupPipelineRunWebhookWithManager registers the webhook handler directly
// on the webhook server, bypassing controller-runtime's CustomDefaulter
// struct round-trip pattern.
func SetupPipelineRunWebhookWithManager(mgr ctrl.Manager, handler *PipelineRunWebhookHandler) error {
	srv := mgr.GetWebhookServer()
	srv.Register("/mutate-tekton-dev-v1-pipelinerun", &admission.Webhook{Handler: handler})
	return nil
}

// ApplyMutations applies all kueue mutations to a PipelineRun in-place.
// Used by both the legacy CustomDefaulter (for the mutate CLI) and can be
// used for testing. This is the single source of truth for mutation logic.
func ApplyMutations(plr *tekv1.PipelineRun, cfg *pkgconfig.Config, mutators []PipelineRunMutator) error {
	plr.Spec.Status = tekv1.PipelineRunSpecStatusPending
	if plr.Labels == nil {
		plr.Labels = make(map[string]string)
	}
	if _, exists := plr.Labels[common.QueueLabel]; !exists {
		plr.Labels[common.QueueLabel] = cfg.QueueName
	}
	if cfg.MultiKueueOverride {
		plr.Spec.ManagedBy = ptr.To(common.ManagedByMultiKueueLabel)
	}
	for _, mutator := range mutators {
		if err := mutator.Mutate(plr); err != nil {
			return err
		}
	}
	return nil
}

// pipelineRunCustomDefaulter implements webhook.CustomDefaulter for backward
// compatibility with the mutate CLI subcommand. The CLI operates on local
// files (no admission webhook path), so the zero-value leak doesn't apply.
type pipelineRunCustomDefaulter struct {
	configStore *ConfigStore
}

func NewCustomDefaulter(configStore *ConfigStore) (webhook.CustomDefaulter, error) {
	return &pipelineRunCustomDefaulter{configStore: configStore}, nil
}

// Default implements webhook.CustomDefaulter.
func (d *pipelineRunCustomDefaulter) Default(ctx context.Context, obj runtime.Object) error {
	plr, ok := obj.(*tekv1.PipelineRun)
	if !ok {
		return k8serrors.NewBadRequest(fmt.Sprintf("expected a PipelineRun object but got %T", obj))
	}

	cfg, mutators := d.configStore.GetConfigAndMutators()
	if err := ApplyMutations(plr, cfg, mutators); err != nil {
		var validationErr *cel.ValidationError
		if errors.As(err, &validationErr) {
			return k8serrors.NewBadRequest(validationErr.Error())
		}
		var evaluationErr *cel.EvaluationError
		if errors.As(err, &evaluationErr) {
			return k8serrors.NewInternalError(evaluationErr)
		}
		return err
	}
	return nil
}
