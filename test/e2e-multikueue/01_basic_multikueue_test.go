package e2e_multikueue

import (
	"context"
	"os"
	"os/exec"
	"time"

	"github.com/konflux-ci/tekton-kueue/pkg/common"
	"github.com/konflux-ci/tekton-kueue/test/utils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	plrv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilrand "k8s.io/apimachinery/pkg/util/rand"
	"sigs.k8s.io/controller-runtime/pkg/client"
	kueue "sigs.k8s.io/kueue/apis/kueue/v1beta1"
)

const (
	NamespacePrefix = "mk-e2e"
	localQueue      = "pipelines-queue"
)

var _ = Describe("MultiKueue Basic Scheduling", Ordered, Label("multikueue", "smoke"), func() {
	ctx := context.Background()
	var nsName = NamespacePrefix + utilrand.String(5)
	BeforeEach(func() {

		By("Setup Namespace on Hub Cluster Namespace:", func() {
			var ns = &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: nsName,
				},
			}
			err := hubClient.Create(ctx, ns)
			Expect(err).NotTo(HaveOccurred())

			cmd := exec.Command(
				"kubectl",
				"apply",
				"--server-side",
				"-n",
				nsName,
				"-f",
				"testdata/multikueue-resources.yaml",
			)
			_, err = cmd.CombinedOutput()

			Expect(err).To(Succeed(), "Failed to apply kueue resources")
		})
		By("Setup Namespace on Spoke Cluster", func() {

			var ns = &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: nsName,
				},
			}

			err := spokeClient.Create(ctx, ns)
			Expect(err).NotTo(HaveOccurred())

			cmd := exec.Command(
				"kubectl",
				"--context",
				spokeContextName,
				"apply",
				"--server-side",
				"-n",
				nsName,
				"-f",
				"testdata/kueue-resources.yaml",
			)
			_, err = cmd.CombinedOutput()
			Expect(err).NotTo(HaveOccurred())
		})
	})
	AfterEach(func() {
		var ns = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: nsName,
			},
		}

		_ = hubClient.Delete(ctx, ns)
		_ = spokeClient.Delete(ctx, ns)
	})

	It("PipelineRun Must be scheduled on Spoke Cluster", func() {
		t := GinkgoT()
		var plr plrv1.PipelineRun

		By("Create a pipelinerun", func() {
			data, err := os.ReadFile("testdata/pipelinerun-without-queue-label.yaml")
			Expect(err).NotTo(HaveOccurred())

			yamlString := string(data)
			utils.MustParseYAML(t, yamlString, &plr)
			plr.Namespace = nsName
			err = hubClient.Create(ctx, &plr)
			Expect(err).NotTo(HaveOccurred())
		})
		time.Sleep(1 * time.Second)
		By(" Check Labels on pipelinerun "+plr.Name, func() {
			err := hubClient.Get(ctx, plr.GetNamespacedName(), &plr)
			Expect(err).NotTo(HaveOccurred())
			Expect(plr.Labels).To(HaveKeyWithValue(common.QueueLabel, localQueue))
			Expect(plr.Labels).To(HaveKeyWithValue("kueue.x-k8s.io/priority-class", "tekton-kueue-default"))
			Expect(*plr.Spec.ManagedBy).To(Equal("kueue.x-k8s.io/multikueue"))
		})

		By("Validate Workload on Hub Cluster", func() {
			var wl kueue.WorkloadList
			Eventually(func() int {
				err := hubClient.List(ctx, &wl, client.InNamespace(nsName))
				Expect(err).NotTo(HaveOccurred())
				return len(wl.Items)
			}, "30s", "5s").Should(BeNumerically(">", 0))

			// Validate Workload
			Expect(wl.Items).ShouldNot(BeEmpty())
			for _, w := range wl.Items {
				Expect(string(w.Spec.QueueName)).To(Equal(localQueue))
			}
		})

		By("Validate Workload on Spoke Cluster", func() {
			var wl kueue.WorkloadList
			Eventually(func() int {
				err := spokeClient.List(ctx, &wl)
				Expect(err).NotTo(HaveOccurred())
				return len(wl.Items)
			}, "30s", "5s").Should(BeNumerically(">", 0))

			// Validate Workload
			Expect(wl.Items).ShouldNot(BeEmpty())
			for _, w := range wl.Items {
				Expect(string(w.Spec.QueueName)).To(Equal(localQueue))
			}
		})

		By("Validate PipelineRun on Spoke Cluster", func() {
			Eventually(func(g Gomega) {
				err := spokeClient.Get(ctx, plr.GetNamespacedName(), &plr)
				g.Expect(err).NotTo(HaveOccurred())
			}, "30s", "5s").Should(Succeed())
		})

	})
})
