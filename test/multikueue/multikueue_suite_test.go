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

package multikueue

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	tekton "github.com/tektoncd/pipeline/pkg/client/clientset/versioned"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
	kueue "sigs.k8s.io/kueue/client-go/clientset/versioned"
)

// TestE2E runs the end-to-end (e2e) test suite for the project. These tests execute in an isolated,
// temporary environment to validate project changes with the purpose to be used in CI jobs.
// The default setup installs CertManager and Prometheus.
// The IMG environment varialbe must be specified with the image that should be used by the controller's deployment
func TestE2E(t *testing.T) {
	RegisterFailHandler(Fail)
	_, _ = fmt.Fprintf(GinkgoWriter, "Starting tekton-kueue multikueue integration test suite\n")
	RunSpecs(t, "Multikueue e2e suite")
}

var (
	HubClientset       *kubernetes.Clientset
	HubTektonClientset *tekton.Clientset
	HubKueueClientset  *kueue.Clientset

	SpokeClientset       *kubernetes.Clientset
	SpokeKueueClientset  *kueue.Clientset
	SpokeTektonClientset *tekton.Clientset

	HubKubeContext   = "kind-hub"
	SpokeKubeContext = "kind-spoke-1"
)

var rawConfig *api.Config

var _ = BeforeSuite(func() {

	By("Setup Kube ClientSets", func() {
		kubeconfig := os.Getenv("KUBECONFIG")
		if kubeconfig == "" {
			home, _ := os.UserHomeDir()
			kubeconfig = filepath.Join(home, ".kube", "config")
		}

		kubeconfigBytes, err := os.ReadFile(kubeconfig)
		Expect(err).NotTo(HaveOccurred())

		rawConfig, err = clientcmd.Load(kubeconfigBytes)
		Expect(err).NotTo(HaveOccurred())

		hubConfig := clientcmd.
			NewNonInteractiveClientConfig(*rawConfig, HubKubeContext, &clientcmd.ConfigOverrides{}, nil)
		restConfig, err := hubConfig.ClientConfig()
		Expect(err).NotTo(HaveOccurred())

		Expect(err).NotTo(HaveOccurred())
		HubClientset, err = kubernetes.NewForConfig(restConfig)
		Expect(err).NotTo(HaveOccurred())

		HubTektonClientset, err = tekton.NewForConfig(restConfig)
		Expect(err).NotTo(HaveOccurred())

		HubKueueClientset, err = kueue.NewForConfig(restConfig)
		Expect(err).NotTo(HaveOccurred())

		spokeConfig := clientcmd.
			NewNonInteractiveClientConfig(*rawConfig, SpokeKubeContext, &clientcmd.ConfigOverrides{}, nil)
		restConfig, err = spokeConfig.ClientConfig()
		Expect(err).NotTo(HaveOccurred())

		Expect(err).NotTo(HaveOccurred())
		SpokeClientset, err = kubernetes.NewForConfig(restConfig)
		Expect(err).NotTo(HaveOccurred())

		SpokeTektonClientset, err = tekton.NewForConfig(restConfig)
		Expect(err).NotTo(HaveOccurred())

		SpokeKueueClientset, err = kueue.NewForConfig(restConfig)
		Expect(err).NotTo(HaveOccurred())

	})
})
