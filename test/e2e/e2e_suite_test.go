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

package e2e

import (
	"fmt"
	"os"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"os/exec"

	"github.com/konflux-ci/tekton-kueue/test/utils"
)

var (
	// Optional Environment Variables:
	// - PROMETHEUS_INSTALL_SKIP=true: Skips Prometheus Operator installation during test setup.
	// - CERT_MANAGER_INSTALL_SKIP=true: Skips CertManager installation during test setup.
	// These variables are useful if Prometheus or CertManager is already installed, avoiding
	// re-installation and conflicts.
	skipPrometheusInstall  = os.Getenv("PROMETHEUS_INSTALL_SKIP") == "true"
	skipCertManagerInstall = os.Getenv("CERT_MANAGER_INSTALL_SKIP") == "true"
	// isPrometheusOperatorAlreadyInstalled will be set true when prometheus CRDs be found on the cluster
	isPrometheusOperatorAlreadyInstalled = false
	// isCertManagerAlreadyInstalled will be set true when CertManager CRDs be found on the cluster
	isCertManagerAlreadyInstalled = false
)

// TestE2E runs the end-to-end (e2e) test suite for the project. These tests execute in an isolated,
// temporary environment to validate project changes with the the purposed to be used in CI jobs.
// The default setup installs CertManager and Prometheus.
// The IMG environment varialbe must be specified with the image that should be used by the controller's deployment
func TestE2E(t *testing.T) {
	RegisterFailHandler(Fail)
	_, _ = fmt.Fprintf(GinkgoWriter, "Starting tekton-kueue integration test suite\n")
	RunSpecs(t, "e2e suite")
}

var _ = BeforeSuite(func() {
	By("Ensure that Prometheus is enabled")
	_ = utils.UncommentCode("config/default/kustomization.yaml", "#- ../prometheus", "#")

	// The tests-e2e are intended to run on a temporary cluster that is created and destroyed for testing.
	// To prevent errors when tests run in environments with Prometheus or CertManager already installed,
	// we check for their presence before execution.
	// Setup Prometheus and CertManager before the suite if not skipped and if not already installed
	if !skipPrometheusInstall {
		By("checking if prometheus is installed already")
		isPrometheusOperatorAlreadyInstalled = utils.IsPrometheusCRDsInstalled()
		if !isPrometheusOperatorAlreadyInstalled {
			_, _ = fmt.Fprintf(GinkgoWriter, "Installing Prometheus Operator...\n")
			Expect(utils.InstallPrometheusOperator()).To(Succeed(), "Failed to install Prometheus Operator")
		} else {
			_, _ = fmt.Fprintf(GinkgoWriter, "WARNING: Prometheus Operator is already installed. Skipping installation...\n")
		}
	}
	if !skipCertManagerInstall {
		By("checking if cert manager is installed already")
		isCertManagerAlreadyInstalled = utils.IsCertManagerCRDsInstalled()
		if !isCertManagerAlreadyInstalled {
			_, _ = fmt.Fprintf(GinkgoWriter, "Installing CertManager...\n")

			Eventually(
				utils.InstallCertManager, 3*time.Second, 1*time.Second,
			).Should(Succeed(), "Failed to install CertManager")
		} else {
			_, _ = fmt.Fprintf(GinkgoWriter, "WARNING: CertManager is already installed. Skipping installation...\n")
		}
	}

	By("checking if Kueue is installed", func() {
		cmd := exec.Command("kubectl", "get", "crd", "workloads.kueue.x-k8s.io")
		_, err := utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Kueue isn't installed")
	})

	By("checking if Kueue is installed", func() {
		cmd := exec.Command("kubectl", "get", "crd", "pipelineruns.tekton.dev")
		_, err := utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Tekton isn't installed")
	})
})

var _ = AfterSuite(func() {
	// Teardown Prometheus and CertManager after the suite if not skipped and if they were not already installed
	if !skipPrometheusInstall && !isPrometheusOperatorAlreadyInstalled {
		_, _ = fmt.Fprintf(GinkgoWriter, "Uninstalling Prometheus Operator...\n")
		utils.UninstallPrometheusOperator()
	}
	if !skipCertManagerInstall && !isCertManagerAlreadyInstalled {
		_, _ = fmt.Fprintf(GinkgoWriter, "Uninstalling CertManager...\n")
		utils.UninstallCertManager()
	}
})
