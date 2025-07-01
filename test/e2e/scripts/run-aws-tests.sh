#!/bin/bash

# AWS Workload Cluster E2E Test Runner
# This script runs comprehensive E2E tests for AWS workload cluster provisioning

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
E2E_TEST_TIMEOUT="${E2E_TEST_TIMEOUT:-45m}"
E2E_WORKLOAD_CLUSTER_NAME="${E2E_WORKLOAD_CLUSTER_NAME:-e2e-aws-cluster}"

# Logging
log() {
    echo "[$(date +'%Y-%m-%d %H:%M:%S')] $*"
}

error() {
    echo "[$(date +'%Y-%m-%d %H:%M:%S')] ERROR: $*" >&2
}

# Check prerequisites
check_prerequisites() {
    log "Checking prerequisites..."
    
    local missing_tools=()
    
    # Check required tools (basic availability check)
    for tool in kind kubectl clusterctl go docker aws; do
        if ! command -v "$tool" &> /dev/null; then
            missing_tools+=("$tool")
        fi
    done
    
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
    log "  kind: $(kind version 2>/dev/null || echo 'version detection failed')"
    log "  kubectl: $(kubectl version --client --short 2>/dev/null || echo 'version detection failed')"
    log "  clusterctl: $(clusterctl version -o short 2>/dev/null || echo 'version detection failed')"
    log "  go: $(go version 2>/dev/null || echo 'version detection failed')"
    log "  docker: $(docker version --format '{{.Client.Version}}' 2>/dev/null || echo 'version detection failed')"
    log "  aws: $(aws --version 2>/dev/null || echo 'version detection failed')"
    
    # Check Docker daemon
    if ! docker info &> /dev/null; then
        error "Docker daemon is not running or accessible"
        error "Please start Docker and ensure the current user has access"
        exit 1
    fi
    
    # Check Docker permissions
    if ! docker ps &> /dev/null; then
        error "Docker permission denied - cannot access Docker daemon"
        error "Please ensure the current user has Docker access"
        exit 1
    fi
    
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
        exit 1
    fi
    
    # Test AWS connectivity
    if ! aws sts get-caller-identity &> /dev/null; then
        error "Failed to authenticate with AWS"
        error "Please check your AWS credentials and region"
        exit 1
    fi
    
    log "Prerequisites check passed"
}

# Check AWS SSH key
check_aws_ssh_key() {
    log "Checking AWS SSH key configuration..."
    
    if [[ -z "${AWS_SSH_KEY_NAME:-}" ]]; then
        log "WARNING: AWS_SSH_KEY_NAME not set - cluster SSH access will not be available"
        log "To enable SSH access, create an EC2 key pair and set AWS_SSH_KEY_NAME"
        return 0
    fi
    
    # Verify SSH key exists in AWS
    if ! aws ec2 describe-key-pairs --key-names "$AWS_SSH_KEY_NAME" --region "$AWS_REGION" &> /dev/null; then
        error "SSH key '$AWS_SSH_KEY_NAME' not found in region '$AWS_REGION'"
        error "Please create the key pair or update AWS_SSH_KEY_NAME"
        exit 1
    fi
    
    log "SSH key '$AWS_SSH_KEY_NAME' found in region '$AWS_REGION'"
}

# Setup test environment
setup_environment() {
    log "Setting up test environment..."
    
    # Export required environment variables
    export KIND_CLUSTER_NAME
    export TEST_NAMESPACE
    export AWS_REGION
    export E2E_CLEANUP_AWS_RESOURCES
    export E2E_TEST_TIMEOUT
    export E2E_WORKLOAD_CLUSTER_NAME
    
    # Change to E2E directory
    cd "$E2E_DIR"
    
    log "Test environment configured"
    log "  Kind cluster: $KIND_CLUSTER_NAME"
    log "  AWS region: $AWS_REGION"
    log "  Test namespace: $TEST_NAMESPACE"
    log "  Cleanup AWS resources: $E2E_CLEANUP_AWS_RESOURCES"
    log "  Test timeout: $E2E_TEST_TIMEOUT"
}

# Run specific test suite
run_test_suite() {
    local test_name="$1"
    local test_pattern="$2"
    
    log "Running test suite: $test_name"
    log "Test pattern: $test_pattern"
    
    # Run the test with timeout
    if timeout "$E2E_TEST_TIMEOUT" go test -v -timeout="$E2E_TEST_TIMEOUT" -run="$test_pattern" .; then
        log "Test suite '$test_name' PASSED"
        return 0
    else
        local exit_code=$?
        error "Test suite '$test_name' FAILED (exit code: $exit_code)"
        return $exit_code
    fi
}

# Run all AWS E2E tests
run_all_tests() {
    log "Running all AWS E2E tests..."
    
    local failed_tests=()
    
    # Test 1: AWS Provider Validation
    log "=== Test 1: AWS Provider Validation ==="
    if ! run_test_suite "AWS Provider Validation" "TestAWSProviderValidation"; then
        failed_tests+=("AWS Provider Validation")
    fi
    
    # Test 2: AWS Workload Cluster Lifecycle
    log "=== Test 2: AWS Workload Cluster Lifecycle ==="
    if ! run_test_suite "AWS Workload Cluster Lifecycle" "TestAWSWorkloadClusterLifecycle"; then
        failed_tests+=("AWS Workload Cluster Lifecycle")
    fi
    
    # Report results
    if [[ ${#failed_tests[@]} -eq 0 ]]; then
        log "All AWS E2E tests PASSED ✓"
        return 0
    else
        error "Failed test suites: ${failed_tests[*]}"
        return 1
    fi
}

# Cleanup function
cleanup() {
    local exit_code=$?
    
    log "Performing cleanup..."
    
    # Try to cleanup any remaining test clusters
    if [[ "$E2E_CLEANUP_AWS_RESOURCES" == "true" ]]; then
        log "Cleaning up AWS resources..."
        
        # Run cleanup script if it exists
        if [[ -f "${SCRIPT_DIR}/cleanup.sh" ]]; then
            bash "${SCRIPT_DIR}/cleanup.sh" || log "Cleanup script failed, continuing..."
        fi
        
        # Force cleanup any remaining instances with our test tag
        local test_clusters=("$E2E_WORKLOAD_CLUSTER_NAME" "test-valid-region" "test-valid-instance")
        for cluster in "${test_clusters[@]}"; do
            log "Cleaning up cluster: $cluster"
            aws ec2 describe-instances \
                --region "$AWS_REGION" \
                --filters "Name=tag:sigs.k8s.io/cluster-api-provider-aws/cluster/$cluster,Values=owned" \
                --query 'Reservations[].Instances[].InstanceId' \
                --output text 2>/dev/null | xargs -r aws ec2 terminate-instances --region "$AWS_REGION" --instance-ids || true
        done
    fi
    
    exit $exit_code
}

# Show usage
show_usage() {
    cat << EOF
Usage: $0 [OPTIONS] [TEST_SUITE]

Run AWS workload cluster E2E tests for the CAPI MCP Server.

OPTIONS:
    -h, --help              Show this help message
    -c, --cleanup-only      Only run cleanup, don't run tests  
    -s, --skip-setup        Skip environment setup (assume already set up)
    -k, --keep-resources    Don't cleanup AWS resources after tests
    --region REGION         AWS region for testing (default: us-west-2)
    --timeout DURATION      Test timeout duration (default: 45m)

TEST_SUITE:
    all                     Run all test suites (default)
    validation              Run AWS provider validation tests only
    lifecycle               Run AWS workload cluster lifecycle tests only

ENVIRONMENT VARIABLES:
    AWS_ACCESS_KEY_ID       AWS access key (required)
    AWS_SECRET_ACCESS_KEY   AWS secret key (required)
    AWS_SSH_KEY_NAME        EC2 key pair name for SSH access (optional)
    AWS_REGION              AWS region (default: us-west-2)
    KIND_CLUSTER_NAME       Kind cluster name (default: capi-e2e)
    E2E_CLEANUP_AWS_RESOURCES  Cleanup AWS resources (default: true)
    E2E_TEST_TIMEOUT        Test timeout (default: 45m)
    E2E_WORKLOAD_CLUSTER_NAME  Test cluster name (default: e2e-aws-cluster)

EXAMPLES:
    # Run all tests with default settings
    $0
    
    # Run only validation tests
    $0 validation
    
    # Run tests in eu-west-1 region
    $0 --region eu-west-1
    
    # Run tests and keep AWS resources for debugging
    $0 --keep-resources
    
    # Only cleanup existing AWS resources
    $0 --cleanup-only

PREREQUISITES:
    - kind (v0.20.0+)
    - kubectl (v1.28+)
    - clusterctl (v1.6.0+)
    - Docker (v20.10+)
    - Go (v1.23+)
    - AWS CLI
    - Valid AWS credentials
    - EC2 key pair (optional, for SSH access)

For more information, see test/e2e/README.md
EOF
}

# Main function
main() {
    local cleanup_only=false
    local skip_setup=false
    local test_suite="all"
    
    # Parse command line arguments
    while [[ $# -gt 0 ]]; do
        case $1 in
            -h|--help)
                show_usage
                exit 0
                ;;
            -c|--cleanup-only)
                cleanup_only=true
                shift
                ;;
            -s|--skip-setup)
                skip_setup=true
                shift
                ;;
            -k|--keep-resources)
                E2E_CLEANUP_AWS_RESOURCES=false
                shift
                ;;
            --region)
                AWS_REGION="$2"
                shift 2
                ;;
            --timeout)
                E2E_TEST_TIMEOUT="$2"
                shift 2
                ;;
            all|validation|lifecycle)
                test_suite="$1"
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
    
    log "Starting AWS workload cluster E2E tests"
    log "Test suite: $test_suite"
    
    # Run cleanup only if requested
    if [[ "$cleanup_only" == "true" ]]; then
        log "Running cleanup only..."
        cleanup
        exit 0
    fi
    
    # Check prerequisites
    check_prerequisites
    check_aws_ssh_key
    
    # Setup environment
    setup_environment
    
    # Skip setup if requested (for debugging)
    if [[ "$skip_setup" == "false" ]]; then
        log "Test environment will be set up by TestMain"
    else
        log "Skipping environment setup as requested"
    fi
    
    # Run tests based on suite selection
    case "$test_suite" in
        "all")
            run_all_tests
            ;;
        "validation")
            run_test_suite "AWS Provider Validation" "TestAWSProviderValidation"
            ;;
        "lifecycle")
            run_test_suite "AWS Workload Cluster Lifecycle" "TestAWSWorkloadClusterLifecycle"
            ;;
        *)
            error "Unknown test suite: $test_suite"
            exit 1
            ;;
    esac
    
    log "AWS workload cluster E2E tests completed successfully ✓"
}

# Run main function
main "$@"