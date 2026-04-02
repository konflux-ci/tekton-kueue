package common

import (
	"fmt"
	"os"
	"strings"
)

const namespaceFile = "/var/run/secrets/kubernetes.io/serviceaccount/namespace"

func GetCurrentNamespace() (string, error) {
	return readNamespace(namespaceFile)
}

func readNamespace(path string) (string, error) {
	bytes, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("not able to read namespace file %s: %w", path, err)
	}
	return strings.TrimSpace(string(bytes)), nil
}
