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

	"github.com/go-logr/logr"
	"github.com/konflux-ci/tekton-kueue/pkg/common"
	tekv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// SetupPipelineRunWebhookWithManager registers the webhook for PipelineRun in the manager.
func SetupPipelineRunWebhookWithManager(mgr ctrl.Manager, defaulter admission.CustomDefaulter) error {
	return ctrl.NewWebhookManagedBy(mgr).For(&tekv1.PipelineRun{}).
		WithDefaulter(defaulter).
		WithLogConstructor(logConstructor).
		Complete()
}

func logConstructor(base logr.Logger, req *admission.Request) logr.Logger {
	gvk := (&tekv1.PipelineRun{}).GetGroupVersionKind()
	log := base.WithValues(
		"webhookGroup", gvk.Group,
		"webhookKind", gvk.Kind,
	)
	if req != nil {
		log = log.WithValues(
			"webhookGroup", tekv1.SchemeGroupVersion.Group,
			"webhookKind", gvk.Kind,
			gvk.Kind, klog.KRef(req.Namespace, req.Name),
			"namespace", req.Namespace,
			"name", req.Name,
			"resource", req.Resource,
			"user", req.UserInfo.Username,
			"requestID", req.UID,
		)

		if a, err := meta.Accessor(req.Object); err == nil {
			if a.GetName() == "" {
				// add the generate name only if the name is unset
				return log.WithValues("generateName", a.GetGenerateName())
			}
		}
	}
	return log
}

type PipelineRunMutator interface {
	Mutate(*tekv1.PipelineRun) error
}

// TODO(user): EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!

// +kubebuilder:webhook:path=/mutate-tekton-dev-v1-pipelinerun,mutating=true,failurePolicy=fail,sideEffects=None,groups=tekton.dev,resources=pipelineruns,verbs=create,versions=v1,name=pipelinerun-kueue-defaulter.tekton-kueue.io,admissionReviewVersions=v1

// PipelineRunCustomDefaulter struct is responsible for setting default values on the custom resource of the
// Kind PipelineRun when those are created or updated.
//
// NOTE: The +kubebuilder:object:generate=false marker prevents controller-gen from generating DeepCopy methods,
// as it is used only for temporary operations and does not need to be deeply copied.
type pipelineRunCustomDefaulter struct {
	configStore *ConfigStore
}

func NewCustomDefaulter(configStore *ConfigStore) (webhook.CustomDefaulter, error) {
	defaulter := &pipelineRunCustomDefaulter{
		configStore: configStore,
	}
	return defaulter, nil
}

// Default implements webhook.CustomDefaulter so a webhook will be registered for the Kind PipelineRun.
func (d *pipelineRunCustomDefaulter) Default(ctx context.Context, obj runtime.Object) error {
	plr, ok := obj.(*tekv1.PipelineRun)

	if !ok {
		return k8serrors.NewBadRequest(fmt.Sprintf("expected an PipelineRun object but got %T", obj))
	}

	// Attempt to catch bad pipelineruns prior to processing so we can catch
	// errors ourselves and handle them appropriately.  Only validate the spec
	// field, since we might be getting a pipelinerun with a generated name, which
	// the top-level Validate() method will reject
	err := plr.Spec.Validate(ctx)
	if err != nil {
		return k8serrors.NewBadRequest(err.Error())
	}

	plr.Spec.Status = tekv1.PipelineRunSpecStatusPending
	if plr.Labels == nil {
		plr.Labels = make(map[string]string)
	}
	config := d.configStore.GetConfig()
	if label, exists := plr.Labels[common.QueueLabel]; !exists || label == "" {
		plr.Labels[common.QueueLabel] = config.QueueName
	}
	if config.MultiClusterEnabled && config.MultiClusterRole == common.MultiClusterRoleHub {
		plr.Spec.ManagedBy = ptr.To(common.ManagedByMultiKueueLabel)
	}
	for _, mutator := range d.configStore.GetMutators() {
		if err := mutator.Mutate(plr); err != nil {
			return err
		}
	}

	return nil
}
