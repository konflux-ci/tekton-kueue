#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

# Number of workers to create, default to 1
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
ROOT="$(dirname "$SCRIPT_DIR")"

export KUBECONFIG=${KUBECONFIG:-/tmp/multikueue.kubeconfig}
export PATH=$PATH:$(go env GOPATH)/bin


#Setup MultiKueue Environment
#source $ROOT/hack/01-setup-multikueue.sh

go install github.com/onsi/ginkgo/v2/ginkgo@latest
ginkgo -v run $ROOT/test/e2e-multikueue


