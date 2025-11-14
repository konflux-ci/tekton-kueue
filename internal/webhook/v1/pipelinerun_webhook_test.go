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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/errors"

	tektondevv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	// TODO (user): Add any additional imports if needed
)

var _ = Describe("PipelineRun Webhook", func() {
	var (
		obj       *tektondevv1.PipelineRun
		oldObj    *tektondevv1.PipelineRun
		defaulter pipelineRunCustomDefaulter
	)

	BeforeEach(func() {
		obj = &tektondevv1.PipelineRun{}
		oldObj = &tektondevv1.PipelineRun{}
		defaulter = pipelineRunCustomDefaulter{}
		Expect(defaulter).NotTo(BeNil(), "Expected defaulter to be initialized")
		Expect(oldObj).NotTo(BeNil(), "Expected oldObj to be initialized")
		Expect(obj).NotTo(BeNil(), "Expected obj to be initialized")
		// TODO (user): Add any setup logic common to all tests
	})

	AfterEach(func() {
		// TODO (user): Add any teardown logic common to all tests
	})

	Context("When creating PipelineRun under Defaulting Webhook", func() {
		It("Should report an error when serialization errors occur", func(ctx SpecContext) {
			obj.Spec.Params = tektondevv1.Params{
				{
					Name: "build-platforms",
					// intentionally leave off a value
				},
			}

			Expect(defaulter.Default(ctx, obj)).Error().To(Satisfy(errors.IsBadRequest))
		})
	})
})
