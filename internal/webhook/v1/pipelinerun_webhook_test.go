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

package v1

import (
	"context"

	"github.com/konflux-ci/tekton-queue/internal/config"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	tektondevv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
)

var _ = Describe("PipelineRun Webhook", func() {
	var (
		defaulter *pipelineRunCustomDefaulter
		plr       *tektondevv1.PipelineRun
		ctx       context.Context
	)

	BeforeEach(func() {
		ctx = context.Background()
		plr = &tektondevv1.PipelineRun{}
	})

	Describe("Default", func() {
		Context("when isMultiKueue is true", func() {
			It("should not set the status", func() {
				cfg := &config.Config{
					QueueName:    "test-queue",
					IsMultiKueue: true,
				}
				var err error
				defaulter, err = newDefaulter(cfg, []PipelineRunMutator{})
				Expect(err).NotTo(HaveOccurred())
				err = defaulter.Default(ctx, plr)
				Expect(err).NotTo(HaveOccurred())
				Expect(plr.Spec.Status).To(BeEmpty())
			})
		})

		Context("when isMultiKueue is false", func() {
			It("should set the status to Pending", func() {
				cfg := &config.Config{
					QueueName:    "test-queue",
					IsMultiKueue: false,
				}
				var err error
				defaulter, err = newDefaulter(cfg, []PipelineRunMutator{})
				Expect(err).NotTo(HaveOccurred())
				err = defaulter.Default(ctx, plr)
				Expect(err).NotTo(HaveOccurred())
				Expect(plr.Spec.Status).To(Equal(tektondevv1.PipelineRunSpecStatus(tektondevv1.PipelineRunSpecStatusPending)))
			})
		})

		It("should set the queue name", func() {
			cfg := &config.Config{
				QueueName: "test-queue",
			}
			var err error
			defaulter, err = newDefaulter(cfg, []PipelineRunMutator{})
			Expect(err).NotTo(HaveOccurred())
			err = defaulter.Default(ctx, plr)
			Expect(err).NotTo(HaveOccurred())
			Expect(plr.Labels[QueueLabel]).To(Equal("test-queue"))
		})
	})
})

func newDefaulter(cfg *config.Config, mutators []PipelineRunMutator) (*pipelineRunCustomDefaulter, error) {
	defaulter, err := NewCustomDefaulter(cfg, mutators)
	if err != nil {
		return nil, err
	}
	return defaulter.(*pipelineRunCustomDefaulter), nil
}