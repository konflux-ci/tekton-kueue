#!/usr/bin/env bash

# This script sets up a MultiKueue environment with one manager and a specified number of workers.

set -o errexit
set -o nounset
set -o pipefail

# Number of workers to create, default to 1
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
ROOT="$(dirname "$SCRIPT_DIR")"

NUM_WORKERS=${1:-1}
TEKTON_MANIFEST_URL="https://infra.tekton.dev/tekton-releases/pipeline/latest/release.yaml"
KUEUE_MANIFEST_URL="https://gist.githubusercontent.com/khrm/a83998529449ae0f0e25c264d4e61dd0/raw/bd7933eea4b509996dbe7a4739ff96dd2101b0e3/gistfile0.txt"
MULTIKUEUE_MANIFEST_URL="https://gist.githubusercontent.com/khrm/4a022f27a97c5f1456cdc05a64885860/raw/ba1b9ae77b55ac3319de207167ab4590eb78bb0a/gistfile0.txt"
CERT_MANAGER_URL="https://github.com/cert-manager/cert-manager/releases/download/v1.16.3/cert-manager.yaml"

TEMP_DIR="/tmp/tekton-kueue/e2e/multikueue"
mkdir -p ${TEMP_DIR}
export KUBECONFIG=${KUBECONFIG:-$TEMP_DIR/multikueue.kubeconfig}

function create_cluster() {
    cluserName=$1
    shift
    if kind get clusters | grep -q "^${cluserName}$"; then
        echo "  ✅ Cluster $cluserName already exists. Continuing with the next command."
        kind export kubeconfig --name $cluserName
        return
    else
        echo "Cluster $cluserName does not exist. Halting script or creating it."
        kind create cluster --name=$cluserName $@
    fi

    kubectl config use-context kind-$cluserName

    echo "Installing tekton and cert-manager"
    make tekton cert-manager > /dev/null

    echo "Waiting for cert-manager to be ready..."
    kubectl wait --for=condition=Available deployment --all -n cert-manager --timeout=300s

    echo "Waiting for Tekton Pipelines to be ready..."
    kubectl wait --for=condition=Available deployment --all -n tekton-pipelines --timeout=300s

    echo "Installing Kueue controller on $cluserName..."
    kubectl apply --server-side -f ${KUEUE_MANIFEST_URL} > /dev/null

    echo "Waiting for Kueue to be ready..."
    kubectl wait --for=condition=Available deployment --all -n kueue-system --timeout=300s

    kubectl get po,svc -n kueue-system --show-labels

    echo "Cluster $cluserName is ready"

}



# Function to set up the manager cluster
setup_hub_cluster() {
  cluserName=$1
  echo "Creating $cluserName cluster..."
  create_cluster $cluserName

  kubectl config get-contexts
  sleep 10 # Sleep some time so Kueue Webhook comes online
  echo "Installing MultiKueue controller on $cluserName..."
  kubectl apply --server-side --force-conflicts -f ${MULTIKUEUE_MANIFEST_URL}

  echo "Waiting for Tekton-Kueue to be ready..."
  kubectl wait --for=condition=Available deployment --all -n tekton-kueue --timeout=300s
  kubectl get deployment -n tekton-kueue

  echo "Apply MultiKueue Setup"
  kubectl apply --server-side -f $ROOT/config/samples/multikueue/multikueue-resources.yaml 

}

# Function to create a kubeconfig for a worker
create_worker_kubeconfig() {
    local worker_name=$1
    local kubeconfig_out="$TEMP_DIR/${worker_name}.kubeconfig"
    local multikueue_sa="multikueue-sa"
    local namespace="kueue-system"

    kubectl config use-context "kind-${worker_name}"

    echo "Creating RBAC for multikueue service account on ${worker_name}..."
    kubectl apply -f - <<EOF
apiVersion: v1
kind: ServiceAccount
metadata:
  name: ${multikueue_sa}
  namespace: ${namespace}
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: ${multikueue_sa}-role
rules:
- apiGroups: [""]
  resources: ["pods"]
  verbs: ["get", "list", "watch", "create", "delete"]
- apiGroups: ["batch"]
  resources: ["jobs"]
  verbs: ["get", "list", "watch", "create", "delete"]
- apiGroups: ["kueue.x-k8s.io"]
  resources: ["workloads", "workloads/status"]
  verbs: ["get", "list", "watch", "create", "delete", "patch", "update"]
- apiGroups: ["tekton.dev"]
  resources: ["pipelineruns", "pipelineruns/status"]
  verbs: ["get", "list", "watch", "create", "delete", "patch", "update"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: ${multikueue_sa}-crb
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: ${multikueue_sa}-role
subjects:
- kind: ServiceAccount
  name: ${multikueue_sa}
  namespace: ${namespace}
EOF

    local sa_secret_name
    sa_secret_name=$(kubectl get -n ${namespace} sa/${multikueue_sa} -o "jsonpath={.secrets[0]..name}")
    if [ -z "$sa_secret_name" ]; then
        kubectl apply -f - <<EOF
apiVersion: v1
kind: Secret
type: kubernetes.io/service-account-token
metadata:
  name: ${multikueue_sa}
  namespace: ${namespace}
  annotations:
    kubernetes.io/service-account.name: "${multikueue_sa}"
EOF
        sa_secret_name=${multikueue_sa}
    fi

    local sa_token
    sa_token=$(kubectl get -n ${namespace} "secrets/${sa_secret_name}" -o "jsonpath={.data['token']}" | base64 -d)
    local ca_cert
    ca_cert=$(kubectl get -n ${namespace} "secrets/${sa_secret_name}" -o "jsonpath={.data['ca\.crt']}")
    local current_context
    current_context=$(kubectl config current-context)
    local current_cluster
    current_cluster=$(kubectl config view -o jsonpath="{.contexts[?(@.name == \"${current_context}\")].context.cluster}")

    local current_cluster_addr="https://${worker_name}-control-plane:6443"

    echo "Writing kubeconfig in ${kubeconfig_out}"
    cat > "${kubeconfig_out}" <<EOF
apiVersion: v1
clusters:
- cluster:
    certificate-authority-data: ${ca_cert}
    server: ${current_cluster_addr}
  name: ${current_cluster}
contexts:
- context:
    cluster: ${current_cluster}
    user: ${current_cluster}-${multikueue_sa}
  name: ${current_context}
current-context: ${current_context}
kind: Config
preferences: {}
users:
- name: ${current_cluster}-${multikueue_sa}
  user:
    token: ${sa_token}
EOF
}

# Function to set up a spoke cluster
setup_spoke_cluster() {
  local cluserName=$1
  echo "Creating worker cluster ${cluserName}..."
  create_cluster $cluserName

  #Apply Worker Setup
  kubectl apply -f $ROOT/config/samples/kueue/kueue-resources.yaml
  create_worker_kubeconfig $cluserName
}

function add_spoke_to_hub() {
    local spoke=$1
    local kubeconfig_out="$TEMP_DIR/${spoke}.kubeconfig"

    hub_context="kind-hub"
    echo "Adding Spoke $spoke into $hub_context"
    kubectl config use-context $hub_context
    kubectl --context=$hub_context create secret generic "${spoke}-secret" -n kueue-system --from-file=kubeconfig=${kubeconfig_out} --dry-run=client -o yaml | kubectl apply -f -

    # Add Spoke into MultiKueueCluster Config

    # Create MultiKueueCluster
    kubectl --context=$hub_context  apply -f - << EOF
      apiVersion: kueue.x-k8s.io/v1beta1
      kind: MultiKueueCluster
      metadata:
        name: $spoke
      spec:
        kubeConfig:
          locationType: Secret
          location: $spoke-secret
EOF


#kubectl get multikueueconfig  multikueue-test -o yaml | \
#  yq  ".spec.clusters += \"$spoke\" | .spec.clusters |=unique " | \
#  kubectl apply -f -

kubectl get multikueueconfig multikueue-test -o yaml | \
  yq '.spec.clusters = (.spec.clusters // []) |
      .spec.clusters += ["'$spoke'"] |
      .spec.clusters |= unique' | \
  kubectl apply -f -

kubectl get multikueueconfig multikueue-test -o jsonpath="{.spec}" | jq


}

function validate() {
   spokes=$1
    kubectl config use-context kind-hub
    sleep 10 # Give some time for controllers to reconcile

    kubectl get clusterqueues cluster-queue -o jsonpath="{.kind} - {'\t'}{.metadata.name} - {'\t'} {range .status.conditions[?(@.type == 'Active')]}{'CQ - Active: '}{@.status}{' Reason: '}{@.reason}{' Message: '}{@.message}{'\n'}{end}"
    kubectl get admissionchecks sample-multikueue -o jsonpath="{.kind} - {'\t'}{.metadata.name} - {'\t'} {range .status.conditions[?(@.type == 'Active')]}{'AC - Active: '}{@.status}{' Reason: '}{@.reason}{' Message: '}{@.message}{'\n'}{end}"
    for key in ${spokes[@]} ; do
      kubectl get multikueuecluster $key -o jsonpath="{.kind} - {'\t'}{.metadata.name} - {'\t'} {range .status.conditions[?(@.type == 'Active')]}{'MC - Active: '}{@.status}{' Reason: '}{@.reason}{' Message: '}{@.message}{'\n'}{end}"
    done
}

function main() {

  # Setup Hub Cluster
  setup_hub_cluster "hub"
  echo "##########  Hub is Ready"
  # Setup  Spoke Clusters
  spokes=()
  for i in $(seq 1 "${NUM_WORKERS}"); do
    local cluserName="spoke-${i}"
    setup_spoke_cluster $cluserName
    add_spoke_to_hub $cluserName
    spokes+=($cluserName)
    echo "Validate Newly added spoke"
    validate $cluserName
  done

  echo "Setup complete. Verifying..."
  validate $spokes
}

main

