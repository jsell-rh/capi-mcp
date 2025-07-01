#!/bin/bash

# E2E Test Suite Runner
# Comprehensive E2E testing with stability validation and AWS environment checks

set -euo pipefail

# Script configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/../../.." && pwd)"
E2E_DIR="${PROJECT_ROOT}/test/e2e"

# Default configuration
KIND_CLUSTER_NAME="${KIND_CLUSTER_NAME:-capi-e2e}"
TEST_NAMESPACE="${TEST_NAMESPACE:-default}"
AWS_REGION="${AWS_REGION:-us-west-2}"
E2E_CLEANUP_AWS_RESOURCES="${E2E_CLEANUP_AWS_RESOURCES:-true}"
E2E_TEST_TIMEOUT="${E2E_TEST_TIMEOUT:-60m}"
E2E_PARALLEL_TESTS="${E2E_PARALLEL_TESTS:-false}"
E2E_STABILITY_TESTS="${E2E_STABILITY_TESTS:-true}"
E2E_PERFORMANCE_TESTS="${E2E_PERFORMANCE_TESTS:-true}"

# Test suite selection
E2E_SUITE_BASIC="${E2E_SUITE_BASIC:-true}"
E2E_SUITE_MCP_TOOLS="${E2E_SUITE_MCP_TOOLS:-true}"
E2E_SUITE_AWS_WORKLOAD="${E2E_SUITE_AWS_WORKLOAD:-true}"
E2E_SUITE_STABILITY="${E2E_SUITE_STABILITY:-true}"

# Logging
log() {
    echo "[$(date +'%Y-%m-%d %H:%M:%S')] $*"
}

error() {
    echo "[$(date +'%Y-%m-%d %H:%M:%S')] ERROR: $*" >&2
}

warn() {
    echo "[$(date +'%Y-%m-%d %H:%M:%S')] WARN: $*" >&2
}

# Check prerequisites
check_prerequisites() {
    log "Checking E2E test prerequisites..."
    
    local missing_tools=()
    
    # Check required tools with basic availability (skip strict version checking for now)
    for tool in kind kubectl clusterctl go docker; do
        if ! command -v "$tool" &> /dev/null; then
            missing_tools+=("$tool")
        fi
    done
    
    # Check AWS tools if AWS tests are enabled
    if [[ "$E2E_SUITE_AWS_WORKLOAD" == "true" || "$E2E_SUITE_STABILITY" == "true" ]]; then
        if ! command -v aws &> /dev/null; then
            missing_tools+=("aws")
        fi
    fi
    
    # Report missing tools
    if [[ ${#missing_tools[@]} -gt 0 ]]; then
        error "Missing required tools: ${missing_tools[*]}"
        error ""
        error "Please install the missing tools:"
        for tool in "${missing_tools[@]}"; do
            case $tool in
                kind)
                    error "  kind: go install sigs.k8s.io/kind@latest"
                    ;;
                kubectl)
                    error "  kubectl: https://kubernetes.io/docs/tasks/tools/install-kubectl/"
                    ;;
                clusterctl)
                    error "  clusterctl: https://cluster-api.sigs.k8s.io/user/quick-start.html#install-clusterctl"
                    ;;
                go)
                    error "  go: https://golang.org/doc/install"
                    ;;
                docker)
                    error "  docker: https://docs.docker.com/get-docker/"
                    ;;
                aws)
                    error "  aws: https://docs.aws.amazon.com/cli/latest/userguide/getting-started-install.html"
                    ;;
            esac
        done
        exit 1
    fi
    
    # Log detected tool versions (informational only)
    log "Detected tool versions:"
    command -v kind &> /dev/null && log "  kind: $(kind version 2>/dev/null || echo 'version detection failed')"
    command -v kubectl &> /dev/null && log "  kubectl: $(kubectl version --client --short 2>/dev/null || echo 'version detection failed')"
    command -v clusterctl &> /dev/null && log "  clusterctl: $(clusterctl version -o short 2>/dev/null || echo 'version detection failed')"
    command -v go &> /dev/null && log "  go: $(go version 2>/dev/null || echo 'version detection failed')"
    command -v docker &> /dev/null && log "  docker: $(docker version --format '{{.Client.Version}}' 2>/dev/null || echo 'version detection failed')"
    if [[ "$E2E_SUITE_AWS_WORKLOAD" == "true" || "$E2E_SUITE_STABILITY" == "true" ]]; then
        command -v aws &> /dev/null && log "  aws: $(aws --version 2>/dev/null || echo 'version detection failed')"
    fi
    
    # Check Docker daemon
    if ! docker info &> /dev/null; then
        error "Docker daemon is not running or accessible"
        error "Please start Docker and ensure the current user has access"
        error "You may need to add your user to the docker group:"
        error "  sudo usermod -aG docker \$USER"
        error "  newgrp docker"
        exit 1
    fi
    
    # Check Docker permissions
    if ! docker ps &> /dev/null; then
        error "Docker permission denied - cannot access Docker daemon"
        error "Please ensure the current user has Docker access"
        exit 1
    fi
    
    log "Prerequisites check passed"
}

# Check AWS environment if needed
check_aws_environment() {
    if [[ "$E2E_SUITE_AWS_WORKLOAD" != "true" && "$E2E_SUITE_STABILITY" != "true" ]]; then
        log "Skipping AWS environment check (AWS tests not enabled)"
        return 0
    fi
    
    log "Checking AWS environment..."
    
    # Check AWS CLI availability
    if ! command -v aws &> /dev/null; then
        error "AWS CLI is required for AWS tests but not found"
        error "Install AWS CLI: https://docs.aws.amazon.com/cli/latest/userguide/getting-started-install.html"
        error "Or disable AWS tests with --no-aws-workload --no-stability"
        exit 1
    fi
    
    # Log AWS CLI version (informational)
    local aws_version_info
    aws_version_info=$(aws --version 2>&1 || echo "version detection failed")
    log "AWS CLI version: $aws_version_info"
    
    # Check AWS credentials
    if [[ -z "${AWS_ACCESS_KEY_ID:-}" || -z "${AWS_SECRET_ACCESS_KEY:-}" ]]; then
        error "AWS credentials not found in environment"
        error ""
        error "Please configure AWS credentials using one of:"
        error "  1. Environment variables:"
        error "     export AWS_ACCESS_KEY_ID=your-access-key"
        error "     export AWS_SECRET_ACCESS_KEY=your-secret-key"
        error "  2. AWS credentials file: aws configure"
        error "  3. IAM role (if running on EC2)"
        error ""
        error "Or disable AWS tests with:"
        error "  $0 --no-aws-workload --no-stability"
        exit 1
    fi
    
    # Check AWS region
    if [[ -z "${AWS_REGION:-}" ]]; then
        error "AWS_REGION not set"
        error "Please set AWS_REGION environment variable or use --region flag"
        exit 1
    fi
    
    # Test AWS connectivity and permissions
    log "Testing AWS connectivity and permissions..."
    
    local caller_identity
    if ! caller_identity=$(aws sts get-caller-identity 2>&1); then
        error "Failed to authenticate with AWS:"
        error "$caller_identity"
        error ""
        error "Please verify:"
        error "  1. AWS credentials are correct"
        error "  2. AWS region '$AWS_REGION' is valid"
        error "  3. Network connectivity to AWS"
        exit 1
    fi
    
    # Log AWS identity info
    local aws_account
    aws_account=$(echo "$caller_identity" | grep -o '"Account": "[^"]*' | cut -d'"' -f4)
    local aws_user
    aws_user=$(echo "$caller_identity" | grep -o '"Arn": "[^"]*' | cut -d'"' -f4)
    log "AWS authentication successful"
    log "  Account: $aws_account"
    log "  User/Role: $aws_user"
    log "  Region: $AWS_REGION"
    
    # Test required AWS permissions
    log "Verifying AWS permissions..."
    
    local permission_errors=()
    
    # Test EC2 permissions
    if ! aws ec2 describe-regions --region "$AWS_REGION" &> /dev/null; then
        permission_errors+=("EC2 describe-regions")
    fi
    
    if ! aws ec2 describe-vpcs --region "$AWS_REGION" --max-items 1 &> /dev/null; then
        permission_errors+=("EC2 describe-vpcs")
    fi
    
    if ! aws ec2 describe-instances --region "$AWS_REGION" --max-items 1 &> /dev/null; then
        permission_errors+=("EC2 describe-instances")
    fi
    
    if ! aws ec2 describe-security-groups --region "$AWS_REGION" --max-items 1 &> /dev/null; then
        permission_errors+=("EC2 describe-security-groups")
    fi
    
    # Test ELB permissions
    if ! aws elbv2 describe-load-balancers --region "$AWS_REGION" --page-size 1 &> /dev/null; then
        permission_errors+=("ELBv2 describe-load-balancers")
    fi
    
    if [[ ${#permission_errors[@]} -gt 0 ]]; then
        error "Missing required AWS permissions:"
        for permission in "${permission_errors[@]}"; do
            error "  $permission"
        done
        error ""
        error "Please ensure your AWS credentials have the following permissions:"
        error "  - EC2: describe-regions, describe-vpcs, describe-instances, describe-security-groups"
        error "  - EC2: run-instances, terminate-instances, create-tags"
        error "  - ELBv2: describe-load-balancers, describe-tags"
        error "  - IAM: create-role, attach-role-policy (for CAPI)"
        exit 1
    fi
    
    # Check AWS SSH key
    if [[ -n "${AWS_SSH_KEY_NAME:-}" ]]; then
        log "Verifying SSH key '$AWS_SSH_KEY_NAME' in region '$AWS_REGION'..."
        if ! aws ec2 describe-key-pairs --key-names "$AWS_SSH_KEY_NAME" --region "$AWS_REGION" &> /dev/null; then
            error "SSH key '$AWS_SSH_KEY_NAME' not found in region '$AWS_REGION'"
            error ""
            error "Please create an EC2 key pair or update AWS_SSH_KEY_NAME"
            error "To create a key pair:"
            error "  aws ec2 create-key-pair --key-name my-key --region $AWS_REGION"
            error ""
            error "Or unset AWS_SSH_KEY_NAME to proceed without SSH access"
            exit 1
        else
            log "SSH key '$AWS_SSH_KEY_NAME' found in region '$AWS_REGION'"
        fi
    else
        warn "AWS_SSH_KEY_NAME not set - cluster SSH access will not be available"
        warn "Set AWS_SSH_KEY_NAME to enable SSH access to cluster nodes"
    fi
    
    log "AWS environment validation passed"
}

# Setup test environment
setup_environment() {
    log "Setting up E2E test environment..."
    
    # Export required environment variables
    export KIND_CLUSTER_NAME
    export TEST_NAMESPACE
    export AWS_REGION
    export E2E_CLEANUP_AWS_RESOURCES
    export E2E_TEST_TIMEOUT
    
    # Change to E2E directory
    cd "$E2E_DIR"
    
    log "Test environment configured"
    log "  Kind cluster: $KIND_CLUSTER_NAME"
    log "  Test namespace: $TEST_NAMESPACE"
    log "  AWS region: $AWS_REGION"
    log "  Cleanup AWS resources: $E2E_CLEANUP_AWS_RESOURCES"
    log "  Test timeout: $E2E_TEST_TIMEOUT"
    log "  Parallel tests: $E2E_PARALLEL_TESTS"
    log "  Stability tests: $E2E_STABILITY_TESTS"
    log "  Performance tests: $E2E_PERFORMANCE_TESTS"
}

# Run specific test suite
run_test_suite() {
    local suite_name="$1"
    local test_pattern="$2"
    local timeout="${3:-$E2E_TEST_TIMEOUT}"
    
    log "Running test suite: $suite_name"
    log "Test pattern: $test_pattern"
    log "Timeout: $timeout"
    
    local go_test_flags="-v -timeout=$timeout"
    
    # Add parallel flag if enabled
    if [[ "$E2E_PARALLEL_TESTS" == "true" ]]; then
        go_test_flags="$go_test_flags -parallel=4"
    fi
    
    # Run the test with timeout
    if timeout "$timeout" go test $go_test_flags -run="$test_pattern" .; then
        log "Test suite '$suite_name' PASSED ✓"
        return 0
    else
        local exit_code=$?
        error "Test suite '$suite_name' FAILED ✗ (exit code: $exit_code)"
        return $exit_code
    fi
}

# Run basic test suites
run_basic_tests() {
    if [[ "$E2E_SUITE_BASIC" != "true" ]]; then
        log "Skipping basic test suite"
        return 0
    fi
    
    log "=== Running Basic Test Suite ==="
    
    # Management cluster tests
    if ! run_test_suite "Management Cluster" "TestManagementCluster" "15m"; then
        return 1
    fi
    
    # Environment validation
    if ! run_test_suite "Environment Validation" "TestE2EEnvironmentValidation" "10m"; then
        return 1
    fi
    
    return 0
}

# Run MCP tools tests
run_mcp_tools_tests() {
    if [[ "$E2E_SUITE_MCP_TOOLS" != "true" ]]; then
        log "Skipping MCP tools test suite"
        return 0
    fi
    
    log "=== Running MCP Tools Test Suite ==="
    
    # Individual tool tests
    if ! run_test_suite "MCP Tools Individual" "TestMCPToolsIndividual" "10m"; then
        return 1
    fi
    
    # Parameter validation tests
    if ! run_test_suite "MCP Tools Parameter Validation" "TestMCPToolsParameterValidation" "5m"; then
        return 1
    fi
    
    # Complete workflow tests (if AWS is available)
    if [[ -n "${AWS_ACCESS_KEY_ID:-}" && -n "${AWS_SECRET_ACCESS_KEY:-}" ]]; then
        if ! run_test_suite "MCP Tools Complete Workflow" "TestMCPToolsComplete" "45m"; then
            return 1
        fi
    else
        log "Skipping MCP Tools Complete Workflow (AWS credentials not available)"
    fi
    
    return 0
}

# Run AWS workload tests
run_aws_workload_tests() {
    if [[ "$E2E_SUITE_AWS_WORKLOAD" != "true" ]]; then
        log "Skipping AWS workload test suite"
        return 0
    fi
    
    if [[ -z "${AWS_ACCESS_KEY_ID:-}" || -z "${AWS_SECRET_ACCESS_KEY:-}" ]]; then
        log "Skipping AWS workload tests (credentials not available)"
        return 0
    fi
    
    log "=== Running AWS Workload Test Suite ==="
    
    # AWS provider validation
    if ! run_test_suite "AWS Provider Validation" "TestAWSProviderValidation" "10m"; then
        return 1
    fi
    
    # AWS workload cluster lifecycle
    if ! run_test_suite "AWS Workload Cluster Lifecycle" "TestAWSWorkloadClusterLifecycle" "60m"; then
        return 1
    fi
    
    return 0
}

# Run stability tests
run_stability_tests() {
    if [[ "$E2E_SUITE_STABILITY" != "true" ]]; then
        log "Skipping stability test suite"
        return 0
    fi
    
    log "=== Running Stability Test Suite ==="
    
    # E2E stability tests
    if ! run_test_suite "E2E Stability" "TestE2EStability" "60m"; then
        return 1
    fi
    
    return 0
}

# Generate test report
generate_test_report() {
    local exit_code=$1
    local start_time=$2
    local end_time=$3
    
    log "=== E2E Test Suite Report ==="
    log "Start time: $(date -d "@$start_time" '+%Y-%m-%d %H:%M:%S')"
    log "End time: $(date -d "@$end_time" '+%Y-%m-%d %H:%M:%S')"
    log "Duration: $((end_time - start_time)) seconds"
    log "Overall result: $([ $exit_code -eq 0 ] && echo "PASSED ✓" || echo "FAILED ✗")"
    
    # Test suite summary
    log "Test suites run:"
    [[ "$E2E_SUITE_BASIC" == "true" ]] && log "  ✓ Basic tests"
    [[ "$E2E_SUITE_MCP_TOOLS" == "true" ]] && log "  ✓ MCP tools tests"
    [[ "$E2E_SUITE_AWS_WORKLOAD" == "true" ]] && log "  ✓ AWS workload tests"
    [[ "$E2E_SUITE_STABILITY" == "true" ]] && log "  ✓ Stability tests"
    
    # Environment information
    log "Environment:"
    log "  Kind cluster: $KIND_CLUSTER_NAME"
    log "  AWS region: $AWS_REGION"
    log "  Go version: $(go version | awk '{print $3}')"
    log "  kubectl version: $(kubectl version --client --short | awk '{print $3}')"
    
    return $exit_code
}

# Cleanup function
cleanup() {
    local exit_code=$?
    
    log "Performing E2E test cleanup..."
    
    # Try to cleanup any remaining test resources
    if [[ "$E2E_CLEANUP_AWS_RESOURCES" == "true" ]]; then
        log "Cleaning up AWS resources..."
        
        # Run AWS cleanup script if it exists
        if [[ -f "${SCRIPT_DIR}/cleanup.sh" ]]; then
            bash "${SCRIPT_DIR}/cleanup.sh" || log "AWS cleanup script failed, continuing..."
        fi
    fi
    
    return $exit_code
}

# Show usage
show_usage() {
    cat << EOF
Usage: $0 [OPTIONS]

Run comprehensive E2E tests for the CAPI MCP Server.

OPTIONS:
    -h, --help              Show this help message
    -b, --basic-only        Run only basic tests (skip AWS-dependent tests)
    -t, --tools-only        Run only MCP tools tests
    -w, --workload-only     Run only AWS workload tests
    -s, --stability-only    Run only stability tests
    -p, --parallel          Enable parallel test execution
    --no-cleanup            Don't cleanup AWS resources after tests
    --timeout DURATION      Test timeout duration (default: 60m)
    --region REGION         AWS region for testing (default: us-west-2)

TEST SUITE CONTROL:
    --no-basic              Skip basic tests
    --no-mcp-tools          Skip MCP tools tests
    --no-aws-workload       Skip AWS workload tests
    --no-stability          Skip stability tests

ENVIRONMENT VARIABLES:
    AWS_ACCESS_KEY_ID       AWS access key (required for AWS tests)
    AWS_SECRET_ACCESS_KEY   AWS secret key (required for AWS tests)
    AWS_SSH_KEY_NAME        EC2 key pair name for SSH access (optional)
    AWS_REGION              AWS region (default: us-west-2)
    KIND_CLUSTER_NAME       Kind cluster name (default: capi-e2e)
    E2E_CLEANUP_AWS_RESOURCES  Cleanup AWS resources (default: true)
    E2E_TEST_TIMEOUT        Test timeout (default: 60m)
    E2E_PARALLEL_TESTS      Enable parallel execution (default: false)

EXAMPLES:
    # Run all test suites
    $0
    
    # Run only basic tests (no AWS required)
    $0 --basic-only
    
    # Run with parallel execution
    $0 --parallel
    
    # Run in eu-west-1 region with longer timeout
    $0 --region eu-west-1 --timeout 90m
    
    # Run without cleanup for debugging
    $0 --no-cleanup

PREREQUISITES:
    - kind (v0.20.0+)
    - kubectl (v1.28+)
    - clusterctl (v1.6.0+)
    - Docker (v20.10+)
    - Go (v1.24+)
    - AWS CLI (for AWS tests)
    - Valid AWS credentials (for AWS tests)

EOF
}

# Main function
main() {
    local start_time
    start_time=$(date +%s)
    
    # Parse command line arguments
    while [[ $# -gt 0 ]]; do
        case $1 in
            -h|--help)
                show_usage
                exit 0
                ;;
            -b|--basic-only)
                E2E_SUITE_BASIC=true
                E2E_SUITE_MCP_TOOLS=false
                E2E_SUITE_AWS_WORKLOAD=false
                E2E_SUITE_STABILITY=false
                shift
                ;;
            -t|--tools-only)
                E2E_SUITE_BASIC=false
                E2E_SUITE_MCP_TOOLS=true
                E2E_SUITE_AWS_WORKLOAD=false
                E2E_SUITE_STABILITY=false
                shift
                ;;
            -w|--workload-only)
                E2E_SUITE_BASIC=false
                E2E_SUITE_MCP_TOOLS=false
                E2E_SUITE_AWS_WORKLOAD=true
                E2E_SUITE_STABILITY=false
                shift
                ;;
            -s|--stability-only)
                E2E_SUITE_BASIC=false
                E2E_SUITE_MCP_TOOLS=false
                E2E_SUITE_AWS_WORKLOAD=false
                E2E_SUITE_STABILITY=true
                shift
                ;;
            -p|--parallel)
                E2E_PARALLEL_TESTS=true
                shift
                ;;
            --no-cleanup)
                E2E_CLEANUP_AWS_RESOURCES=false
                shift
                ;;
            --timeout)
                E2E_TEST_TIMEOUT="$2"
                shift 2
                ;;
            --region)
                AWS_REGION="$2"
                shift 2
                ;;
            --no-basic)
                E2E_SUITE_BASIC=false
                shift
                ;;
            --no-mcp-tools)
                E2E_SUITE_MCP_TOOLS=false
                shift
                ;;
            --no-aws-workload)
                E2E_SUITE_AWS_WORKLOAD=false
                shift
                ;;
            --no-stability)
                E2E_SUITE_STABILITY=false
                shift
                ;;
            *)
                error "Unknown option: $1"
                show_usage
                exit 1
                ;;
        esac
    done
    
    # Set up signal handling
    trap cleanup EXIT INT TERM
    
    log "Starting comprehensive E2E test suite"
    
    # Check prerequisites
    check_prerequisites
    check_aws_environment
    
    # Setup environment
    setup_environment
    
    # Track overall success
    local overall_success=true
    
    # Run test suites
    if ! run_basic_tests; then
        overall_success=false
    fi
    
    if ! run_mcp_tools_tests; then
        overall_success=false
    fi
    
    if ! run_aws_workload_tests; then
        overall_success=false
    fi
    
    if ! run_stability_tests; then
        overall_success=false
    fi
    
    # Generate report
    local end_time
    end_time=$(date +%s)
    
    if [[ "$overall_success" == "true" ]]; then
        generate_test_report 0 "$start_time" "$end_time"
        log "All E2E test suites completed successfully ✓"
        exit 0
    else
        generate_test_report 1 "$start_time" "$end_time"
        error "One or more E2E test suites failed ✗"
        exit 1
    fi
}

# Run main function
main "$@"