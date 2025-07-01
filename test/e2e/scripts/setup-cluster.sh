#!/bin/bash

# setup-cluster.sh
# Sets up a kind cluster with CAPI components for E2E testing

set -euo pipefail

# Configuration
KIND_CLUSTER_NAME="${KIND_CLUSTER_NAME:-capi-e2e}"
CLUSTER_API_VERSION="${CLUSTER_API_VERSION:-v1.6.0}"
CAPA_VERSION="${CAPA_VERSION:-v2.3.0}"
KUBERNETES_VERSION="${KUBERNETES_VERSION:-v1.28.0}"
SCRIPTS_DIR="${SCRIPTS_DIR:-$(dirname "$0")}"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

log() {
    echo -e "${BLUE}[$(date +'%Y-%m-%d %H:%M:%S')] $1${NC}"
}

warn() {
    echo -e "${YELLOW}[$(date +'%Y-%m-%d %H:%M:%S')] WARNING: $1${NC}"
}

error() {
    echo -e "${RED}[$(date +'%Y-%m-%d %H:%M:%S')] ERROR: $1${NC}"
}

success() {
    echo -e "${GREEN}[$(date +'%Y-%m-%d %H:%M:%S')] $1${NC}"
}

# Check prerequisites
check_prerequisites() {
    log "Checking prerequisites..."
    
    # Check if kind is installed
    if ! command -v kind &> /dev/null; then
        error "kind is not installed. Please install kind: https://kind.sigs.k8s.io/docs/user/quick-start/#installation"
        exit 1
    fi
    
    # Check if kubectl is installed
    if ! command -v kubectl &> /dev/null; then
        error "kubectl is not installed. Please install kubectl: https://kubernetes.io/docs/tasks/tools/"
        exit 1
    fi
    
    # Check if clusterctl is installed
    if ! command -v clusterctl &> /dev/null; then
        error "clusterctl is not installed. Please install clusterctl: https://cluster-api.sigs.k8s.io/user/quick-start.html#install-clusterctl"
        exit 1
    fi
    
    # Check if docker is running
    if ! docker info &> /dev/null; then
        error "Docker is not running. Please start Docker."
        exit 1
    fi
    
    success "Prerequisites check passed"
}

# Create kind cluster configuration
create_kind_config() {
    local config_file="/tmp/kind-config-${KIND_CLUSTER_NAME}.yaml"
    
    log "Creating kind cluster configuration..." >&2
    
    cat > "$config_file" <<EOF
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
name: ${KIND_CLUSTER_NAME}
nodes:
- role: control-plane
  image: kindest/node:${KUBERNETES_VERSION}
  kubeadmConfigPatches:
  - |
    kind: InitConfiguration
    nodeRegistration:
      kubeletExtraArgs:
        node-labels: "ingress-ready=true"
  extraPortMappings:
  - containerPort: 80
    hostPort: 80
    protocol: TCP
  - containerPort: 443
    hostPort: 443
    protocol: TCP
  - containerPort: 8080
    hostPort: 8080
    protocol: TCP
EOF
    
    echo "$config_file"
}

# Create or verify kind cluster
setup_kind_cluster() {
    log "Setting up kind cluster: ${KIND_CLUSTER_NAME}"
    
    # Check if cluster already exists
    if kind get clusters | grep -q "^${KIND_CLUSTER_NAME}$"; then
        warn "Kind cluster ${KIND_CLUSTER_NAME} already exists"
        
        # Verify it's healthy
        if kubectl cluster-info --context "kind-${KIND_CLUSTER_NAME}" &> /dev/null; then
            success "Existing kind cluster is healthy"
            return 0
        else
            warn "Existing cluster is unhealthy, recreating..."
            kind delete cluster --name "${KIND_CLUSTER_NAME}"
        fi
    fi
    
    # Create new cluster
    local config_file
    config_file=$(create_kind_config)
    
    log "Creating new kind cluster..."
    kind create cluster --config "$config_file"
    
    # Clean up config file
    rm -f "$config_file"
    
    # Wait for cluster to be ready
    log "Waiting for cluster to be ready..."
    kubectl wait --for=condition=Ready nodes --all --timeout=300s
    
    success "Kind cluster ${KIND_CLUSTER_NAME} is ready"
}

# Initialize cluster API
initialize_cluster_api() {
    log "Initializing Cluster API..."
    
    # Set the kubectl context
    kubectl config use-context "kind-${KIND_CLUSTER_NAME}"
    
    # Initialize core Cluster API components
    log "Installing core Cluster API components..."
    clusterctl init --wait-providers
    
    success "Core Cluster API components installed"
}

# Install AWS provider (CAPA)
install_aws_provider() {
    log "Installing AWS provider (CAPA)..."
    
    # Check if AWS credentials are available
    if [[ -z "${AWS_ACCESS_KEY_ID:-}" ]] || [[ -z "${AWS_SECRET_ACCESS_KEY:-}" ]]; then
        warn "AWS credentials not found in environment"
        warn "Some E2E tests may fail without AWS credentials"
        warn "Set AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY environment variables"
    else
        log "AWS credentials found, creating secret..."
        
        # Create AWS credentials secret
        kubectl create namespace capa-system --dry-run=client -o yaml | kubectl apply -f -
        
        kubectl create secret generic capa-manager-bootstrap-credentials \
            --namespace capa-system \
            --from-literal AccessKeyID="${AWS_ACCESS_KEY_ID}" \
            --from-literal SecretAccessKey="${AWS_SECRET_ACCESS_KEY}" \
            --dry-run=client -o yaml | kubectl apply -f -
    fi
    
    # Install AWS provider
    clusterctl init --infrastructure aws --wait-providers
    
    success "AWS provider (CAPA) installed"
}

# Apply CAPI CRDs and wait for them to be established
wait_for_capi_crds() {
    log "Waiting for CAPI CRDs to be established..."
    
    local crds=(
        "clusters.cluster.x-k8s.io"
        "clusterclasses.cluster.x-k8s.io"
        "machinedeployments.cluster.x-k8s.io"
        "machines.cluster.x-k8s.io"
        "awsclusters.infrastructure.cluster.x-k8s.io"
        "awsmachines.infrastructure.cluster.x-k8s.io"
    )
    
    for crd in "${crds[@]}"; do
        log "Waiting for CRD: ${crd}"
        kubectl wait --for=condition=Established crd/${crd} --timeout=300s
    done
    
    success "All CAPI CRDs are established"
}

# Verify CAPI installation
verify_capi_installation() {
    log "Verifying CAPI installation..."
    
    # Check that all pods are running
    log "Checking CAPI system pods..."
    kubectl wait --for=condition=Ready pods --all -n capi-system --timeout=300s
    
    log "Checking CAPA system pods..."
    kubectl wait --for=condition=Ready pods --all -n capa-system --timeout=300s
    
    # Verify clusterctl can see the providers
    log "Verifying clusterctl providers..."
    clusterctl describe provider cluster-api --show-pods
    clusterctl describe provider infrastructure-aws --show-pods
    
    success "CAPI installation verified"
}

# Apply test ClusterClass
apply_test_clusterclass() {
    log "Applying test ClusterClass..."
    
    local manifests_dir
    manifests_dir="$(dirname "$SCRIPTS_DIR")/manifests"
    
    if [[ ! -f "${manifests_dir}/clusterclass.yaml" ]]; then
        warn "ClusterClass manifest not found at ${manifests_dir}/clusterclass.yaml"
        warn "Creating a basic ClusterClass for testing..."
        create_test_clusterclass "${manifests_dir}/clusterclass.yaml"
    fi
    
    kubectl apply -f "${manifests_dir}/clusterclass.yaml"
    
    # Wait for ClusterClass to be ready
    kubectl wait --for=condition=Ready clusterclass --all --timeout=60s || true
    
    success "Test ClusterClass applied"
}

# Create a basic test ClusterClass
create_test_clusterclass() {
    local output_file="$1"
    
    log "Creating basic test ClusterClass..."
    
    mkdir -p "$(dirname "$output_file")"
    
    cat > "$output_file" <<EOF
apiVersion: cluster.x-k8s.io/v1beta1
kind: ClusterClass
metadata:
  name: aws-test-cluster-class
  namespace: default
spec:
  controlPlane:
    ref:
      apiVersion: controlplane.cluster.x-k8s.io/v1beta1
      kind: KubeadmControlPlaneTemplate
      name: aws-test-control-plane
  infrastructure:
    ref:
      apiVersion: infrastructure.cluster.x-k8s.io/v1beta2
      kind: AWSClusterTemplate
      name: aws-test-cluster
  workers:
    machineDeployments:
    - class: default-worker
      template:
        bootstrap:
          ref:
            apiVersion: bootstrap.cluster.x-k8s.io/v1beta1
            kind: KubeadmConfigTemplate
            name: aws-test-worker-bootstrap
        infrastructure:
          ref:
            apiVersion: infrastructure.cluster.x-k8s.io/v1beta2
            kind: AWSMachineTemplate
            name: aws-test-worker
  variables:
  - name: region
    required: true
    schema:
      openAPIV3Schema:
        type: string
        default: us-west-2
  - name: instanceType
    required: false
    schema:
      openAPIV3Schema:
        type: string
        default: t3.small
---
apiVersion: infrastructure.cluster.x-k8s.io/v1beta2
kind: AWSClusterTemplate
metadata:
  name: aws-test-cluster
  namespace: default
spec:
  template:
    spec:
      region: us-west-2
      sshKeyName: ""
---
apiVersion: controlplane.cluster.x-k8s.io/v1beta1
kind: KubeadmControlPlaneTemplate
metadata:
  name: aws-test-control-plane
  namespace: default
spec:
  template:
    spec:
      kubeadmConfigSpec:
        initConfiguration:
          nodeRegistration:
            name: '{{ ds.meta_data.local_hostname }}'
            kubeletExtraArgs:
              cloud-provider: aws
        joinConfiguration:
          nodeRegistration:
            name: '{{ ds.meta_data.local_hostname }}'
            kubeletExtraArgs:
              cloud-provider: aws
---
apiVersion: infrastructure.cluster.x-k8s.io/v1beta2
kind: AWSMachineTemplate
metadata:
  name: aws-test-worker
  namespace: default
spec:
  template:
    spec:
      instanceType: t3.small
      iamInstanceProfile: nodes.cluster-api-provider-aws.sigs.k8s.io
      cloudInit:
        insecureSkipSecretsManager: true
---
apiVersion: bootstrap.cluster.x-k8s.io/v1beta1
kind: KubeadmConfigTemplate
metadata:
  name: aws-test-worker-bootstrap
  namespace: default
spec:
  template:
    spec:
      joinConfiguration:
        nodeRegistration:
          name: '{{ ds.meta_data.local_hostname }}'
          kubeletExtraArgs:
            cloud-provider: aws
EOF
}

# Main execution
main() {
    log "Starting E2E test cluster setup..."
    
    check_prerequisites
    setup_kind_cluster
    initialize_cluster_api
    install_aws_provider
    wait_for_capi_crds
    verify_capi_installation
    apply_test_clusterclass
    
    success "E2E test cluster setup completed successfully!"
    log "Cluster: ${KIND_CLUSTER_NAME}"
    log "Context: kind-${KIND_CLUSTER_NAME}"
    log ""
    log "Next steps:"
    log "  1. Deploy the MCP server: ./scripts/deploy-server.sh"
    log "  2. Run E2E tests: go test ./test/e2e -v"
}

# Run main function
main "$@"