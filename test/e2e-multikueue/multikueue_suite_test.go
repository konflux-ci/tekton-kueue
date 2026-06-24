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

package e2e_multikueue

import (
	"context"
	"fmt"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	tekv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	kueue "sigs.k8s.io/kueue/apis/kueue/v1beta1"
	"sigs.k8s.io/kueue/pkg/controller/jobframework"
)

var hubClient client.Client
var spokeClient client.Client

const spokeContextName = "kind-spoke-1"
const hubContextName = "kind-hub"

// TestE2E runs the end-to-end (e2e) test suite for the project. These tests execute in an isolated,
// temporary environment to validate project changes with the purpose to be used in CI jobs.
// The default setup installs CertManager and Prometheus.
// The IMG environment varialbe must be specified with the image that should be used by the controller's deployment
func TestE2E(t *testing.T) {
	RegisterFailHandler(Fail)
	_, _ = fmt.Fprintf(GinkgoWriter, "Starting tekton-kueue multikueue integration test suite\n")
	RunSpecs(t, "Multikueue e2e suite")
}

var _ = BeforeSuite(func() {

	By("Setup Kube ClientSets", func() {
		ctx := context.Background()
		hubClient = getK8sClientOrDie(ctx, hubContextName)
		spokeClient = getK8sClientOrDie(ctx, spokeContextName)
	})
})

func getK8sClientOrDie(ctx context.Context, contextName string) client.Client {
	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(tekv1.AddToScheme(scheme))
	utilruntime.Must(kueue.AddToScheme(scheme))

	// 1. Define standard loading rules (finds ~/.kube/config or KUBECONFIG env var)
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()

	// 2. Override the current context with the one provided
	configOverrides := &clientcmd.ConfigOverrides{
		CurrentContext: contextName,
	}

	// 3. Build the specific REST config
	kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)
	cfg, err := kubeConfig.ClientConfig()
	if err != nil {
		panic("failed to create config for context " + contextName + ": " + err.Error())
	}

	k8sCache, err := cache.New(cfg, cache.Options{Scheme: scheme, ReaderFailOnMissingInformer: true})
	Expect(err).ToNot(HaveOccurred(), "failed to create cache")

	_, err = k8sCache.GetInformer(ctx, &kueue.Workload{})
	Expect(err).ToNot(HaveOccurred(), "failed to setup informer for workloads")

	_, err = k8sCache.GetInformer(ctx, &tekv1.PipelineRun{})
	Expect(err).ToNot(HaveOccurred(), "failed to setup informer for pipelineruns")

	Expect(jobframework.SetupWorkloadOwnerIndex(
		ctx,
		k8sCache,
		tekv1.SchemeGroupVersion.WithKind("PipelineRun"),
	)).To(Succeed(), "failed to setup indexer")

	go func() {
		if err := k8sCache.Start(ctx); err != nil {
			panic(err)
		}
	}()

	if synced := k8sCache.WaitForCacheSync(ctx); !synced {
		panic("failed waiting for cache to sync")
	}

	k8sClient, err := client.New(
		cfg,
		client.Options{
			Cache:  &client.CacheOptions{Reader: k8sCache},
			Scheme: scheme,
		},
	)
	Expect(err).ToNot(HaveOccurred(), "failed to create client")

	return k8sClient
}
