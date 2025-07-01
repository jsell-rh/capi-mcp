#!/bin/bash

# cleanup.sh
# Cleans up the E2E test environment and AWS resources

set -euo pipefail

# Configuration
KIND_CLUSTER_NAME="${KIND_CLUSTER_NAME:-capi-e2e}"
MCP_SERVER_NAMESPACE="${MCP_SERVER_NAMESPACE:-capi-mcp-system}"
AWS_REGION="${AWS_REGION:-us-west-2}"
CLEANUP_AWS="${CLEANUP_AWS:-true}"
FORCE_CLEANUP="${FORCE_CLEANUP:-false}"

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

# Confirm cleanup if not forced
confirm_cleanup() {
    if [[ "${FORCE_CLEANUP}" == "true" ]]; then
        log "Force cleanup enabled, proceeding without confirmation"
        return 0
    fi
    
    echo
    warn "This will clean up the E2E test environment:"
    warn "  - Kind cluster: ${KIND_CLUSTER_NAME}"
    warn "  - All CAPI resources in the cluster"
    if [[ "${CLEANUP_AWS}" == "true" ]]; then
        warn "  - AWS resources created during testing"
        warn "  - This may incur costs or remove important resources!"
    fi
    echo
    
    read -p "Are you sure you want to continue? (y/N): " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        log "Cleanup cancelled"
        exit 0
    fi
}

# Delete all CAPI clusters (triggers AWS cleanup)
cleanup_capi_clusters() {
    log "Cleaning up CAPI clusters..."
    
    # Switch to the kind cluster context
    if kubectl config get-contexts | grep -q "kind-${KIND_CLUSTER_NAME}"; then
        kubectl config use-context "kind-${KIND_CLUSTER_NAME}"
    else
        warn "Kind cluster context not found, skipping CAPI cleanup"
        return 0
    fi
    
    # Get all clusters
    local clusters
    clusters=$(kubectl get clusters --all-namespaces -o jsonpath='{range .items[*]}{.metadata.namespace}{" "}{.metadata.name}{"\n"}{end}' 2>/dev/null || true)
    
    if [[ -z "$clusters" ]]; then
        log "No CAPI clusters found to clean up"
        return 0
    fi
    
    log "Found CAPI clusters to delete:"
    echo "$clusters"
    
    # Delete each cluster
    while IFS= read -r line; do
        if [[ -n "$line" ]]; then
            local namespace name
            namespace=$(echo "$line" | awk '{print $1}')
            name=$(echo "$line" | awk '{print $2}')
            
            log "Deleting cluster: ${namespace}/${name}"
            kubectl delete cluster "$name" -n "$namespace" --timeout=60s || true
        fi
    done <<< "$clusters"
    
    # Wait for clusters to be deleted (this triggers AWS resource cleanup)
    log "Waiting for clusters to be deleted (this may take several minutes)..."
    local timeout=600  # 10 minutes
    local elapsed=0
    
    while [[ $elapsed -lt $timeout ]]; do
        local remaining_clusters
        remaining_clusters=$(kubectl get clusters --all-namespaces --no-headers 2>/dev/null | wc -l || echo "0")
        
        if [[ "$remaining_clusters" -eq 0 ]]; then
            success "All CAPI clusters have been deleted"
            return 0
        fi
        
        log "Still waiting for ${remaining_clusters} clusters to be deleted..."
        sleep 30
        elapsed=$((elapsed + 30))
    done
    
    warn "Timeout waiting for clusters to be deleted"
    kubectl get clusters --all-namespaces || true
}

# Clean up AWS resources directly (if CAPI cleanup didn't work)
cleanup_aws_resources() {
    if [[ "${CLEANUP_AWS}" != "true" ]]; then
        log "AWS cleanup disabled, skipping"
        return 0
    fi
    
    log "Cleaning up AWS resources..."
    
    # Check if AWS CLI is available
    if ! command -v aws &> /dev/null; then
        warn "AWS CLI not found, skipping AWS resource cleanup"
        warn "Please manually check for and delete any AWS resources created during testing"
        return 0
    fi
    
    # Check AWS credentials
    if ! aws sts get-caller-identity &> /dev/null; then
        warn "AWS credentials not configured, skipping AWS resource cleanup"
        return 0
    fi
    
    local current_region="${AWS_REGION}"
    log "Cleaning up AWS resources in region: ${current_region}"
    
    # Clean up EC2 instances with CAPI tags
    cleanup_ec2_instances "$current_region"
    
    # Clean up VPCs with CAPI tags
    cleanup_vpcs "$current_region"
    
    # Clean up security groups
    cleanup_security_groups "$current_region"
    
    # Clean up load balancers
    cleanup_load_balancers "$current_region"
    
    success "AWS resource cleanup completed"
}

# Clean up EC2 instances
cleanup_ec2_instances() {
    local region="$1"
    log "Cleaning up EC2 instances in region: ${region}"
    
    # Find instances with CAPI tags
    local instances
    instances=$(aws ec2 describe-instances \
        --region "$region" \
        --filters "Name=tag:cluster.x-k8s.io/cluster-name,Values=*" \
                  "Name=instance-state-name,Values=running,pending,stopping,stopped" \
        --query 'Reservations[*].Instances[*].InstanceId' \
        --output text 2>/dev/null || true)
    
    if [[ -n "$instances" && "$instances" != "None" ]]; then
        log "Found EC2 instances to terminate: $instances"
        aws ec2 terminate-instances --region "$region" --instance-ids $instances || true
        
        # Wait a bit for termination to start
        sleep 10
    else
        log "No CAPI EC2 instances found"
    fi
}

# Clean up VPCs
cleanup_vpcs() {
    local region="$1"
    log "Cleaning up VPCs in region: ${region}"
    
    # Find VPCs with CAPI tags
    local vpcs
    vpcs=$(aws ec2 describe-vpcs \
        --region "$region" \
        --filters "Name=tag:cluster.x-k8s.io/cluster-name,Values=*" \
        --query 'Vpcs[*].VpcId' \
        --output text 2>/dev/null || true)
    
    if [[ -n "$vpcs" && "$vpcs" != "None" ]]; then
        log "Found VPCs with CAPI tags: $vpcs"
        
        for vpc in $vpcs; do
            log "Cleaning up VPC: $vpc"
            
            # Delete subnets
            local subnets
            subnets=$(aws ec2 describe-subnets \
                --region "$region" \
                --filters "Name=vpc-id,Values=$vpc" \
                --query 'Subnets[*].SubnetId' \
                --output text 2>/dev/null || true)
            
            if [[ -n "$subnets" && "$subnets" != "None" ]]; then
                for subnet in $subnets; do
                    log "Deleting subnet: $subnet"
                    aws ec2 delete-subnet --region "$region" --subnet-id "$subnet" || true
                done
            fi
            
            # Delete internet gateways
            local igws
            igws=$(aws ec2 describe-internet-gateways \
                --region "$region" \
                --filters "Name=attachment.vpc-id,Values=$vpc" \
                --query 'InternetGateways[*].InternetGatewayId' \
                --output text 2>/dev/null || true)
            
            if [[ -n "$igws" && "$igws" != "None" ]]; then
                for igw in $igws; do
                    log "Detaching and deleting internet gateway: $igw"
                    aws ec2 detach-internet-gateway --region "$region" --internet-gateway-id "$igw" --vpc-id "$vpc" || true
                    aws ec2 delete-internet-gateway --region "$region" --internet-gateway-id "$igw" || true
                done
            fi
            
            # Delete VPC (after a delay to allow cleanup)
            sleep 30
            log "Deleting VPC: $vpc"
            aws ec2 delete-vpc --region "$region" --vpc-id "$vpc" || true
        done
    else
        log "No CAPI VPCs found"
    fi
}

# Clean up security groups
cleanup_security_groups() {
    local region="$1"
    log "Cleaning up security groups in region: ${region}"
    
    # Find security groups with CAPI tags
    local sgs
    sgs=$(aws ec2 describe-security-groups \
        --region "$region" \
        --filters "Name=tag:cluster.x-k8s.io/cluster-name,Values=*" \
        --query 'SecurityGroups[*].GroupId' \
        --output text 2>/dev/null || true)
    
    if [[ -n "$sgs" && "$sgs" != "None" ]]; then
        log "Found security groups with CAPI tags: $sgs"
        
        for sg in $sgs; do
            log "Deleting security group: $sg"
            aws ec2 delete-security-group --region "$region" --group-id "$sg" || true
        done
    else
        log "No CAPI security groups found"
    fi
}

# Clean up load balancers
cleanup_load_balancers() {
    local region="$1"
    log "Cleaning up load balancers in region: ${region}"
    
    # Classic load balancers
    local elbs
    elbs=$(aws elb describe-load-balancers \
        --region "$region" \
        --query 'LoadBalancerDescriptions[?contains(LoadBalancerName, `cluster-api`) || contains(LoadBalancerName, `capi`)].LoadBalancerName' \
        --output text 2>/dev/null || true)
    
    if [[ -n "$elbs" && "$elbs" != "None" ]]; then
        for elb in $elbs; do
            log "Deleting classic load balancer: $elb"
            aws elb delete-load-balancer --region "$region" --load-balancer-name "$elb" || true
        done
    fi
    
    # Application load balancers
    local albs
    albs=$(aws elbv2 describe-load-balancers \
        --region "$region" \
        --query 'LoadBalancers[?contains(LoadBalancerName, `cluster-api`) || contains(LoadBalancerName, `capi`)].LoadBalancerArn' \
        --output text 2>/dev/null || true)
    
    if [[ -n "$albs" && "$albs" != "None" ]]; then
        for alb in $albs; do
            log "Deleting application load balancer: $alb"
            aws elbv2 delete-load-balancer --region "$region" --load-balancer-arn "$alb" || true
        done
    fi
}

# Kill port-forward processes
cleanup_port_forwards() {
    log "Cleaning up port-forward processes..."
    
    # Kill any kubectl port-forward processes
    pkill -f "kubectl.*port-forward" || true
    
    success "Port-forward processes cleaned up"
}

# Delete kind cluster
cleanup_kind_cluster() {
    log "Cleaning up kind cluster: ${KIND_CLUSTER_NAME}"
    
    if kind get clusters | grep -q "^${KIND_CLUSTER_NAME}$"; then
        kind delete cluster --name "${KIND_CLUSTER_NAME}"
        success "Kind cluster deleted"
    else
        log "Kind cluster ${KIND_CLUSTER_NAME} does not exist"
    fi
}

# Clean up Docker images
cleanup_docker_images() {
    log "Cleaning up Docker images..."
    
    # Remove MCP server test images
    docker rmi capi-mcp-server:latest 2>/dev/null || true
    
    # Clean up dangling images
    docker image prune -f || true
    
    success "Docker images cleaned up"
}

# Main execution
main() {
    log "Starting E2E test environment cleanup..."
    
    confirm_cleanup
    
    cleanup_port_forwards
    cleanup_capi_clusters
    cleanup_aws_resources
    cleanup_kind_cluster
    cleanup_docker_images
    
    success "E2E test environment cleanup completed!"
    log ""
    log "Cleanup Summary:"
    log "  - Kind cluster: ${KIND_CLUSTER_NAME} (deleted)"
    log "  - CAPI clusters: (deleted)"
    if [[ "${CLEANUP_AWS}" == "true" ]]; then
        log "  - AWS resources: (cleaned up in ${AWS_REGION})"
    else
        log "  - AWS resources: (skipped)"
    fi
    log ""
    log "Note: It may take several minutes for AWS resources to be fully deleted"
    log "Check the AWS console to verify all resources have been removed"
}

# Run main function
main "$@"