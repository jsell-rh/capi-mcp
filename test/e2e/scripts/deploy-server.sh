#!/bin/bash

# deploy-server.sh
# Deploys the CAPI MCP Server to the kind cluster for E2E testing

set -euo pipefail

# Configuration
MCP_SERVER_NAMESPACE="${MCP_SERVER_NAMESPACE:-capi-mcp-system}"
MCP_SERVER_NAME="${MCP_SERVER_NAME:-capi-mcp-server}"
MCP_SERVER_IMAGE="${MCP_SERVER_IMAGE:-capi-mcp-server:latest}"
MCP_SERVER_PORT="${MCP_SERVER_PORT:-8080}"
API_KEY="${API_KEY:-test-api-key}"

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

# Build the MCP server Docker image
build_server_image() {
    log "Building MCP server Docker image..."
    
    # Get the project root directory
    local project_root
    project_root="$(cd "$(dirname "$0")/../../.." && pwd)"
    
    log "Project root: ${project_root}"
    
    # Build the Docker image
    cd "$project_root"
    
    if [[ ! -f "Dockerfile" ]]; then
        error "Dockerfile not found in project root"
        exit 1
    fi
    
    docker build -t "${MCP_SERVER_IMAGE}" .
    
    # Load the image into kind
    log "Loading image into kind cluster..."
    kind load docker-image "${MCP_SERVER_IMAGE}" --name "capi-e2e"
    
    success "MCP server image built and loaded"
}

# Create namespace for the MCP server
create_namespace() {
    log "Creating namespace: ${MCP_SERVER_NAMESPACE}"
    
    kubectl create namespace "${MCP_SERVER_NAMESPACE}" --dry-run=client -o yaml | kubectl apply -f -
    
    success "Namespace created"
}

# Create ServiceAccount and RBAC
create_rbac() {
    log "Creating ServiceAccount and RBAC..."
    
    cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: ServiceAccount
metadata:
  name: ${MCP_SERVER_NAME}
  namespace: ${MCP_SERVER_NAMESPACE}
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: ${MCP_SERVER_NAME}
rules:
# Cluster API permissions
- apiGroups: ["cluster.x-k8s.io"]
  resources: ["clusters", "clusterclasses", "machinedeployments", "machines"]
  verbs: ["get", "list", "create", "update", "patch", "delete", "watch"]
# AWS Infrastructure permissions  
- apiGroups: ["infrastructure.cluster.x-k8s.io"]
  resources: ["awsclusters", "awsmachines", "awsmachinetemplates", "awsclustertemplates"]
  verbs: ["get", "list", "create", "update", "patch", "delete", "watch"]
# Control plane permissions
- apiGroups: ["controlplane.cluster.x-k8s.io"]
  resources: ["kubeadmcontrolplanes", "kubeadmcontrolplanetemplates"]
  verbs: ["get", "list", "create", "update", "patch", "delete", "watch"]
# Bootstrap permissions
- apiGroups: ["bootstrap.cluster.x-k8s.io"]
  resources: ["kubeadmconfigs", "kubeadmconfigtemplates"]
  verbs: ["get", "list", "create", "update", "patch", "delete", "watch"]
# Core Kubernetes permissions
- apiGroups: [""]
  resources: ["secrets", "nodes"]
  verbs: ["get", "list", "watch"]
# Events for status updates
- apiGroups: [""]
  resources: ["events"]
  verbs: ["get", "list", "watch"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: ${MCP_SERVER_NAME}
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: ${MCP_SERVER_NAME}
subjects:
- kind: ServiceAccount
  name: ${MCP_SERVER_NAME}
  namespace: ${MCP_SERVER_NAMESPACE}
EOF
    
    success "RBAC created"
}

# Create configuration secret
create_config_secret() {
    log "Creating configuration secret..."
    
    kubectl create secret generic "${MCP_SERVER_NAME}-config" \
        --namespace "${MCP_SERVER_NAMESPACE}" \
        --from-literal="api-key=${API_KEY}" \
        --from-literal="log-level=info" \
        --from-literal="port=${MCP_SERVER_PORT}" \
        --dry-run=client -o yaml | kubectl apply -f -
    
    success "Configuration secret created"
}

# Deploy the MCP server
deploy_server() {
    log "Deploying MCP server..."
    
    cat <<EOF | kubectl apply -f -
apiVersion: apps/v1
kind: Deployment
metadata:
  name: ${MCP_SERVER_NAME}
  namespace: ${MCP_SERVER_NAMESPACE}
  labels:
    app: ${MCP_SERVER_NAME}
spec:
  replicas: 1
  selector:
    matchLabels:
      app: ${MCP_SERVER_NAME}
  template:
    metadata:
      labels:
        app: ${MCP_SERVER_NAME}
    spec:
      serviceAccountName: ${MCP_SERVER_NAME}
      containers:
      - name: server
        image: ${MCP_SERVER_IMAGE}
        imagePullPolicy: IfNotPresent
        ports:
        - containerPort: ${MCP_SERVER_PORT}
          name: http
        env:
        - name: API_KEY
          valueFrom:
            secretKeyRef:
              name: ${MCP_SERVER_NAME}-config
              key: api-key
        - name: LOG_LEVEL
          valueFrom:
            secretKeyRef:
              name: ${MCP_SERVER_NAME}-config
              key: log-level
        - name: PORT
          valueFrom:
            secretKeyRef:
              name: ${MCP_SERVER_NAME}-config
              key: port
        livenessProbe:
          httpGet:
            path: /health
            port: http
          initialDelaySeconds: 30
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /health
            port: http
          initialDelaySeconds: 5
          periodSeconds: 5
        resources:
          requests:
            memory: "128Mi"
            cpu: "100m"
          limits:
            memory: "512Mi"
            cpu: "500m"
---
apiVersion: v1
kind: Service
metadata:
  name: ${MCP_SERVER_NAME}
  namespace: ${MCP_SERVER_NAMESPACE}
  labels:
    app: ${MCP_SERVER_NAME}
spec:
  selector:
    app: ${MCP_SERVER_NAME}
  ports:
  - name: http
    port: 8080
    targetPort: http
    protocol: TCP
  type: ClusterIP
EOF
    
    success "MCP server deployed"
}

# Wait for deployment to be ready
wait_for_deployment() {
    log "Waiting for deployment to be ready..."
    
    kubectl wait --for=condition=available deployment/${MCP_SERVER_NAME} \
        --namespace ${MCP_SERVER_NAMESPACE} \
        --timeout=300s
    
    success "Deployment is ready"
}

# Verify deployment
verify_deployment() {
    log "Verifying deployment..."
    
    # Check pod status
    kubectl get pods -n ${MCP_SERVER_NAMESPACE} -l app=${MCP_SERVER_NAME}
    
    # Check service
    kubectl get service -n ${MCP_SERVER_NAMESPACE} ${MCP_SERVER_NAME}
    
    # Get pod logs
    log "Recent pod logs:"
    kubectl logs -n ${MCP_SERVER_NAMESPACE} -l app=${MCP_SERVER_NAME} --tail=20 || true
    
    success "Deployment verified"
}

# Setup port-forward for testing
setup_port_forward() {
    log "Setting up port-forward for testing..."
    
    # Kill any existing port-forward processes
    pkill -f "kubectl.*port-forward.*${MCP_SERVER_NAME}" || true
    
    # Start new port-forward in background
    kubectl port-forward -n ${MCP_SERVER_NAMESPACE} service/${MCP_SERVER_NAME} 8080:8080 &
    local pf_pid=$!
    
    # Wait a moment for port-forward to establish
    sleep 3
    
    # Test the connection
    if curl -s -f http://localhost:8080/health > /dev/null; then
        success "Port-forward established and server is responding"
        log "Server accessible at: http://localhost:8080"
        log "Port-forward PID: ${pf_pid}"
    else
        warn "Server may not be responding yet"
        log "Port-forward PID: ${pf_pid}"
    fi
}

# Create test API key secret for E2E tests
create_test_secret() {
    log "Creating test API key secret..."
    
    kubectl create secret generic e2e-test-config \
        --namespace ${MCP_SERVER_NAMESPACE} \
        --from-literal="api-key=${API_KEY}" \
        --from-literal="server-url=http://localhost:8080" \
        --dry-run=client -o yaml | kubectl apply -f -
    
    success "Test configuration secret created"
}

# Main execution
main() {
    log "Starting MCP server deployment..."
    
    # Check if we're in the right context
    local current_context
    current_context=$(kubectl config current-context)
    if [[ "$current_context" != "kind-capi-e2e" ]]; then
        warn "Current kubectl context is not 'kind-capi-e2e': ${current_context}"
        warn "This script is intended for E2E testing with kind"
        read -p "Continue anyway? (y/N): " -n 1 -r
        echo
        if [[ ! $REPLY =~ ^[Yy]$ ]]; then
            log "Aborted"
            exit 1
        fi
    fi
    
    build_server_image
    create_namespace
    create_rbac
    create_config_secret
    deploy_server
    wait_for_deployment
    verify_deployment
    create_test_secret
    setup_port_forward
    
    success "MCP server deployment completed successfully!"
    log ""
    log "Server Details:"
    log "  Namespace: ${MCP_SERVER_NAMESPACE}"
    log "  Service: ${MCP_SERVER_NAME}"
    log "  URL: http://localhost:8080"
    log "  API Key: ${API_KEY}"
    log ""
    log "Next steps:"
    log "  1. Test the server: curl -H 'Authorization: Bearer ${API_KEY}' http://localhost:8080/health"
    log "  2. Run E2E tests: go test ./test/e2e -v"
}

# Run main function
main "$@"