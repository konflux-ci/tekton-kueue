package controller

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"

	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kueue "sigs.k8s.io/kueue/apis/kueue/v1beta1"
	"sigs.k8s.io/kueue/pkg/controller/jobframework"
	"sigs.k8s.io/kueue/pkg/podset"

	tekv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	corev1 "k8s.io/api/core/v1"

	"k8s.io/apimachinery/pkg/api/resource"
	kapi "knative.dev/pkg/apis"

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"

	kueueconfig "sigs.k8s.io/kueue/apis/config/v1beta1"
)

// +kubebuilder:rbac:groups=scheduling.k8s.io,resources=priorityclasses,verbs=list;get;watch
// +kubebuilder:rbac:groups="",resources=events,verbs=create;watch;update;patch
// +kubebuilder:rbac:groups=kueue.x-k8s.io,resources=workloads,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=kueue.x-k8s.io,resources=workloads/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=kueue.x-k8s.io,resources=workloads/finalizers,verbs=update
// +kubebuilder:rbac:groups=kueue.x-k8s.io,resources=resourceflavors,verbs=get;list;watch
// +kubebuilder:rbac:groups=kueue.x-k8s.io,resources=workloadpriorityclasses,verbs=get;list;watch
// +kubebuilder:rbac:groups="tekton.dev",resources=pipelineruns,verbs=watch;update;patch

type PipelineRun tekv1.PipelineRun

const (
	ConditionTypeTerminationTarget = "TerminationTarget"
)

const (
	ControllerName = "KueuePipelineRunController"
)

var (
	_      jobframework.GenericJob        = &PipelineRun{}
	_      jobframework.JobWithCustomStop = &PipelineRun{}
	PLRGVK                                = tekv1.SchemeGroupVersion.WithKind("PipelineRun")
	PLRLog                                = ctrl.Log.WithName(ControllerName)
)

func SetupWithManager(mgr ctrl.Manager) error {
	workloadReconciler := jobframework.NewGenericReconcilerFactory(
		func() jobframework.GenericJob { return &PipelineRun{} },
		func(b *builder.Builder, c client.Client) *builder.Builder {
			return b.Named("PipelineRunWorkloads")
		},
	)

	selector := labels.NewSelector()
	req1, err := labels.NewRequirement("konflux.ci/type", selection.In, []string{"user"})
	if err != nil {
		PLRLog.Error(err, "unable to create namespace label selector")
		return err
	}
	selector = selector.Add(*req1)

	return workloadReconciler(
		mgr.GetClient(),
		mgr.GetEventRecorderFor("kueue-plr"),
		jobframework.WithManageJobsWithoutQueueName(true),
		jobframework.WithManagedJobsNamespaceSelector(selector),
		jobframework.WithWaitForPodsReady(&kueueconfig.WaitForPodsReady{}),
	).SetupWithManager(mgr)
}

func SetupIndexer(ctx context.Context, fieldIndexer client.FieldIndexer) error {
	return jobframework.SetupWorkloadOwnerIndex(ctx, fieldIndexer, tekv1.SchemeGroupVersion.WithKind("PipelineRun"))
}

// Stop implements jobframework.JobWithCustomStop.
func (p *PipelineRun) Stop(ctx context.Context, c client.Client, _ []podset.PodSetInfo, stopReason jobframework.StopReason, eventMsg string) (bool, error) {
	plr := (*tekv1.PipelineRun)(p)
	plrPendingOrRunning := (plr.Spec.Status == "") || (plr.Spec.Status == tekv1.PipelineRunSpecStatusPending)

	if plr.IsDone() || !plrPendingOrRunning {
		return false, nil
	}

	plrCopy := plr.DeepCopy()
	plrCopy.SetManagedFields(nil)
	// should we wait for the pipeline to stop?
	plrCopy.Spec.Status = tekv1.PipelineRunSpecStatusStoppedRunFinally
	err := c.Patch(ctx, plrCopy, client.Apply, client.FieldOwner(ControllerName), client.ForceOwnership)
	if err != nil {
		return false, err
	}

	return true, nil
}

// Finished implements jobframework.GenericJob.
func (p *PipelineRun) Finished() (message string, success bool, finished bool) {
	plr := (*tekv1.PipelineRun)(p)
	condition := plr.Status.GetCondition(kapi.ConditionSucceeded)

	if condition == nil {
		return "", false, false
	}

	message = condition.Message
	success = (condition.Reason == tekv1.PipelineRunReasonSuccessful.String()) ||
		(condition.Reason == tekv1.PipelineRunReasonCompleted.String())
	finished = plr.IsDone()

	return

}

// GVK implements jobframework.GenericJob.
func (p *PipelineRun) GVK() schema.GroupVersionKind {
	return PLRGVK
}

// IsActive implements jobframework.GenericJob.
func (p *PipelineRun) IsActive() bool {
	return (*tekv1.PipelineRun)(p).HasStarted()
}

// IsSuspended implements jobframework.GenericJob.
func (p *PipelineRun) IsSuspended() bool {
	return p.Spec.Status == tekv1.PipelineRunSpecStatusPending
}

// Object implements jobframework.GenericJob.
func (p *PipelineRun) Object() client.Object {
	return (*tekv1.PipelineRun)(p)
}

// PodSets implements jobframework.GenericJob.
func (p *PipelineRun) PodSets() []kueue.PodSet {
	return []kueue.PodSet{
		{
			Name: "pod-set-1",
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "dummy",
							Image: "dummy",
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceMemory: resource.MustParse("1Gi"),
								},
							},
						},
					},
				},
			},
			Count: 1,
		},
	}
}

// PodsReady implements jobframework.GenericJob.
func (p *PipelineRun) PodsReady() bool {
	panic("pods ready shouldn't be called")
}

// RestorePodSetsInfo implements jobframework.GenericJob.
func (p *PipelineRun) RestorePodSetsInfo(podSetsInfo []podset.PodSetInfo) bool {
	return false
}

// RunWithPodSetsInfo implements jobframework.GenericJob.
func (p *PipelineRun) RunWithPodSetsInfo(podSetsInfo []podset.PodSetInfo) error {
	p.Spec.Status = ""
	return nil
}

// Suspend implements jobframework.GenericJob.
func (p *PipelineRun) Suspend() {
	// Not implemented because this is not called when JobWithCustomStop is implemented.
}
