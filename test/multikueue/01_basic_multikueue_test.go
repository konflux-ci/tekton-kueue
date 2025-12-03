package multikueue

import (
	"context"
	"os"
	"os/exec"

	"github.com/konflux-ci/tekton-kueue/internal/common"
	"github.com/konflux-ci/tekton-kueue/test/utils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	plrv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	corev1 "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilrand "k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	kueue "sigs.k8s.io/kueue/apis/kueue/v1beta1"
)

const (
	NamespacePrefix = "mk-e2e"
	localQueue      = "pipelines-queue"
)

var (
	SpokeClientset kubernetes.Interface
)

var _ = Describe("MultiKueue Basic Scheduling", Ordered, Label("multikueue", "smoke"), func() {
	ctx := context.Background()
	var nsName string

	BeforeEach(func() {
		nsName = NamespacePrefix + utilrand.String(4)

		By("Setup Namespace on Hub Cluster Namespace:", func() {
			_, err := Clientset.CoreV1().Namespaces().Create(ctx, &corev1.Namespace{
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
			contextName := "kind-spoke-1"
			spokeConfig := clientcmd.NewNonInteractiveClientConfig(*rawConfig, contextName, &clientcmd.ConfigOverrides{}, nil)
			restConfig, err := spokeConfig.ClientConfig()
			Expect(err).NotTo(HaveOccurred())

			SpokeClientset, err = kubernetes.NewForConfig(restConfig)
			Expect(err).NotTo(HaveOccurred())

			_, err = SpokeClientset.CoreV1().Namespaces().Create(ctx, &corev1.Namespace{
				ObjectMeta: meta.ObjectMeta{Name: nsName},
			}, meta.CreateOptions{})
			Expect(err).NotTo(HaveOccurred())

			cmd := exec.Command(
				"kubectl",
				"--context",
				contextName,
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
		_ = Clientset.CoreV1().Namespaces().Delete(ctx, nsName, meta.DeleteOptions{})
	})

	It("PipelineRun Must be scheduled on Spoke Cluster", func() {
		t := GinkgoT()

		var plr *plrv1.PipelineRun
		By("Create a pipelinerun", func() {
			data, err := os.ReadFile("testdata/pipelinerun-without-queue-label.yaml")
			Expect(err).NotTo(HaveOccurred())

			yamlString := string(data)
			plr = utils.MustParseV1PipelineRun(t, yamlString)
			plr, err = TektonClientset.TektonV1().PipelineRuns(nsName).Create(ctx, plr, meta.CreateOptions{})
			Expect(err).NotTo(HaveOccurred())
		})
		By(" Check Labels on pipelinerun "+plr.Name, func() {
			createdPLR, err := TektonClientset.TektonV1().PipelineRuns(nsName).Get(ctx, plr.Name, meta.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(createdPLR.Labels).To(HaveKeyWithValue(common.QueueLabel, localQueue))
			Expect(createdPLR.Labels).To(HaveKeyWithValue("kueue.x-k8s.io/priority-class", "tekton-kueue-default"))
			Expect(*createdPLR.Spec.ManagedBy).To(Equal("kueue.x-k8s.io/multikueue"))
		})

		By("Validate Workload on Hub Cluster", func() {
			var wl *kueue.WorkloadList
			Eventually(func() int {
				wl, _ = KueueClientset.KueueV1beta1().Workloads(nsName).List(ctx, meta.ListOptions{})
				return len(wl.Items)
			}, "30s", "5s").Should(BeNumerically(">", 0))

			// Validate Workload
			Expect(len(wl.Items)).Should(BeNumerically(">", 0))
			for _, w := range wl.Items {
				Expect(w.Spec.QueueName).To(Equal(localQueue))
			}
		})

		By("Validate Workload on Spoke Cluster", func() {

		})

	})
})
