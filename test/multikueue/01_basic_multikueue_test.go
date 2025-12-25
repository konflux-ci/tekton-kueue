package multikueue

import (
	"context"
	"fmt"
	"os"
	"os/exec"

	v1 "github.com/konflux-ci/tekton-queue/internal/webhook/v1"
	"github.com/konflux-ci/tekton-queue/test/utils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	plrv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilrand "k8s.io/apimachinery/pkg/util/rand"
	"knative.dev/pkg/apis"
	kueueb1 "sigs.k8s.io/kueue/apis/kueue/v1beta1"
	kueue "sigs.k8s.io/kueue/client-go/clientset/versioned"
)

const (
	NamespacePrefix = "mk-e2e-"
	localQueue      = "pipelines-queue"
)

var _ = Describe("MultiKueue Basic Scheduling", Ordered, Label("multikueue", "smoke"), func() {
	ctx := context.Background()
	var nsName string

	BeforeEach(func() {
		nsName = NamespacePrefix + utilrand.String(4)

		By("Setup Namespace on Hub Cluster Namespace:", func() {
			_, err := HubClientset.CoreV1().Namespaces().Create(ctx, &corev1.Namespace{
				ObjectMeta: meta.ObjectMeta{Name: nsName},
			}, meta.CreateOptions{})
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
			_, err = utils.Run(cmd)
			Expect(err).To(Succeed(), "Failed to apply kueue resources")
		})
		By("Setup Namespace on Spoke Cluster", func() {

			_, err := SpokeClientset.CoreV1().Namespaces().Create(ctx, &corev1.Namespace{
				ObjectMeta: meta.ObjectMeta{Name: nsName},
			}, meta.CreateOptions{})
			Expect(err).NotTo(HaveOccurred())

			cmd := exec.Command(
				"kubectl",
				"--context",
				SpokeKubeContext,
				"apply",
				"--server-side",
				"-n",
				nsName,
				"-f",
				"testdata/kueue-resources.yaml",
			)
			out, err := cmd.CombinedOutput()
			Expect(err).To(Succeed(), string(out))

		})
	})
	AfterEach(func() {
		_ = SpokeClientset.CoreV1().Namespaces().Delete(ctx, nsName, meta.DeleteOptions{})
		_ = HubClientset.CoreV1().Namespaces().Delete(ctx, nsName, meta.DeleteOptions{})
	})

	It("PipelineRun Must be scheduled on Spoke Cluster", func() {
		t := GinkgoT()

		var plr *plrv1.PipelineRun
		By("Create a pipelinerun", func() {
			data, err := os.ReadFile("testdata/pipelinerun-without-queue-label.yaml")
			Expect(err).NotTo(HaveOccurred())

			yamlString := string(data)
			plr = utils.MustParseV1PipelineRun(t, yamlString)
			plr, err = HubTektonClientset.TektonV1().PipelineRuns(nsName).Create(ctx, plr, meta.CreateOptions{})
			Expect(err).NotTo(HaveOccurred())
		})
		By(" Check Labels on pipelinerun "+plr.Name, func() {
			createdPLR, err := HubTektonClientset.TektonV1().PipelineRuns(nsName).Get(ctx, plr.Name, meta.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(createdPLR.Labels).To(HaveKeyWithValue(v1.QueueLabel, localQueue))
			Expect(createdPLR.Labels).To(HaveKeyWithValue("kueue.x-k8s.io/priority-class", "tekton-kueue-default"))
			Expect(*createdPLR.Spec.ManagedBy).To(Equal("kueue.x-k8s.io/multikueue"))
		})

		By("Validate Workload on Hub Cluster", func() {
			validateWorkloads(ctx, HubKueueClientset, nsName)
		})

		By("Validate Workload on Spoke Cluster", func() {
			validateWorkloads(ctx, SpokeKueueClientset, nsName)
		})

		By("Validate PipelineRun on Spoke Cluster", func() {
			createdPLR, err := SpokeTektonClientset.TektonV1().PipelineRuns(nsName).Get(ctx, plr.Name, meta.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(createdPLR.Labels).To(HaveKeyWithValue(v1.QueueLabel, localQueue))
			Expect(createdPLR.Labels).To(HaveKeyWithValue("kueue.x-k8s.io/priority-class", "tekton-kueue-default"))
			Expect(createdPLR.Spec.ManagedBy).To(BeNil())
		})

		By("Wait for PipelineRun to Prune from Spoke", func() {
			Eventually(func(g Gomega) {
				plr, err := SpokeTektonClientset.TektonV1().PipelineRuns(nsName).Get(ctx, plr.Name, meta.GetOptions{})
				if err == nil {
					reason := plr.Status.GetCondition(apis.ConditionSucceeded).GetReason()
					_, _ = fmt.Fprintf(GinkgoWriter, "PipelineRun %s - %+v\n", plr.Name, reason)
					g.Expect(reason).To(BeEquivalentTo("Succeeded"))
				} else {
					_, _ = fmt.Fprintf(GinkgoWriter, "Error: %+v\n", err)
					g.Expect(errors.IsNotFound(err)).To(BeTrue())
					_, _ = fmt.Fprintf(GinkgoWriter, "Error: %+v\n", err)
				}
			}, "10m", "5s").Should(Succeed())
		})

		By("PipelineRun Status on Hub Cluster should be Succeeded", func() {
			plr, err := HubTektonClientset.TektonV1().PipelineRuns(nsName).Get(ctx, plr.Name, meta.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
			reason := plr.Status.GetCondition(apis.ConditionSucceeded).GetReason()
			Expect(reason).To(Equal("Succeeded"))

		})
	})
})

func validateWorkloads(ctx context.Context, clientSet *kueue.Clientset, nsName string) {
	var wl *kueueb1.WorkloadList
	Eventually(func(g Gomega) {
		wl, _ = clientSet.KueueV1beta1().Workloads(nsName).List(ctx, meta.ListOptions{})
		g.Expect(wl.Items).ShouldNot(BeEmpty())

	}, "30s", "5s").Should(Succeed())

	// Validate Workload
	Expect(wl.Items).ShouldNot(BeEmpty())
	for _, w := range wl.Items {
		Expect(w.Spec.QueueName).To(Equal(localQueue))
	}
}
