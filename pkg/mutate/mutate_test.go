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

package mutate

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/konflux-ci/tekton-kueue/pkg/common"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	tekv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/yaml"
)

func TestMutate(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Mutate Suite")
}

const validPipelineRunYAML = `apiVersion: tekton.dev/v1
kind: PipelineRun
metadata:
  name: test-pipelinerun
spec:
  pipelineRef:
    name: my-pipeline
`

var _ = Describe("MutatePipelineRun", func() {
	var (
		tmpDir string
	)

	BeforeEach(func() {
		var err error
		tmpDir, err = os.MkdirTemp("", "mutate-test")
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		Expect(os.RemoveAll(tmpDir)).To(Succeed())
	})

	Context("with valid PipelineRun", func() {
		It("should mutate a PipelineRun with pipelineRef", func() {
			// Write config file
			configPath := filepath.Join(tmpDir, "config.yaml")
			Expect(os.WriteFile(configPath, []byte(`queueName: "test-queue"`), 0644)).To(Succeed())

			// Write PipelineRun file
			plrPath := filepath.Join(tmpDir, "pipelinerun.yaml")
			Expect(os.WriteFile(plrPath, []byte(validPipelineRunYAML), 0644)).To(Succeed())

			// Call MutatePipelineRun
			mutatedData, err := MutatePipelineRun(plrPath, tmpDir)
			Expect(err).NotTo(HaveOccurred())

			// Parse and validate
			var pipelineRun tekv1.PipelineRun
			Expect(yaml.Unmarshal(mutatedData, &pipelineRun)).To(Succeed())
			Expect(pipelineRun.Labels[common.QueueLabel]).To(Equal("test-queue"))
			Expect(pipelineRun.Spec.Status).To(Equal(tekv1.PipelineRunSpecStatus(tekv1.PipelineRunSpecStatusPending)))
		})

		It("should mutate a PipelineRun with a CEL expression", func() {
			// Write config file with CEL expressions
			configPath := filepath.Join(tmpDir, "config.yaml")
			configContent := `
queueName: "test-queue"
cel:
  expressions:
    - 'label("env", "production")'
    - 'annotation("team", "platform")'
`
			Expect(os.WriteFile(configPath, []byte(configContent), 0644)).To(Succeed())

			// Write PipelineRun file
			plrPath := filepath.Join(tmpDir, "pipelinerun.yaml")
			Expect(os.WriteFile(plrPath, []byte(validPipelineRunYAML), 0644)).To(Succeed())

			// Call MutatePipelineRun
			mutatedData, err := MutatePipelineRun(plrPath, tmpDir)
			Expect(err).NotTo(HaveOccurred())

			// Parse and validate
			var pipelineRun tekv1.PipelineRun
			Expect(yaml.Unmarshal(mutatedData, &pipelineRun)).To(Succeed())
			Expect(pipelineRun.Labels[common.QueueLabel]).To(Equal("test-queue"))
			Expect(pipelineRun.Labels["env"]).To(Equal("production"))
			Expect(pipelineRun.Annotations["team"]).To(Equal("platform"))
			Expect(pipelineRun.Spec.Status).To(Equal(tekv1.PipelineRunSpecStatus(tekv1.PipelineRunSpecStatusPending)))
		})

		It("should mutate a PipelineRun with pipelineSpec", func() {
			// Write config file
			configPath := filepath.Join(tmpDir, "config.yaml")
			Expect(os.WriteFile(configPath, []byte(`queueName: "my-queue"`), 0644)).To(Succeed())

			// Write PipelineRun file
			plrContent := `apiVersion: tekton.dev/v1
kind: PipelineRun
metadata:
  name: test-pipelinerun
spec:
  pipelineSpec:
    tasks:
      - name: echo
        taskSpec:
          steps:
            - name: echo
              image: alpine:latest
              script: echo hello
`
			plrPath := filepath.Join(tmpDir, "pipelinerun.yaml")
			Expect(os.WriteFile(plrPath, []byte(plrContent), 0644)).To(Succeed())

			// Call MutatePipelineRun
			mutatedData, err := MutatePipelineRun(plrPath, tmpDir)
			Expect(err).NotTo(HaveOccurred())

			// Parse and validate
			var pipelineRun tekv1.PipelineRun
			Expect(yaml.Unmarshal(mutatedData, &pipelineRun)).To(Succeed())
			Expect(pipelineRun.Labels[common.QueueLabel]).To(Equal("my-queue"))
			Expect(pipelineRun.Spec.Status).To(Equal(tekv1.PipelineRunSpecStatus(tekv1.PipelineRunSpecStatusPending)))
		})
	})

	Context("with invalid PipelineRun", func() {
		It("should reject a PipelineRun with neither pipelineRef nor pipelineSpec", func() {
			// Write config file with a CEL expression so the CEL mutator is created
			// and Validate() runs on the PipelineRun copy.
			configPath := filepath.Join(tmpDir, "config.yaml")
			configContent := `
queueName: "test-queue"
cel:
  expressions:
    - 'label("env", "test")'
`
			Expect(os.WriteFile(configPath, []byte(configContent), 0644)).To(Succeed())

			// Write invalid PipelineRun file
			plrContent := `apiVersion: tekton.dev/v1
kind: PipelineRun
metadata:
  name: test-pipelinerun
spec: {}
`
			plrPath := filepath.Join(tmpDir, "pipelinerun.yaml")
			Expect(os.WriteFile(plrPath, []byte(plrContent), 0644)).To(Succeed())

			// Call MutatePipelineRun - should fail with a BadRequest error
			_, err := MutatePipelineRun(plrPath, tmpDir)
			Expect(err).To(HaveOccurred())
			Expect(k8serrors.IsBadRequest(err)).To(BeTrue(), "expected BadRequest error, got: %v", err)
			Expect(err.Error()).To(ContainSubstring("expected exactly one, got neither: pipelineRef, pipelineSpec"))
		})
	})

	Context("with CEL evaluation error", func() {
		It("should return InternalServerError when a CEL expression fails at runtime", func() {
			// Write config with a CEL expression that accesses a non-existent field
			configPath := filepath.Join(tmpDir, "config.yaml")
			configContent := `
queueName: "test-queue"
cel:
  expressions:
    - 'annotation("key", pipelineRun.doesNotExist)'
`
			Expect(os.WriteFile(configPath, []byte(configContent), 0644)).To(Succeed())

			// Write a valid PipelineRun
			plrPath := filepath.Join(tmpDir, "pipelinerun.yaml")
			Expect(os.WriteFile(plrPath, []byte(validPipelineRunYAML), 0644)).To(Succeed())

			// Call MutatePipelineRun - should fail with an InternalServerError
			_, err := MutatePipelineRun(plrPath, tmpDir)
			Expect(err).To(HaveOccurred())
			Expect(k8serrors.IsInternalError(err)).To(BeTrue(), "expected InternalServerError, got: %v", err)
			Expect(err.Error()).To(ContainSubstring("CEL evaluation failed"))
		})
	})

	Context("with invalid inputs", func() {
		It("should reject empty pipelineRunFile", func() {
			_, err := MutatePipelineRun("", "/tmp")
			Expect(err).To(HaveOccurred())
		})

		It("should reject empty configDir", func() {
			_, err := MutatePipelineRun("/tmp/plr.yaml", "")
			Expect(err).To(HaveOccurred())
		})

		It("should reject non-existent pipelinerun file", func() {
			// Write config file
			configPath := filepath.Join(tmpDir, "config.yaml")
			Expect(os.WriteFile(configPath, []byte(`queueName: "test-queue"`), 0644)).To(Succeed())

			_, err := MutatePipelineRun("/non/existent/file.yaml", tmpDir)
			Expect(err).To(HaveOccurred())
		})

		It("should reject non-existent config dir", func() {
			// Write PipelineRun file
			plrContent := `apiVersion: tekton.dev/v1
kind: PipelineRun
metadata:
  name: test
spec:
  pipelineRef:
    name: my-pipeline
`
			plrPath := filepath.Join(tmpDir, "pipelinerun.yaml")
			Expect(os.WriteFile(plrPath, []byte(plrContent), 0644)).To(Succeed())

			_, err := MutatePipelineRun(plrPath, "/non/existent/config")
			Expect(err).To(HaveOccurred())
		})
	})
})

var _ = Describe("LoadDefaulter", func() {
	var (
		tmpDir string
	)

	BeforeEach(func() {
		var err error
		tmpDir, err = os.MkdirTemp("", "mutate-test")
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		Expect(os.RemoveAll(tmpDir)).To(Succeed())
	})

	It("should load a valid config", func() {
		configPath := filepath.Join(tmpDir, "config.yaml")
		Expect(os.WriteFile(configPath, []byte(`queueName: "test-queue"`), 0644)).To(Succeed())

		cfgStore, err := LoadDefaulter(tmpDir)
		Expect(err).NotTo(HaveOccurred())
		Expect(cfgStore).NotTo(BeNil())
	})

	It("should reject empty configDir", func() {
		_, err := LoadDefaulter("")
		Expect(err).To(HaveOccurred())
	})

	It("should reject missing config file", func() {
		_, err := LoadDefaulter(tmpDir)
		Expect(err).To(HaveOccurred())
	})

	It("should reject invalid config", func() {
		// Write invalid config (missing queueName)
		configPath := filepath.Join(tmpDir, "config.yaml")
		Expect(os.WriteFile(configPath, []byte(`invalid: config`), 0644)).To(Succeed())

		_, err := LoadDefaulter(tmpDir)
		Expect(err).To(HaveOccurred())
	})
})
