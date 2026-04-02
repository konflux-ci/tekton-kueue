/*
Copyright 2026.

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

package controller

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	tekv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kapi "knative.dev/pkg/apis"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	"sigs.k8s.io/kueue/pkg/controller/jobframework"
	"sigs.k8s.io/kueue/pkg/podset"
)

func newTestPipelineRun(opts ...func(*tekv1.PipelineRun)) *PipelineRun {
	plr := &tekv1.PipelineRun{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "tekton.dev/v1",
			Kind:       "PipelineRun",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-plr",
			Namespace: "default",
		},
	}
	for _, o := range opts {
		o(plr)
	}
	return (*PipelineRun)(plr)
}

var _ = Describe("PipelineRun", func() {

	Describe("GVK", func() {
		It("should return the PipelineRun GroupVersionKind", func() {
			p := newTestPipelineRun()
			gvk := p.GVK()
			Expect(gvk.Group).To(Equal("tekton.dev"))
			Expect(gvk.Version).To(Equal("v1"))
			Expect(gvk.Kind).To(Equal("PipelineRun"))
		})
	})

	Describe("IsActive", func() {
		It("should return false when PipelineRun has not started", func() {
			p := newTestPipelineRun()
			Expect(p.IsActive()).To(BeFalse())
		})

		It("should return true when PipelineRun has started", func() {
			now := metav1.Now()
			p := newTestPipelineRun(func(plr *tekv1.PipelineRun) {
				plr.Status.StartTime = &now
			})
			Expect(p.IsActive()).To(BeTrue())
		})
	})

	Describe("IsSuspended", func() {
		It("should return false when status is empty", func() {
			p := newTestPipelineRun()
			Expect(p.IsSuspended()).To(BeFalse())
		})

		It("should return true when status is Pending", func() {
			p := newTestPipelineRun(func(plr *tekv1.PipelineRun) {
				plr.Spec.Status = tekv1.PipelineRunSpecStatusPending
			})
			Expect(p.IsSuspended()).To(BeTrue())
		})

		It("should return false when status is StoppedRunFinally", func() {
			p := newTestPipelineRun(func(plr *tekv1.PipelineRun) {
				plr.Spec.Status = tekv1.PipelineRunSpecStatusStoppedRunFinally
			})
			Expect(p.IsSuspended()).To(BeFalse())
		})
	})

	Describe("Object", func() {
		It("should return the underlying PipelineRun as client.Object", func() {
			p := newTestPipelineRun()
			obj := p.Object()
			Expect(obj).NotTo(BeNil())
			Expect(obj.GetName()).To(Equal("test-plr"))
			Expect(obj.GetNamespace()).To(Equal("default"))
		})
	})

	Describe("PodsReady", func() {
		It("should panic because it should not be called", func() {
			p := newTestPipelineRun()
			Expect(func() { p.PodsReady() }).To(PanicWith("pods ready shouldn't be called"))
		})
	})

	Describe("RestorePodSetsInfo", func() {
		It("should return false with nil input", func() {
			p := newTestPipelineRun()
			Expect(p.RestorePodSetsInfo(nil)).To(BeFalse())
		})

		It("should return false with non-empty input", func() {
			p := newTestPipelineRun()
			Expect(p.RestorePodSetsInfo([]podset.PodSetInfo{{}})).To(BeFalse())
		})
	})

	Describe("RunWithPodSetsInfo", func() {
		It("should clear Spec.Status when it was Pending", func() {
			p := newTestPipelineRun(func(plr *tekv1.PipelineRun) {
				plr.Spec.Status = tekv1.PipelineRunSpecStatusPending
			})
			err := p.RunWithPodSetsInfo(nil)
			Expect(err).NotTo(HaveOccurred())
			Expect(p.Spec.Status).To(BeEmpty())
		})

		It("should clear Spec.Status when it was StoppedRunFinally", func() {
			p := newTestPipelineRun(func(plr *tekv1.PipelineRun) {
				plr.Spec.Status = tekv1.PipelineRunSpecStatusStoppedRunFinally
			})
			err := p.RunWithPodSetsInfo(nil)
			Expect(err).NotTo(HaveOccurred())
			Expect(p.Spec.Status).To(BeEmpty())
		})
	})

	Describe("Suspend", func() {
		It("should not change the PipelineRun state (no-op)", func() {
			p := newTestPipelineRun(func(plr *tekv1.PipelineRun) {
				plr.Spec.Status = ""
			})
			p.Suspend()
			// Suspend is a no-op because JobWithCustomStop is implemented
			Expect(p.Spec.Status).To(BeEmpty())
		})
	})

	Describe("Finished", func() {
		It("should return empty values when no condition is set", func() {
			p := newTestPipelineRun()
			msg, success, finished := p.Finished()
			Expect(msg).To(BeEmpty())
			Expect(success).To(BeFalse())
			Expect(finished).To(BeFalse())
		})

		It("should report success when reason is Successful", func() {
			p := newTestPipelineRun(func(plr *tekv1.PipelineRun) {
				plr.Status.Conditions = []kapi.Condition{
					{
						Type:    kapi.ConditionSucceeded,
						Status:  corev1.ConditionTrue,
						Reason:  tekv1.PipelineRunReasonSuccessful.String(),
						Message: "All tasks completed successfully",
					},
				}
			})
			msg, success, finished := p.Finished()
			Expect(msg).To(Equal("All tasks completed successfully"))
			Expect(success).To(BeTrue())
			Expect(finished).To(BeTrue())
		})

		It("should report success when reason is Completed", func() {
			p := newTestPipelineRun(func(plr *tekv1.PipelineRun) {
				plr.Status.Conditions = []kapi.Condition{
					{
						Type:    kapi.ConditionSucceeded,
						Status:  corev1.ConditionTrue,
						Reason:  tekv1.PipelineRunReasonCompleted.String(),
						Message: "Pipeline completed",
					},
				}
			})
			msg, success, finished := p.Finished()
			Expect(msg).To(Equal("Pipeline completed"))
			Expect(success).To(BeTrue())
			Expect(finished).To(BeTrue())
		})

		It("should report failure when PipelineRun has failed", func() {
			p := newTestPipelineRun(func(plr *tekv1.PipelineRun) {
				plr.Status.Conditions = []kapi.Condition{
					{
						Type:    kapi.ConditionSucceeded,
						Status:  corev1.ConditionFalse,
						Reason:  "Failed",
						Message: "Task my-task failed",
					},
				}
			})
			msg, success, finished := p.Finished()
			Expect(msg).To(Equal("Task my-task failed"))
			Expect(success).To(BeFalse())
			Expect(finished).To(BeTrue())
		})

		It("should report not finished when condition status is Unknown", func() {
			p := newTestPipelineRun(func(plr *tekv1.PipelineRun) {
				plr.Status.Conditions = []kapi.Condition{
					{
						Type:    kapi.ConditionSucceeded,
						Status:  corev1.ConditionUnknown,
						Reason:  "Running",
						Message: "Tasks are still running",
					},
				}
			})
			msg, success, finished := p.Finished()
			Expect(msg).To(Equal("Tasks are still running"))
			Expect(success).To(BeFalse())
			Expect(finished).To(BeFalse())
		})
	})

	Describe("resourcesRequests", func() {
		It("should return only the default pipelinerun count when no annotations are set", func() {
			p := newTestPipelineRun()
			requests := p.resourcesRequests()
			Expect(requests).To(HaveLen(1))
			Expect(requests).To(HaveKeyWithValue(
				corev1.ResourceName(ResourcePipelineRunCount),
				resource.MustParse("1"),
			))
		})

		It("should ignore annotations that do not match the resource requests prefix", func() {
			p := newTestPipelineRun(func(plr *tekv1.PipelineRun) {
				plr.Annotations = map[string]string{
					"some-other-annotation":      "value",
					"kueue.konflux-ci.dev/other":  "value",
					"kueue.konflux-ci.dev/config": "value",
				}
			})
			requests := p.resourcesRequests()
			Expect(requests).To(HaveLen(1))
			Expect(requests).To(HaveKey(corev1.ResourceName(ResourcePipelineRunCount)))
		})

		It("should parse cpu and memory resource annotations", func() {
			p := newTestPipelineRun(func(plr *tekv1.PipelineRun) {
				plr.Annotations = map[string]string{
					"kueue.konflux-ci.dev/requests-cpu":    "500m",
					"kueue.konflux-ci.dev/requests-memory": "256Mi",
				}
			})
			requests := p.resourcesRequests()
			Expect(requests).To(HaveLen(3))
			Expect(requests).To(HaveKeyWithValue(corev1.ResourceName("cpu"), resource.MustParse("500m")))
			Expect(requests).To(HaveKeyWithValue(corev1.ResourceName("memory"), resource.MustParse("256Mi")))
			Expect(requests).To(HaveKeyWithValue(
				corev1.ResourceName(ResourcePipelineRunCount),
				resource.MustParse("1"),
			))
		})

		It("should parse ephemeral-storage and storage annotations", func() {
			p := newTestPipelineRun(func(plr *tekv1.PipelineRun) {
				plr.Annotations = map[string]string{
					"kueue.konflux-ci.dev/requests-ephemeral-storage": "10Gi",
					"kueue.konflux-ci.dev/requests-storage":           "50Gi",
				}
			})
			requests := p.resourcesRequests()
			Expect(requests).To(HaveLen(3))
			Expect(requests).To(HaveKeyWithValue(corev1.ResourceName("ephemeral-storage"), resource.MustParse("10Gi")))
			Expect(requests).To(HaveKeyWithValue(corev1.ResourceName("storage"), resource.MustParse("50Gi")))
		})

		It("should include only matching annotations when mixed with unrelated ones", func() {
			p := newTestPipelineRun(func(plr *tekv1.PipelineRun) {
				plr.Annotations = map[string]string{
					"kueue.konflux-ci.dev/requests-cpu": "2",
					"unrelated/annotation":              "ignored",
					"kueue.konflux-ci.dev/queue-name":   "also-ignored",
				}
			})
			requests := p.resourcesRequests()
			Expect(requests).To(HaveLen(2))
			Expect(requests).To(HaveKeyWithValue(corev1.ResourceName("cpu"), resource.MustParse("2")))
			Expect(requests).To(HaveKeyWithValue(
				corev1.ResourceName(ResourcePipelineRunCount),
				resource.MustParse("1"),
			))
		})

		It("should panic when annotation value is not a valid resource.Quantity", func() {
			p := newTestPipelineRun(func(plr *tekv1.PipelineRun) {
				plr.Annotations = map[string]string{
					"kueue.konflux-ci.dev/requests-cpu": "not-a-quantity",
				}
			})
			Expect(func() { p.resourcesRequests() }).To(Panic())
		})
	})

	Describe("PodSets", func() {
		It("should return a single pod set with the correct structure", func() {
			p := newTestPipelineRun()
			podSets, err := p.PodSets()
			Expect(err).NotTo(HaveOccurred())
			Expect(podSets).To(HaveLen(1))

			ps := podSets[0]
			Expect(string(ps.Name)).To(Equal("pod-set-1"))
			Expect(ps.Count).To(Equal(int32(1)))

			containers := ps.Template.Spec.Containers
			Expect(containers).To(HaveLen(1))
			Expect(containers[0].Name).To(Equal("dummy"))
			Expect(containers[0].Image).To(Equal("dummy"))
		})

		It("should include resource requests from annotations in the pod set", func() {
			p := newTestPipelineRun(func(plr *tekv1.PipelineRun) {
				plr.Annotations = map[string]string{
					"kueue.konflux-ci.dev/requests-cpu":    "4",
					"kueue.konflux-ci.dev/requests-memory": "8Gi",
				}
			})
			podSets, err := p.PodSets()
			Expect(err).NotTo(HaveOccurred())

			requests := podSets[0].Template.Spec.Containers[0].Resources.Requests
			Expect(requests).To(HaveKeyWithValue(corev1.ResourceName("cpu"), resource.MustParse("4")))
			Expect(requests).To(HaveKeyWithValue(corev1.ResourceName("memory"), resource.MustParse("8Gi")))
			Expect(requests).To(HaveKeyWithValue(
				corev1.ResourceName(ResourcePipelineRunCount),
				resource.MustParse("1"),
			))
		})
	})

	Describe("Stop", func() {
		var (
			testCtx context.Context
			s       *runtime.Scheme
		)

		BeforeEach(func() {
			testCtx = context.Background()
			s = runtime.NewScheme()
			Expect(tekv1.AddToScheme(s)).To(Succeed())
		})

		It("should return false when PipelineRun is already done", func() {
			p := newTestPipelineRun(func(plr *tekv1.PipelineRun) {
				plr.Status.Conditions = []kapi.Condition{
					{
						Type:   kapi.ConditionSucceeded,
						Status: corev1.ConditionTrue,
					},
				}
			})

			fakeClient := fake.NewClientBuilder().WithScheme(s).Build()
			stopped, err := p.Stop(testCtx, fakeClient, nil, jobframework.StopReasonWorkloadEvicted, "evicted")
			Expect(err).NotTo(HaveOccurred())
			Expect(stopped).To(BeFalse())
		})

		It("should return false when PipelineRun is done with failure", func() {
			p := newTestPipelineRun(func(plr *tekv1.PipelineRun) {
				plr.Status.Conditions = []kapi.Condition{
					{
						Type:   kapi.ConditionSucceeded,
						Status: corev1.ConditionFalse,
					},
				}
			})

			fakeClient := fake.NewClientBuilder().WithScheme(s).Build()
			stopped, err := p.Stop(testCtx, fakeClient, nil, jobframework.StopReasonWorkloadEvicted, "evicted")
			Expect(err).NotTo(HaveOccurred())
			Expect(stopped).To(BeFalse())
		})

		It("should return false when PipelineRun status is not pending or running", func() {
			p := newTestPipelineRun(func(plr *tekv1.PipelineRun) {
				plr.Spec.Status = tekv1.PipelineRunSpecStatusStoppedRunFinally
			})

			fakeClient := fake.NewClientBuilder().WithScheme(s).Build()
			stopped, err := p.Stop(testCtx, fakeClient, nil, jobframework.StopReasonWorkloadEvicted, "evicted")
			Expect(err).NotTo(HaveOccurred())
			Expect(stopped).To(BeFalse())
		})

		It("should stop a running PipelineRun with empty status", func() {
			p := newTestPipelineRun(func(plr *tekv1.PipelineRun) {
				plr.Spec.Status = ""
			})
			tekPlr := (*tekv1.PipelineRun)(p)

			fakeClient := fake.NewClientBuilder().
				WithScheme(s).
				WithObjects(tekPlr).
				Build()

			stopped, err := p.Stop(testCtx, fakeClient, nil, jobframework.StopReasonWorkloadEvicted, "evicted")
			Expect(err).NotTo(HaveOccurred())
			Expect(stopped).To(BeTrue())

			var updated tekv1.PipelineRun
			Expect(fakeClient.Get(testCtx, client.ObjectKeyFromObject(tekPlr), &updated)).To(Succeed())
			Expect(string(updated.Spec.Status)).To(Equal(string(tekv1.PipelineRunSpecStatusStoppedRunFinally)))
		})

		It("should stop a pending PipelineRun", func() {
			p := newTestPipelineRun(func(plr *tekv1.PipelineRun) {
				plr.Spec.Status = tekv1.PipelineRunSpecStatusPending
			})
			tekPlr := (*tekv1.PipelineRun)(p)

			fakeClient := fake.NewClientBuilder().
				WithScheme(s).
				WithObjects(tekPlr).
				Build()

			stopped, err := p.Stop(testCtx, fakeClient, nil, jobframework.StopReasonWorkloadEvicted, "evicted")
			Expect(err).NotTo(HaveOccurred())
			Expect(stopped).To(BeTrue())

			var updated tekv1.PipelineRun
			Expect(fakeClient.Get(testCtx, client.ObjectKeyFromObject(tekPlr), &updated)).To(Succeed())
			Expect(string(updated.Spec.Status)).To(Equal(string(tekv1.PipelineRunSpecStatusStoppedRunFinally)))
		})

		It("should return error when the patch fails", func() {
			p := newTestPipelineRun(func(plr *tekv1.PipelineRun) {
				plr.Spec.Status = ""
			})
			tekPlr := (*tekv1.PipelineRun)(p)

			patchErr := fmt.Errorf("server unavailable")
			fakeClient := fake.NewClientBuilder().
				WithScheme(s).
				WithObjects(tekPlr).
				WithInterceptorFuncs(interceptor.Funcs{
					Patch: func(_ context.Context, _ client.WithWatch, _ client.Object, _ client.Patch, _ ...client.PatchOption) error {
						return patchErr
					},
				}).
				Build()

			stopped, err := p.Stop(testCtx, fakeClient, nil, jobframework.StopReasonWorkloadEvicted, "evicted")
			Expect(err).To(MatchError("server unavailable"))
			Expect(stopped).To(BeFalse())
		})
	})
})
