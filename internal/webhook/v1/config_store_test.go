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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Config Store ", func() {
	Context("Loading Config Store", func() {
		It("When QueueName is Set", func(ctx context.Context) {
			configData := "queueName: test-queue"
			cfgStore := &ConfigStore{}
			err := cfgStore.Update([]byte(configData))
			Expect(err).NotTo(HaveOccurred())

			cfg, mutators := cfgStore.GetConfigAndMutators()
			Expect(mutators).To(BeEmpty())
			Expect(cfg.QueueName).To(Equal("test-queue"))
			Expect(cfg.MultiKueueOverride).To(BeFalse())

		})
		It("When QueueName is Not Set", func(ctx context.Context) {
			configData := ""
			cfgStore := &ConfigStore{}
			err := cfgStore.Update([]byte(configData))
			Expect(err).To(HaveOccurred())
		})
	})

	Context("Test MultiKueueOverride", func() {
		It("When MultiKueueOverride is Set", func(ctx context.Context) {
			configData := "queueName: test-queue\nmultiKueueOverride: true "
			cfgStore := &ConfigStore{}
			err := cfgStore.Update([]byte(configData))
			Expect(err).NotTo(HaveOccurred())

			cfg, mutators := cfgStore.GetConfigAndMutators()
			Expect(mutators).To(BeEmpty())
			Expect(cfg.QueueName).To(Equal("test-queue"))
			Expect(cfg.MultiKueueOverride).To(BeTrue())

		})

		It("When MultiKueueOverride is not Set", func(ctx context.Context) {
			configData := "queueName: test-queue"
			cfgStore := &ConfigStore{}
			err := cfgStore.Update([]byte(configData))
			Expect(err).NotTo(HaveOccurred())

			cfg, mutators := cfgStore.GetConfigAndMutators()
			Expect(mutators).To(BeEmpty())
			Expect(cfg.QueueName).To(Equal("test-queue"))
			Expect(cfg.MultiKueueOverride).To(BeFalse())

		})
	})

	Context("Test CEL", func() {
		configData := "queueName: pipelines-queue\ncel:\n  expressions:\n    - priority(\"tekton-kueue-default\")\n"
		It("When CEL is set", func(ctx context.Context) {
			cfgStore := &ConfigStore{}
			err := cfgStore.Update([]byte(configData))
			Expect(err).NotTo(HaveOccurred())
			cfg, mutators := cfgStore.GetConfigAndMutators()
			Expect(mutators).NotTo(BeEmpty())
			Expect(cfg.QueueName).To(Equal("pipelines-queue"))
			Expect(cfg.MultiKueueOverride).To(BeFalse())

		})
		It("Invalid CEL Configuration", func(ctx context.Context) {
			configData := "queueName: pipelines-queue\ncel:\n  expressions:\n    - invalid_priority(\"tekton-kueue-default\")\n"
			cfgStore := &ConfigStore{}
			err := cfgStore.Update([]byte(configData))
			Expect(err).To(HaveOccurred())
		})
	})

	Context("Invalid Configuration", func() {
		configData := "Random Config	 "
		It("Invalid Tekton-Kueue Configuration", func(ctx context.Context) {
			cfgStore := &ConfigStore{}
			err := cfgStore.Update([]byte(configData))
			Expect(err).To(HaveOccurred())
		})
	})
})
