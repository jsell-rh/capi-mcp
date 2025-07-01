# E2E Testing Makefile for CAPI MCP Server

# Configuration
KIND_CLUSTER_NAME ?= capi-e2e
MCP_SERVER_NAMESPACE ?= capi-mcp-system
AWS_REGION ?= us-west-2
TIMEOUT ?= 30m

# Default target
.PHONY: help
help: ## Show this help message
	@echo "CAPI MCP Server E2E Testing"
	@echo ""
	@echo "Available targets:"
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Setup and Cleanup

.PHONY: setup
setup: ## Set up the complete E2E test environment
	@echo "Setting up E2E test environment..."
	./scripts/setup-cluster.sh
	./scripts/deploy-server.sh
	@echo "E2E environment setup complete!"

.PHONY: setup-cluster
setup-cluster: ## Set up kind cluster with CAPI components only
	@echo "Setting up kind cluster and CAPI components..."
	./scripts/setup-cluster.sh

.PHONY: deploy-server
deploy-server: ## Deploy MCP server to existing cluster
	@echo "Deploying MCP server..."
	./scripts/deploy-server.sh

.PHONY: cleanup
cleanup: ## Clean up the entire E2E test environment
	@echo "Cleaning up E2E test environment..."
	./scripts/cleanup.sh

.PHONY: cleanup-force
cleanup-force: ## Force cleanup without confirmation
	@echo "Force cleaning up E2E test environment..."
	FORCE_CLEANUP=true ./scripts/cleanup.sh

##@ Testing

.PHONY: test-e2e
test-e2e: ## Run all E2E tests
	@echo "Running E2E tests..."
	go test -v -timeout $(TIMEOUT) ./...

.PHONY: test-management
test-management: ## Run management cluster tests only
	@echo "Running management cluster tests..."
	go test -v -timeout $(TIMEOUT) -run TestManagementCluster ./...

.PHONY: test-tools
test-tools: ## Run tool validation tests only
	@echo "Running tool validation tests..."
	go test -v -timeout $(TIMEOUT) -run TestToolValidation ./...

.PHONY: test-lifecycle
test-lifecycle: ## Run cluster lifecycle tests only
	@echo "Running cluster lifecycle tests..."
	go test -v -timeout $(TIMEOUT) -run TestClusterLifecycle ./...

.PHONY: test-integration
test-integration: ## Run integration workflow tests only
	@echo "Running integration workflow tests..."
	go test -v -timeout $(TIMEOUT) -run TestIntegrationWorkflow ./...

##@ Development and Debugging

.PHONY: status
status: ## Show the current status of the E2E environment
	@echo "E2E Environment Status:"
	@echo "======================="
	@echo ""
	@echo "Kind cluster:"
	@kind get clusters | grep $(KIND_CLUSTER_NAME) || echo "  ❌ Kind cluster not found"
	@echo ""
	@echo "Kubernetes context:"
	@kubectl config current-context 2>/dev/null || echo "  ❌ No kubectl context"
	@echo ""
	@echo "CAPI system pods:"
	@kubectl get pods -n capi-system 2>/dev/null || echo "  ❌ CAPI system not accessible"
	@echo ""
	@echo "CAPA system pods:"
	@kubectl get pods -n capa-system 2>/dev/null || echo "  ❌ CAPA system not accessible"
	@echo ""
	@echo "MCP server pods:"
	@kubectl get pods -n $(MCP_SERVER_NAMESPACE) 2>/dev/null || echo "  ❌ MCP server not accessible"
	@echo ""
	@echo "CAPI clusters:"
	@kubectl get clusters --all-namespaces 2>/dev/null || echo "  ❌ Cannot list clusters"

.PHONY: logs
logs: ## Show logs from MCP server and CAPI components
	@echo "MCP Server logs:"
	@echo "================"
	@kubectl logs -n $(MCP_SERVER_NAMESPACE) -l app=capi-mcp-server --tail=50 || true
	@echo ""
	@echo "CAPI Controller logs:"
	@echo "===================="
	@kubectl logs -n capi-system -l cluster.x-k8s.io/provider=cluster-api --tail=20 || true
	@echo ""
	@echo "CAPA Controller logs:"
	@echo "===================="
	@kubectl logs -n capa-system -l cluster.x-k8s.io/provider=infrastructure-aws --tail=20 || true

.PHONY: debug
debug: ## Start interactive debugging session
	@echo "Starting debug session..."
	@echo "Available commands:"
	@echo "  kubectl get clusters --all-namespaces"
	@echo "  kubectl get clusterclasses"
	@echo "  kubectl describe cluster <name>"
	@echo "  curl -H 'Authorization: Bearer test-api-key' http://localhost:8080/health"
	@echo ""
	@bash

.PHONY: port-forward
port-forward: ## Set up port-forward to MCP server
	@echo "Setting up port-forward to MCP server..."
	@echo "Server will be available at: http://localhost:8080"
	@echo "Press Ctrl+C to stop"
	kubectl port-forward -n $(MCP_SERVER_NAMESPACE) service/capi-mcp-server 8080:8080

.PHONY: test-connection
test-connection: ## Test connection to MCP server
	@echo "Testing connection to MCP server..."
	@curl -s -H "Authorization: Bearer test-api-key" http://localhost:8080/health && echo "✅ Connection successful" || echo "❌ Connection failed"

##@ Prerequisites

.PHONY: check-prereqs
check-prereqs: ## Check if all prerequisites are installed
	@echo "Checking prerequisites..."
	@command -v kind >/dev/null 2>&1 || { echo "❌ kind is not installed"; exit 1; }
	@command -v kubectl >/dev/null 2>&1 || { echo "❌ kubectl is not installed"; exit 1; }
	@command -v clusterctl >/dev/null 2>&1 || { echo "❌ clusterctl is not installed"; exit 1; }
	@command -v docker >/dev/null 2>&1 || { echo "❌ docker is not installed"; exit 1; }
	@command -v go >/dev/null 2>&1 || { echo "❌ go is not installed"; exit 1; }
	@docker info >/dev/null 2>&1 || { echo "❌ docker is not running"; exit 1; }
	@echo "✅ All prerequisites are installed and running"

.PHONY: install-prereqs
install-prereqs: ## Install prerequisites (Linux only)
	@echo "Installing prerequisites..."
	@echo "Installing kind..."
	@go install sigs.k8s.io/kind@latest
	@echo "Installing clusterctl..."
	@curl -L https://github.com/kubernetes-sigs/cluster-api/releases/download/v1.6.0/clusterctl-linux-amd64 -o /tmp/clusterctl
	@chmod +x /tmp/clusterctl
	@sudo mv /tmp/clusterctl /usr/local/bin/
	@echo "✅ Prerequisites installed"

##@ Configuration

.PHONY: configure-aws
configure-aws: ## Configure AWS credentials for testing
	@echo "Configuring AWS credentials..."
	@echo "Please ensure the following environment variables are set:"
	@echo "  AWS_ACCESS_KEY_ID"
	@echo "  AWS_SECRET_ACCESS_KEY"
	@echo "  AWS_REGION (optional, defaults to us-west-2)"
	@echo "  AWS_SSH_KEY_NAME (optional, for SSH access to instances)"
	@echo ""
	@echo "Current AWS configuration:"
	@aws sts get-caller-identity 2>/dev/null || echo "❌ AWS credentials not configured"

.PHONY: verify-aws
verify-aws: ## Verify AWS credentials and permissions
	@echo "Verifying AWS credentials and permissions..."
	@aws sts get-caller-identity || { echo "❌ AWS credentials invalid"; exit 1; }
	@aws ec2 describe-regions --region $(AWS_REGION) >/dev/null || { echo "❌ Cannot access EC2 in region $(AWS_REGION)"; exit 1; }
	@echo "✅ AWS credentials and permissions verified"

##@ Common Workflows

.PHONY: quick-test
quick-test: setup test-management cleanup ## Complete quick test workflow
	@echo "Quick test workflow completed"

.PHONY: full-test
full-test: setup test-e2e cleanup ## Complete full test workflow
	@echo "Full test workflow completed"

.PHONY: rebuild-server
rebuild-server: ## Rebuild and redeploy the MCP server
	@echo "Rebuilding and redeploying MCP server..."
	@cd ../../.. && make build
	./scripts/deploy-server.sh
	@echo "MCP server rebuilt and redeployed"

.PHONY: reset-environment
reset-environment: cleanup setup ## Reset the entire test environment
	@echo "Test environment reset complete"

##@ Variables
.PHONY: show-config
show-config: ## Show current configuration
	@echo "Current E2E Test Configuration:"
	@echo "==============================="
	@echo "KIND_CLUSTER_NAME: $(KIND_CLUSTER_NAME)"
	@echo "MCP_SERVER_NAMESPACE: $(MCP_SERVER_NAMESPACE)"
	@echo "AWS_REGION: $(AWS_REGION)"
	@echo "TIMEOUT: $(TIMEOUT)"
	@echo "AWS_ACCESS_KEY_ID: $(if $(AWS_ACCESS_KEY_ID),✅ Set,❌ Not set)"
	@echo "AWS_SECRET_ACCESS_KEY: $(if $(AWS_SECRET_ACCESS_KEY),✅ Set,❌ Not set)"
	@echo "AWS_SSH_KEY_NAME: $(if $(AWS_SSH_KEY_NAME),$(AWS_SSH_KEY_NAME),❌ Not set)"