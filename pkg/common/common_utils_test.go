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
	"testing"
)

func TestGetCurrentNamespace(t *testing.T) {
	t.Run("should return namespace from file", func(t *testing.T) {
		tmpDir := t.TempDir()
		tmpFile := filepath.Join(tmpDir, "namespace")
		if err := os.WriteFile(tmpFile, []byte("test-namespace"), 0644); err != nil {
			t.Fatalf("failed to write temp file: %v", err)
		}

		original := namespaceFile
		namespaceFile = tmpFile
		t.Cleanup(func() { namespaceFile = original })

		ns, err := GetCurrentNamespace()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if ns != "test-namespace" {
			t.Errorf("expected 'test-namespace', got %q", ns)
		}
	})

	t.Run("should return error when file does not exist", func(t *testing.T) {
		original := namespaceFile
		namespaceFile = "/nonexistent/path/namespace"
		t.Cleanup(func() { namespaceFile = original })

		ns, err := GetCurrentNamespace()
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if ns != "" {
			t.Errorf("expected empty namespace, got %q", ns)
		}
	})
}
