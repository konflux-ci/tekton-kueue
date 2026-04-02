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
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	v1 "github.com/konflux-ci/tekton-kueue/internal/webhook/v1"
	"github.com/konflux-ci/tekton-kueue/pkg/common"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

var _ = Describe("ConfigMapReconciler", func() {
	var (
		reconciler *ConfigMapReconciler
		store      *v1.ConfigStore
		s          *runtime.Scheme
		nsName     types.NamespacedName
	)

	BeforeEach(func() {
		s = runtime.NewScheme()
		Expect(clientgoscheme.AddToScheme(s)).To(Succeed())

		store = &v1.ConfigStore{}
		nsName = types.NamespacedName{
			Name:      common.ConfigMapName,
			Namespace: "tekton-kueue",
		}
	})

	Describe("Reconcile", func() {
		It("should return success when ConfigMap is not found", func(ctx context.Context) {
			fakeClient := fake.NewClientBuilder().
				WithScheme(s).
				WithInterceptorFuncs(interceptor.Funcs{
					Get: func(_ context.Context, _ client.WithWatch, key client.ObjectKey, _ client.Object, _ ...client.GetOption) error {
						Expect(key).To(Equal(nsName))
						return apierrors.NewNotFound(schema.GroupResource{Resource: "configmaps"}, key.Name)
					},
				}).
				Build()
			reconciler = &ConfigMapReconciler{Client: fakeClient, Store: store}

			Expect(reconciler.Reconcile(ctx, ctrl.Request{NamespacedName: nsName})).To(Equal(ctrl.Result{}))
		})

		It("should return error when Client.Get fails with a non-NotFound error", func(ctx context.Context) {
			getErr := fmt.Errorf("connection refused")
			fakeClient := fake.NewClientBuilder().
				WithScheme(s).
				WithInterceptorFuncs(interceptor.Funcs{
					Get: func(_ context.Context, _ client.WithWatch, key client.ObjectKey, _ client.Object, _ ...client.GetOption) error {
						Expect(key).To(Equal(nsName))
						return getErr
					},
				}).
				Build()
			reconciler = &ConfigMapReconciler{Client: fakeClient, Store: store}

			_, err := reconciler.Reconcile(ctx, ctrl.Request{NamespacedName: nsName})
			Expect(err).To(MatchError("connection refused"))
		})

		It("should return success when ConfigMap exists but config key is missing", func(ctx context.Context) {
			cm := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      common.ConfigMapName,
					Namespace: "tekton-kueue",
				},
				Data: map[string]string{
					"other-key": "some-value",
				},
			}
			fakeClient := fake.NewClientBuilder().WithScheme(s).WithObjects(cm).Build()
			reconciler = &ConfigMapReconciler{Client: fakeClient, Store: store}

			Expect(reconciler.Reconcile(ctx, ctrl.Request{NamespacedName: nsName})).To(Equal(ctrl.Result{}))
		})

		It("should requeue when config YAML is invalid", func(ctx context.Context) {
			cm := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      common.ConfigMapName,
					Namespace: "tekton-kueue",
				},
				Data: map[string]string{
					common.ConfigKey: "invalid: yaml: [:",
				},
			}
			fakeClient := fake.NewClientBuilder().WithScheme(s).WithObjects(cm).Build()
			reconciler = &ConfigMapReconciler{Client: fakeClient, Store: store}

			result, err := reconciler.Reconcile(ctx, ctrl.Request{NamespacedName: nsName})
			Expect(err).To(HaveOccurred())
			Expect(result.RequeueAfter).To(Equal(10 * time.Second))
		})

		It("should requeue when config validation fails (empty queue name)", func(ctx context.Context) {
			cm := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      common.ConfigMapName,
					Namespace: "tekton-kueue",
				},
				Data: map[string]string{
					common.ConfigKey: "multiKueueOverride: true",
				},
			}
			fakeClient := fake.NewClientBuilder().WithScheme(s).WithObjects(cm).Build()
			reconciler = &ConfigMapReconciler{Client: fakeClient, Store: store}

			result, err := reconciler.Reconcile(ctx, ctrl.Request{NamespacedName: nsName})
			Expect(err).To(MatchError(ContainSubstring("queue name")))
			Expect(result.RequeueAfter).To(Equal(10 * time.Second))
		})

		It("should update the store successfully with valid config", func(ctx context.Context) {
			cm := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      common.ConfigMapName,
					Namespace: "tekton-kueue",
				},
				Data: map[string]string{
					common.ConfigKey: "queueName: test-queue\nmultiKueueOverride: false",
				},
			}
			fakeClient := fake.NewClientBuilder().WithScheme(s).WithObjects(cm).Build()
			reconciler = &ConfigMapReconciler{Client: fakeClient, Store: store}

			Expect(reconciler.Reconcile(ctx, ctrl.Request{NamespacedName: nsName})).To(Equal(ctrl.Result{}))

			cfg, mutators := store.GetConfigAndMutators()
			Expect(cfg).NotTo(BeNil())
			Expect(cfg.QueueName).To(Equal("test-queue"))
			Expect(cfg.MultiKueueOverride).To(BeFalse())
			Expect(mutators).To(BeEmpty())
		})
	})
})
