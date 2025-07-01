# End-to-End Testing Framework

This directory contains the comprehensive End-to-End (E2E) testing framework for the CAPI MCP Server, designed to validate all seven MCP tools against real Cluster API environments with stability and performance validation.

## Overview

The E2E testing framework creates a complete testing environment that includes:

1. **Management Cluster**: A local Kubernetes cluster running in kind (Kubernetes-in-Docker)
2. **CAPI Components**: Core Cluster API controllers and AWS Provider (CAPA) installed in the management cluster
3. **MCP Server**: The CAPI MCP Server deployed and running against the management cluster
4. **Workload Clusters**: Real AWS infrastructure provisioned through CAPI for testing
5. **Stability Testing**: Comprehensive reliability and error recovery validation
6. **Performance Benchmarking**: Validation against roadmap performance criteria

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

### 1. Management Cluster Tests (`management_cluster_test.go`)
- Verify kind cluster creation and CAPI installation
- Test MCP server deployment and health checks
- Validate RBAC and authentication setup
- Environment validation and connectivity tests

### 2. MCP Tools Tests (`mcp_tools_test.go`)
- **Individual Tool Testing**: Test all seven MCP tools separately
- **Complete Workflow Testing**: Full cluster lifecycle using all tools
- **Parameter Validation**: Input validation and error handling
- **Error Recovery**: Resilience testing and failure scenarios

### 3. AWS Workload Tests (`aws_workload_test.go`)
- **AWS Provider Validation**: Region, instance type, and parameter validation  
- **Cluster Lifecycle**: End-to-end AWS cluster creation, scaling, and deletion
- **Infrastructure Validation**: VPC, security group, and EC2 instance verification
- **Resource Cleanup**: Comprehensive AWS resource cleanup validation

### 4. Stability Tests (`e2e_stability_test.go`)
- **Environment Health**: Test environment validation and monitoring
- **AWS Connectivity**: AWS API connectivity and permission validation
- **MCP Server Responsiveness**: Load testing and concurrent request handling
- **Error Recovery**: Comprehensive error handling and recovery testing
- **Performance Benchmarking**: Response time validation against roadmap criteria

### 5. Integration Tests (`integration_test.go`) 
- Multi-step workflows combining multiple tools
- Concurrent operations testing  
- Cross-provider compatibility testing
- Complex failure and recovery scenarios

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

### 2. Run E2E Test Suites

#### Complete Test Suite (Recommended)
```bash
# Run all E2E tests with comprehensive validation
./test/e2e/scripts/run-e2e-suite.sh

# Run with parallel execution (faster)
./test/e2e/scripts/run-e2e-suite.sh --parallel

# Run specific test suites
./test/e2e/scripts/run-e2e-suite.sh --basic-only
./test/e2e/scripts/run-e2e-suite.sh --tools-only
./test/e2e/scripts/run-e2e-suite.sh --stability-only
```

#### Individual Test Categories
```bash
# Management cluster and environment tests
go test ./test/e2e -v -run TestManagementCluster -timeout 15m

# MCP tools comprehensive testing
go test ./test/e2e -v -run TestMCPTools -timeout 60m

# AWS workload cluster lifecycle
go test ./test/e2e -v -run TestAWSWorkloadClusterLifecycle -timeout 60m

# Stability and performance tests
go test ./test/e2e -v -run TestE2EStability -timeout 45m
```

#### AWS-Specific Tests
```bash
# Run AWS workload tests only
./test/e2e/scripts/run-aws-tests.sh

# Run AWS tests with specific region
./test/e2e/scripts/run-aws-tests.sh --region eu-west-1

# Run and keep resources for debugging
./test/e2e/scripts/run-aws-tests.sh --keep-resources
```

### 3. Manual Environment Setup (for debugging)
```bash
# Create kind cluster and install CAPI
./test/e2e/scripts/setup-cluster.sh

# Deploy MCP server
./test/e2e/scripts/deploy-server.sh

# Run individual tests
go test ./test/e2e -v -run TestCreateCluster -timeout 30m

# Cleanup when done
./test/e2e/scripts/cleanup.sh
```

## Test Structure

```
test/e2e/
├── README.md                    # This file - comprehensive testing documentation
├── main_test.go                 # Test suite setup and teardown (TestMain)
├── management_cluster_test.go   # Management cluster validation tests
├── mcp_tools_test.go           # Comprehensive MCP tools testing
├── aws_workload_test.go        # AWS workload cluster lifecycle tests
├── e2e_stability_test.go       # Stability and performance validation
├── scripts/
│   ├── setup-cluster.sh        # Kind cluster and CAPI setup
│   ├── deploy-server.sh        # MCP server deployment
│   ├── cleanup.sh              # Test environment cleanup
│   ├── run-aws-tests.sh        # AWS-specific test runner
│   └── run-e2e-suite.sh        # Complete E2E test suite runner
├── manifests/
│   ├── clusterclass.yaml       # Generic test ClusterClass definition
│   ├── aws-clusterclass.yaml   # AWS-specific ClusterClass with variables
│   └── server-deployment.yaml  # MCP server deployment manifests
└── utils/
    ├── cluster.go              # Cluster management utilities
    ├── mcp_client.go           # MCP client helpers
    ├── aws.go                  # AWS resource management and validation
    └── command.go              # Command execution utilities
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
- **Read Operations**: `list_clusters`, `get_cluster` < 500ms (validated in stability tests)
- **Acknowledgment**: Long-running operations return within 1s (create/delete/scale operations)
- **Cluster Creation**: Complete within 15 minutes (AWS workload cluster tests)
- **Scaling Operations**: Complete within 5 minutes (cluster scaling tests)
- **Concurrent Operations**: Multiple simultaneous MCP calls (load testing)
- **Error Recovery**: Server responsiveness after errors (resilience testing)

## Test Execution Strategies

### Continuous Integration (CI)
```bash
# Fast CI pipeline - basic tests only (no AWS)
./test/e2e/scripts/run-e2e-suite.sh --basic-only --parallel

# Full CI pipeline - with AWS credentials
./test/e2e/scripts/run-e2e-suite.sh --parallel
```

### Development Testing
```bash
# Quick development iteration
go test ./test/e2e -v -run TestMCPToolsIndividual

# Full local validation before PR
./test/e2e/scripts/run-e2e-suite.sh --no-aws-workload
```

### Production Validation
```bash
# Complete validation including AWS environment
./test/e2e/scripts/run-e2e-suite.sh

# Stress testing and stability validation
./test/e2e/scripts/run-e2e-suite.sh --stability-only --timeout 90m
```

## Monitoring and Observability

### Test Metrics Collected
- **Environment Setup Time**: Kind cluster creation and CAPI installation
- **MCP Server Response Times**: All tool operations with timing
- **AWS Resource Provisioning**: Infrastructure creation and cleanup times
- **Error Rates**: Failed operations and recovery success rates
- **Resource Usage**: Memory and CPU utilization during tests

### Logging and Debugging
- **Structured Logging**: All tests use structured logging with context
- **Test Artifacts**: Kubernetes manifests and logs preserved on failure
- **AWS Resource Tracking**: Complete inventory of created/cleaned resources
- **Performance Profiling**: Response time distribution and outliers