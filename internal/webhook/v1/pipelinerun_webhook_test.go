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
	"encoding/json"
	"testing"

	"github.com/konflux-ci/tekton-kueue/pkg/common"
	"github.com/konflux-ci/tekton-kueue/pkg/config"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	tektondevv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

func TestV1Webhook(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "V1 Webhook Suite")
}

var _ = Describe("PipelineRun Webhook", func() {
	var (
		defaulter webhook.CustomDefaulter
		plr       *tektondevv1.PipelineRun
	)

	BeforeEach(func(ctx context.Context) {
		plr = &tektondevv1.PipelineRun{
			Spec: tektondevv1.PipelineRunSpec{
				PipelineRef: &tektondevv1.PipelineRef{
					Name: "test-pipeline",
				},
			},
		}
	})

	Describe("Default", func() {
		Context("when MultiKueueOverride is true", func() {
			It("should set the managedBy", func(ctx context.Context) {
				cfg := &config.Config{
					QueueName:          "test-queue",
					MultiKueueOverride: true,
				}

				cfgStore := &ConfigStore{
					config: cfg,
				}

				var err error
				defaulter, err = NewCustomDefaulter(cfgStore)
				Expect(err).NotTo(HaveOccurred())
				err = defaulter.Default(ctx, plr)
				Expect(err).NotTo(HaveOccurred())
				Expect(*plr.Spec.ManagedBy).To(Equal(common.ManagedByMultiKueueLabel))
				Expect(plr.Spec.Status).To(Equal(tektondevv1.PipelineRunSpecStatus(tektondevv1.PipelineRunSpecStatusPending)))
			})
		})

		Context("when MultiKueueOverride is false", func() {
			It("should set the status to Pending", func(ctx context.Context) {
				cfg := &config.Config{
					QueueName:          "test-queue",
					MultiKueueOverride: false,
				}
				cfgStore := &ConfigStore{
					config: cfg,
				}
				var err error
				defaulter, err = NewCustomDefaulter(cfgStore)
				Expect(err).NotTo(HaveOccurred())
				err = defaulter.Default(ctx, plr)
				Expect(err).NotTo(HaveOccurred())
				Expect(plr.Spec.Status).To(Equal(tektondevv1.PipelineRunSpecStatus(tektondevv1.PipelineRunSpecStatusPending)))
			})
		})

		It("should set the queue name", func(ctx context.Context) {
			cfg := &config.Config{
				QueueName: "test-queue",
			}
			cfgStore := &ConfigStore{
				config: cfg,
			}
			var err error
			defaulter, err = NewCustomDefaulter(cfgStore)
			Expect(err).NotTo(HaveOccurred())
			err = defaulter.Default(ctx, plr)
			Expect(err).NotTo(HaveOccurred())
			Expect(plr.Labels[common.QueueLabel]).To(Equal("test-queue"))
		})

		It("should accept a valid PipelineRun with pipelineRef", func(ctx context.Context) {
			plrWithRef := &tektondevv1.PipelineRun{
				Spec: tektondevv1.PipelineRunSpec{
					PipelineRef: &tektondevv1.PipelineRef{
						Name: "my-pipeline",
					},
				},
			}

			cfg := &config.Config{
				QueueName: "test-queue",
			}
			cfgStore := &ConfigStore{
				config: cfg,
			}
			var err error
			defaulter, err = NewCustomDefaulter(cfgStore)
			Expect(err).NotTo(HaveOccurred())
			err = defaulter.Default(ctx, plrWithRef)
			Expect(err).NotTo(HaveOccurred())
			Expect(plrWithRef.Spec.Status).To(Equal(tektondevv1.PipelineRunSpecStatus(tektondevv1.PipelineRunSpecStatusPending)))
			Expect(plrWithRef.Labels[common.QueueLabel]).To(Equal("test-queue"))
		})

		It("should accept a valid PipelineRun with pipelineSpec", func(ctx context.Context) {
			plrWithSpec := &tektondevv1.PipelineRun{
				Spec: tektondevv1.PipelineRunSpec{
					PipelineSpec: &tektondevv1.PipelineSpec{
						Tasks: []tektondevv1.PipelineTask{
							{
								Name: "echo",
								TaskSpec: &tektondevv1.EmbeddedTask{
									TaskSpec: tektondevv1.TaskSpec{
										Steps: []tektondevv1.Step{
											{
												Name:   "echo",
												Image:  "alpine:latest",
												Script: "echo hello",
											},
										},
									},
								},
							},
						},
					},
				},
			}

			cfg := &config.Config{
				QueueName: "test-queue",
			}
			cfgStore := &ConfigStore{
				config: cfg,
			}
			var err error
			defaulter, err = NewCustomDefaulter(cfgStore)
			Expect(err).NotTo(HaveOccurred())
			err = defaulter.Default(ctx, plrWithSpec)
			Expect(err).NotTo(HaveOccurred())
			Expect(plrWithSpec.Spec.Status).To(Equal(tektondevv1.PipelineRunSpecStatus(tektondevv1.PipelineRunSpecStatusPending)))
			Expect(plrWithSpec.Labels[common.QueueLabel]).To(Equal("test-queue"))
		})

		It("should reject an invalid PipelineRun with neither pipelineRef nor pipelineSpec", func(ctx context.Context) {
			invalidPlr := &tektondevv1.PipelineRun{
				Spec: tektondevv1.PipelineRunSpec{
					// Neither pipelineRef nor pipelineSpec is set - this is invalid
				},
			}

			cfg := &config.Config{
				QueueName: "test-queue",
			}
			cfgStore := &ConfigStore{
				config: cfg,
			}
			var err error
			defaulter, err = NewCustomDefaulter(cfgStore)
			Expect(err).NotTo(HaveOccurred())
			Expect(defaulter.Default(ctx, invalidPlr)).
				Error().
				To(And(
					Satisfy(errors.IsBadRequest),
					MatchError(ContainSubstring("invalid pipelinerun: expected exactly one, got neither: pipelineRef, pipelineSpec"))))
		})

		It("should accept a PipelineRun with pipelineSpec containing a parameter without explicit type", func(ctx context.Context) {
			// This reproduces the bug where PipelineRuns with parameters missing the 'type' field
			// (like enable-cache-proxy) were rejected with:
			// "invalid value: : params.enable-cache-proxy.type"
			// The type should default to "string" and be valid.
			plrWithParamNoType := &tektondevv1.PipelineRun{
				Spec: tektondevv1.PipelineRunSpec{
					Params: []tektondevv1.Param{
						{
							Name:  "enable-cache-proxy",
							Value: tektondevv1.ParamValue{Type: tektondevv1.ParamTypeString, StringVal: "false"},
						},
					},
					PipelineSpec: &tektondevv1.PipelineSpec{
						Params: []tektondevv1.ParamSpec{
							{
								Name:        "enable-cache-proxy",
								Description: "Enable cache proxy configuration",
								Default:     &tektondevv1.ParamValue{Type: tektondevv1.ParamTypeString, StringVal: "false"},
								// Note: Type field is NOT set - this should default to string
							},
						},
						Tasks: []tektondevv1.PipelineTask{
							{
								Name: "init",
								Params: []tektondevv1.Param{
									{
										Name:  "enable-cache-proxy",
										Value: tektondevv1.ParamValue{Type: tektondevv1.ParamTypeString, StringVal: "$(params.enable-cache-proxy)"},
									},
								},
								TaskSpec: &tektondevv1.EmbeddedTask{
									TaskSpec: tektondevv1.TaskSpec{
										Params: []tektondevv1.ParamSpec{
											{
												Name: "enable-cache-proxy",
												// Type field is NOT set
											},
										},
										Steps: []tektondevv1.Step{
											{
												Name:   "echo",
												Image:  "alpine:latest",
												Script: "echo $(params.enable-cache-proxy)",
											},
										},
									},
								},
							},
						},
					},
				},
			}

			cfg := &config.Config{
				QueueName: "test-queue",
			}
			cfgStore := &ConfigStore{
				config: cfg,
			}
			var err error
			defaulter, err = NewCustomDefaulter(cfgStore)
			Expect(err).NotTo(HaveOccurred())
			err = defaulter.Default(ctx, plrWithParamNoType)
			Expect(err).NotTo(HaveOccurred())
			Expect(plrWithParamNoType.Spec.Status).To(Equal(tektondevv1.PipelineRunSpecStatus(tektondevv1.PipelineRunSpecStatusPending)))
			Expect(plrWithParamNoType.Labels[common.QueueLabel]).To(Equal("test-queue"))
		})

		It("should reject an invalid PipelineRun", func(ctx context.Context) {
			// found via fuzzing
			badJson := []byte("{\"spec\":{\"pipelineSpec\":{\"params\":[{}]},\"params\":[{}]}}")

			pipelineRun := tektondevv1.PipelineRun{}
			Expect(json.Unmarshal(badJson, &pipelineRun)).To(Succeed())

			cfg := &config.Config{
				QueueName: "test-queue",
			}
			cfgStore := &ConfigStore{
				config: cfg,
			}
			var err error
			defaulter, err = NewCustomDefaulter(cfgStore)
			Expect(err).NotTo(HaveOccurred())
			// we expect to see a 400 Bad Request here
			Expect(defaulter.Default(ctx, &pipelineRun)).
				Error().
				To(And(
					Satisfy(errors.IsBadRequest),
					MatchError(ContainSubstring("failed to serialize pipelinerun"))))
		})

		It("should reject a non-pipelinerun object", func(ctx context.Context) {
			cfg := &config.Config{
				QueueName: "test-queue",
			}
			cfgStore := &ConfigStore{
				config: cfg,
			}
			var err error
			defaulter, err = NewCustomDefaulter(cfgStore)
			Expect(err).NotTo(HaveOccurred())
			// we don't expect to see this in practice, but better safe than sorry
			Expect(defaulter.Default(ctx, &tektondevv1.Pipeline{})).
				Error().
				To(And(
					Satisfy(errors.IsBadRequest),
					MatchError(ContainSubstring("expected a PipelineRun object but got *v1.Pipeline"))))
		})
	})
})
