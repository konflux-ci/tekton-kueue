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
	"fmt"

	tekv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// nolint:unused
// log is for logging in this package.
var pipelinerunlog = logf.Log.WithName("pipelinerun-resource")

// SetupPipelineRunWebhookWithManager registers the webhook for PipelineRun in the manager.
func SetupPipelineRunWebhookWithManager(mgr ctrl.Manager, defaulter admission.CustomDefaulter) error {
	return ctrl.NewWebhookManagedBy(mgr).For(&tekv1.PipelineRun{}).
		WithDefaulter(defaulter).
		Complete()
}

// TODO(user): EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!

// +kubebuilder:webhook:path=/mutate-tekton-dev-v1-pipelinerun,mutating=true,failurePolicy=fail,sideEffects=None,groups=tekton.dev,resources=pipelineruns,verbs=create,versions=v1,name=mpipelinerun-v1.kb.io,admissionReviewVersions=v1

// PipelineRunCustomDefaulter struct is responsible for setting default values on the custom resource of the
// Kind PipelineRun when those are created or updated.
//
// NOTE: The +kubebuilder:object:generate=false marker prevents controller-gen from generating DeepCopy methods,
// as it is used only for temporary operations and does not need to be deeply copied.
type PipelineRunCustomDefaulter struct {
	KueueName string
}

var _ webhook.CustomDefaulter = &PipelineRunCustomDefaulter{}

const QueueLabel = "kueue.x-k8s.io/queue-name"

// Default implements webhook.CustomDefaulter so a webhook will be registered for the Kind PipelineRun.
func (d *PipelineRunCustomDefaulter) Default(ctx context.Context, obj runtime.Object) error {
	plr, ok := obj.(*tekv1.PipelineRun)

	if !ok {
		return fmt.Errorf("expected an PipelineRun object but got %T", obj)
	}
	pipelinerunlog.Info("Defaulting for PipelineRun", "name", plr.GetName())

	plr.Spec.Status = tekv1.PipelineRunSpecStatusPending
	if d.KueueName != "" {
		plr.Labels[QueueLabel] = d.KueueName
	}

	return nil
}
