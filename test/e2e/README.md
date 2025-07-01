# End-to-End Testing Framework

This directory contains the End-to-End (E2E) testing framework for the CAPI MCP Server, designed to validate all seven MCP tools against a real Cluster API environment.

## Overview

The E2E testing framework creates a complete testing environment that includes:

1. **Management Cluster**: A local Kubernetes cluster running in kind (Kubernetes-in-Docker)
2. **CAPI Components**: Core Cluster API controllers and AWS Provider (CAPA) installed in the management cluster
3. **MCP Server**: The CAPI MCP Server deployed and running against the management cluster
4. **Workload Clusters**: Real AWS infrastructure provisioned through CAPI for testing

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    E2E Test Environment                     │
├─────────────────────────────────────────────────────────────┤
│  ┌─────────────────┐    ┌─────────────────────────────────┐ │
│  │   Test Runner   │───▶│        MCP Server              │ │
│  │  (Go test)      │    │  (deployed in kind cluster)   │ │
│  └─────────────────┘    └─────────────────────────────────┘ │
│           │                           │                      │
│           │              ┌─────────────────────────────────┐ │
│           │              │     Management Cluster          │ │
│           │              │      (kind cluster)             │ │
│           │              │                                 │ │
│           │              │  ┌─────────────────────────────┐ │ │
│           │              │  │    CAPI Controllers         │ │ │
│           │              │  │  - cluster-api-core         │ │ │
│           │              │  │  - capa-controller          │ │ │
│           │              │  └─────────────────────────────┘ │ │
│           │              │                                 │ │
│           │              │  ┌─────────────────────────────┐ │ │
│           │              │  │    CAPI Resources           │ │ │
│           │              │  │  - ClusterClasses           │ │ │
│           │              │  │  - Clusters                 │ │ │
│           │              │  │  - MachineDeployments       │ │ │
│           │              │  └─────────────────────────────┘ │ │
│           │              └─────────────────────────────────┘ │
│           │                           │                      │
│           └───────────────────────────┼──────────────────────┘
│                                       │
│              ┌─────────────────────────▼──────────────────────┐
│              │              AWS Cloud                        │
│              │                                               │
│              │  ┌─────────────────────────────────────────┐  │
│              │  │           Workload Clusters             │  │
│              │  │  - EC2 instances                        │  │
│              │  │  - VPCs, subnets, security groups      │  │
│              │  │  - ELBs for control plane              │  │
│              │  └─────────────────────────────────────────┘  │
│              └───────────────────────────────────────────────┘
```

## Test Categories

### 1. Management Cluster Tests
- Verify kind cluster creation and CAPI installation
- Test MCP server deployment and health checks
- Validate RBAC and authentication setup

### 2. Tool Validation Tests  
- Test all seven MCP tools individually
- Validate tool input/output schemas
- Test error handling and edge cases

### 3. Cluster Lifecycle Tests
- End-to-end cluster creation with `create_cluster`
- Cluster scaling with `scale_cluster`  
- Cluster deletion with `delete_cluster`
- Kubeconfig retrieval and node listing

### 4. Integration Workflow Tests
- Multi-step workflows combining multiple tools
- Concurrent operations testing
- Failure recovery scenarios

## Prerequisites

### Required Tools
- **kind** (v0.20.0+): For local Kubernetes clusters
- **kubectl** (v1.28+): Kubernetes CLI
- **clusterctl** (v1.6.0+): CAPI CLI tool
- **Docker** (v20.10+): Container runtime
- **Go** (v1.23+): For running tests

### AWS Prerequisites
- AWS account with appropriate permissions
- AWS credentials configured (via `~/.aws/credentials` or environment variables)
- Key pair created in target AWS region for SSH access

### Environment Variables
```bash
# AWS Configuration
export AWS_REGION=us-west-2
export AWS_ACCESS_KEY_ID=your-access-key
export AWS_SECRET_ACCESS_KEY=your-secret-key
export AWS_SSH_KEY_NAME=your-key-pair-name

# Test Configuration
export E2E_CLEANUP_AWS_RESOURCES=true
export E2E_TEST_TIMEOUT=30m
export E2E_WORKLOAD_CLUSTER_NAME=e2e-test-cluster
```

## Quick Start

### 1. Install Prerequisites
```bash
# Install kind
go install sigs.k8s.io/kind@latest

# Install clusterctl
curl -L https://github.com/kubernetes-sigs/cluster-api/releases/download/v1.6.0/clusterctl-linux-amd64 -o clusterctl
chmod +x clusterctl && sudo mv clusterctl /usr/local/bin/

# Verify installations
kind --version
clusterctl version
kubectl version --client
```

### 2. Run E2E Tests
```bash
# Run all E2E tests
make test-e2e

# Or run specific test suites
go test ./test/e2e -v -run TestManagementCluster
go test ./test/e2e -v -run TestToolValidation  
go test ./test/e2e -v -run TestClusterLifecycle
```

### 3. Manual Environment Setup (for debugging)
```bash
# Create kind cluster and install CAPI
./test/e2e/scripts/setup-cluster.sh

# Deploy MCP server
./test/e2e/scripts/deploy-server.sh

# Run individual tests
go test ./test/e2e -v -run TestCreateCluster
```

## Test Structure

```
test/e2e/
├── README.md                    # This file
├── main_test.go                 # Test suite setup and teardown
├── management_cluster_test.go   # Management cluster validation
├── tool_validation_test.go      # Individual tool testing
├── cluster_lifecycle_test.go    # End-to-end cluster workflows
├── integration_workflow_test.go # Complex multi-tool scenarios
├── scripts/
│   ├── setup-cluster.sh        # Kind cluster and CAPI setup
│   ├── deploy-server.sh        # MCP server deployment
│   ├── cleanup.sh              # Test environment cleanup
│   └── wait-for-ready.sh       # Wait for resources to be ready
├── manifests/
│   ├── capi-components.yaml    # CAPI core components
│   ├── capa-components.yaml    # AWS provider components
│   ├── clusterclass.yaml       # Test ClusterClass definition
│   └── server-deployment.yaml  # MCP server deployment
└── utils/
    ├── cluster.go              # Cluster management utilities
    ├── mcp_client.go           # MCP client helpers
    └── aws.go                  # AWS resource validation
```

## Configuration

### ClusterClass Template
The E2E tests use a standardized ClusterClass template that defines:
- **Control Plane**: Single-node kubeadm control plane
- **Workers**: Configurable number of worker nodes (default: 2)
- **Networking**: Calico CNI
- **Instance Types**: t3.medium for control plane, t3.small for workers
- **AMI**: Latest Ubuntu 20.04 LTS

### Test Timeouts
- **Cluster Creation**: 15 minutes
- **Cluster Deletion**: 10 minutes  
- **Node Scaling**: 5 minutes
- **Management Operations**: 30 seconds

## Troubleshooting

### Common Issues

1. **Kind cluster creation fails**
   ```bash
   # Check Docker status
   docker ps
   
   # Recreate cluster
   kind delete cluster --name capi-e2e
   kind create cluster --name capi-e2e
   ```

2. **CAPI controllers not ready**
   ```bash
   # Check controller status
   kubectl get pods -n capi-system
   kubectl get pods -n capa-system
   
   # View controller logs
   kubectl logs -n capi-system deployment/capi-controller-manager
   ```

3. **AWS authentication issues**
   ```bash
   # Verify AWS credentials
   aws sts get-caller-identity
   
   # Check secret creation
   kubectl get secret -n capa-system capa-manager-bootstrap-credentials
   ```

4. **Workload cluster stuck in provisioning**
   ```bash
   # Check cluster status
   kubectl get clusters
   kubectl describe cluster e2e-test-cluster
   
   # Check AWS resources
   aws ec2 describe-instances --region us-west-2
   ```

### Cleanup
```bash
# Cleanup everything
make clean-e2e

# Manual cleanup
./test/e2e/scripts/cleanup.sh
kind delete cluster --name capi-e2e
```

## Development

### Adding New Tests
1. Create test file in appropriate category (`*_test.go`)
2. Use provided utilities in `utils/` package
3. Follow naming convention: `TestFeatureName`
4. Add cleanup logic in test teardown

### Testing Against Different Providers
The framework is designed to be provider-agnostic. To test against different providers:
1. Update `manifests/` with provider-specific components
2. Modify `ClusterClass` for provider requirements  
3. Update AWS utilities in `utils/aws.go` for provider-specific validation

## Security Considerations

- E2E tests create real AWS resources that incur costs
- Always enable cleanup (`E2E_CLEANUP_AWS_RESOURCES=true`)
- Use dedicated AWS account for testing
- Set appropriate resource limits and timeouts
- Never commit AWS credentials to version control

## Performance Benchmarks

The E2E tests validate the performance criteria from the roadmap:
- **Read Operations**: `list_clusters`, `get_cluster` < 500ms
- **Acknowledgment**: Long-running operations return within 1s
- **Cluster Creation**: Complete within 15 minutes
- **Scaling Operations**: Complete within 5 minutes