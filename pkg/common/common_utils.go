package common

import (
	"errors"
	"os"
)

// namespaceFile is the path to the Kubernetes service account namespace file.
const namespaceFile = "/var/run/secrets/kubernetes.io/serviceaccount/namespace"

// GetCurrentNamespace reads the namespace from the mounted service account token.
// This is used by the ConfigMapReconciler to scope its watch to the deployment namespace.
func GetCurrentNamespace() (string, error) {
	bytes, err := os.ReadFile(namespaceFile)
	if err != nil {
		return "", errors.New("not able to read  namespace file: " + namespaceFile)
	}
	namespace := string(bytes)
	return namespace, nil
}
