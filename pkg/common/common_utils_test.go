/*
Copyright 2026.

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

package common

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("readNamespace", func() {
	It("should return namespace from file", func() {
		tmpFile := filepath.Join(GinkgoT().TempDir(), "namespace")
		Expect(os.WriteFile(tmpFile, []byte("test-namespace"), 0644)).To(Succeed())

		Expect(readNamespace(tmpFile)).To(Equal("test-namespace"))
	})

	It("should trim whitespace and newlines from namespace", func() {
		tmpFile := filepath.Join(GinkgoT().TempDir(), "namespace")
		Expect(os.WriteFile(tmpFile, []byte("test-namespace\n"), 0644)).To(Succeed())

		Expect(readNamespace(tmpFile)).To(Equal("test-namespace"))
	})

	It("should return error when file does not exist", func() {
		ns, err := readNamespace("/nonexistent/path/namespace")
		Expect(err).To(MatchError(ContainSubstring("not able to read namespace file")))
		Expect(ns).To(BeEmpty())
	})
})
