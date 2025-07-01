package utils

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	elbv2types "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
)

// AWSUtil provides utilities for managing and validating AWS resources during E2E tests
type AWSUtil struct {
	logger    *slog.Logger
	ec2Client *ec2.Client
	elbClient *elasticloadbalancingv2.Client
	region    string
}

// NewAWSUtil creates a new AWS utility instance
func NewAWSUtil(logger *slog.Logger) *AWSUtil {
	return &AWSUtil{
		logger: logger.With("component", "aws_util"),
		region: getAWSRegion(),
	}
}

// HasCredentials checks if AWS credentials are available
func (a *AWSUtil) HasCredentials() bool {
	return os.Getenv("AWS_ACCESS_KEY_ID") != "" && os.Getenv("AWS_SECRET_ACCESS_KEY") != ""
}

// GetRegion returns the configured AWS region
func (a *AWSUtil) GetRegion() string {
	return a.region
}

// GetSSHKeyName returns the configured SSH key name for EC2 instances
func (a *AWSUtil) GetSSHKeyName() string {
	return os.Getenv("AWS_SSH_KEY_NAME")
}

// Initialize initializes the AWS clients
func (a *AWSUtil) Initialize(ctx context.Context) error {
	if a.ec2Client != nil {
		return nil // Already initialized
	}
	
	a.logger.Info("Initializing AWS clients", "region", a.region)
	
	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(a.region),
	)
	if err != nil {
		return fmt.Errorf("failed to load AWS config: %w", err)
	}
	
	a.ec2Client = ec2.NewFromConfig(cfg)
	a.elbClient = elasticloadbalancingv2.NewFromConfig(cfg)
	
	// Test credentials by making a simple API call
	_, err = a.ec2Client.DescribeRegions(ctx, &ec2.DescribeRegionsInput{})
	if err != nil {
		return fmt.Errorf("failed to test AWS credentials: %w", err)
	}
	
	a.logger.Info("AWS clients initialized successfully")
	return nil
}

// ListVPCs lists VPCs associated with a cluster
func (a *AWSUtil) ListVPCs(ctx context.Context, clusterName string) ([]types.Vpc, error) {
	if err := a.Initialize(ctx); err != nil {
		return nil, err
	}
	
	a.logger.Debug("Listing VPCs", "cluster", clusterName)
	
	// List all VPCs and filter by cluster tags
	input := &ec2.DescribeVpcsInput{
		Filters: []types.Filter{
			{
				Name:   aws.String("tag:sigs.k8s.io/cluster-api-provider-aws/cluster/" + clusterName),
				Values: []string{"owned"},
			},
		},
	}
	
	result, err := a.ec2Client.DescribeVpcs(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to describe VPCs: %w", err)
	}
	
	a.logger.Debug("Listed VPCs", "cluster", clusterName, "count", len(result.Vpcs))
	return result.Vpcs, nil
}

// ListSecurityGroups lists security groups associated with a cluster
func (a *AWSUtil) ListSecurityGroups(ctx context.Context, clusterName string) ([]types.SecurityGroup, error) {
	if err := a.Initialize(ctx); err != nil {
		return nil, err
	}
	
	a.logger.Debug("Listing security groups", "cluster", clusterName)
	
	input := &ec2.DescribeSecurityGroupsInput{
		Filters: []types.Filter{
			{
				Name:   aws.String("tag:sigs.k8s.io/cluster-api-provider-aws/cluster/" + clusterName),
				Values: []string{"owned"},
			},
		},
	}
	
	result, err := a.ec2Client.DescribeSecurityGroups(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to describe security groups: %w", err)
	}
	
	a.logger.Debug("Listed security groups", "cluster", clusterName, "count", len(result.SecurityGroups))
	return result.SecurityGroups, nil
}

// ListEC2Instances lists EC2 instances associated with a cluster
func (a *AWSUtil) ListEC2Instances(ctx context.Context, clusterName string) ([]types.Instance, error) {
	if err := a.Initialize(ctx); err != nil {
		return nil, err
	}
	
	a.logger.Debug("Listing EC2 instances", "cluster", clusterName)
	
	input := &ec2.DescribeInstancesInput{
		Filters: []types.Filter{
			{
				Name:   aws.String("tag:sigs.k8s.io/cluster-api-provider-aws/cluster/" + clusterName),
				Values: []string{"owned"},
			},
		},
	}
	
	result, err := a.ec2Client.DescribeInstances(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to describe instances: %w", err)
	}
	
	// Flatten instances from reservations
	var instances []types.Instance
	for _, reservation := range result.Reservations {
		instances = append(instances, reservation.Instances...)
	}
	
	a.logger.Debug("Listed EC2 instances", "cluster", clusterName, "count", len(instances))
	return instances, nil
}

// ListLoadBalancers lists load balancers associated with a cluster
func (a *AWSUtil) ListLoadBalancers(ctx context.Context, clusterName string) ([]elbv2types.LoadBalancer, error) {
	if err := a.Initialize(ctx); err != nil {
		return nil, err
	}
	
	a.logger.Debug("Listing load balancers", "cluster", clusterName)
	
	// List all load balancers first
	input := &elasticloadbalancingv2.DescribeLoadBalancersInput{}
	result, err := a.elbClient.DescribeLoadBalancers(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to describe load balancers: %w", err)
	}
	
	// Filter by cluster tags
	var clusterLBs []elbv2types.LoadBalancer
	for _, lb := range result.LoadBalancers {
		tags, err := a.elbClient.DescribeTags(ctx, &elasticloadbalancingv2.DescribeTagsInput{
			ResourceArns: []string{*lb.LoadBalancerArn},
		})
		if err != nil {
			a.logger.Warn("Failed to get load balancer tags", "lb_arn", *lb.LoadBalancerArn, "error", err)
			continue
		}
		
		// Check if this load balancer belongs to our cluster
		for _, tagDesc := range tags.TagDescriptions {
			for _, tag := range tagDesc.Tags {
				if *tag.Key == "sigs.k8s.io/cluster-api-provider-aws/cluster/"+clusterName && *tag.Value == "owned" {
					clusterLBs = append(clusterLBs, lb)
					break
				}
			}
		}
	}
	
	a.logger.Debug("Listed load balancers", "cluster", clusterName, "count", len(clusterLBs))
	return clusterLBs, nil
}

// FilterClusterVPCs filters VPCs that belong to the specified cluster
func (a *AWSUtil) FilterClusterVPCs(vpcs []types.Vpc, clusterName string) []types.Vpc {
	var clusterVPCs []types.Vpc
	
	for _, vpc := range vpcs {
		// Check for cluster tag
		for _, tag := range vpc.Tags {
			if tag.Key != nil && tag.Value != nil {
				if *tag.Key == "sigs.k8s.io/cluster-api-provider-aws/cluster/"+clusterName && *tag.Value == "owned" {
					clusterVPCs = append(clusterVPCs, vpc)
					break
				}
			}
		}
	}
	
	return clusterVPCs
}

// FilterClusterSecurityGroups filters security groups that belong to the specified cluster
func (a *AWSUtil) FilterClusterSecurityGroups(securityGroups []types.SecurityGroup, clusterName string) []types.SecurityGroup {
	var clusterSGs []types.SecurityGroup
	
	for _, sg := range securityGroups {
		// Check for cluster tag
		for _, tag := range sg.Tags {
			if tag.Key != nil && tag.Value != nil {
				if *tag.Key == "sigs.k8s.io/cluster-api-provider-aws/cluster/"+clusterName && *tag.Value == "owned" {
					clusterSGs = append(clusterSGs, sg)
					break
				}
			}
		}
	}
	
	return clusterSGs
}

// FilterRunningInstances filters instances that are currently running
func (a *AWSUtil) FilterRunningInstances(instances []types.Instance) []types.Instance {
	var runningInstances []types.Instance
	
	for _, instance := range instances {
		if instance.State != nil && instance.State.Name == types.InstanceStateNameRunning {
			runningInstances = append(runningInstances, instance)
		}
	}
	
	return runningInstances
}

// ValidateClusterResources validates that all expected AWS resources exist for a cluster
func (a *AWSUtil) ValidateClusterResources(ctx context.Context, clusterName string, expectedNodeCount int) error {
	a.logger.Info("Validating cluster AWS resources", "cluster", clusterName, "expected_nodes", expectedNodeCount)
	
	// Check VPC
	vpcs, err := a.ListVPCs(ctx, clusterName)
	if err != nil {
		return fmt.Errorf("failed to list VPCs: %w", err)
	}
	if len(vpcs) == 0 {
		return fmt.Errorf("no VPC found for cluster %s", clusterName)
	}
	
	// Check security groups
	securityGroups, err := a.ListSecurityGroups(ctx, clusterName)
	if err != nil {
		return fmt.Errorf("failed to list security groups: %w", err)
	}
	if len(securityGroups) == 0 {
		return fmt.Errorf("no security groups found for cluster %s", clusterName)
	}
	
	// Check instances
	instances, err := a.ListEC2Instances(ctx, clusterName)
	if err != nil {
		return fmt.Errorf("failed to list EC2 instances: %w", err)
	}
	
	runningInstances := a.FilterRunningInstances(instances)
	if len(runningInstances) < expectedNodeCount {
		return fmt.Errorf("expected at least %d running instances, found %d", expectedNodeCount, len(runningInstances))
	}
	
	a.logger.Info("Cluster AWS resources validated successfully",
		"cluster", clusterName,
		"vpcs", len(vpcs),
		"security_groups", len(securityGroups),
		"instances", len(instances),
		"running_instances", len(runningInstances),
	)
	
	return nil
}

// ValidateResourceCleanup validates that all AWS resources for a cluster have been cleaned up
func (a *AWSUtil) ValidateResourceCleanup(ctx context.Context, clusterName string) error {
	a.logger.Info("Validating AWS resource cleanup", "cluster", clusterName)
	
	// Check VPCs
	vpcs, err := a.ListVPCs(ctx, clusterName)
	if err != nil {
		return fmt.Errorf("failed to list VPCs: %w", err)
	}
	if len(vpcs) > 0 {
		return fmt.Errorf("found %d VPCs still associated with cluster %s", len(vpcs), clusterName)
	}
	
	// Check security groups
	securityGroups, err := a.ListSecurityGroups(ctx, clusterName)
	if err != nil {
		return fmt.Errorf("failed to list security groups: %w", err)
	}
	if len(securityGroups) > 0 {
		return fmt.Errorf("found %d security groups still associated with cluster %s", len(securityGroups), clusterName)
	}
	
	// Check instances
	instances, err := a.ListEC2Instances(ctx, clusterName)
	if err != nil {
		return fmt.Errorf("failed to list EC2 instances: %w", err)
	}
	
	runningInstances := a.FilterRunningInstances(instances)
	if len(runningInstances) > 0 {
		return fmt.Errorf("found %d running instances still associated with cluster %s", len(runningInstances), clusterName)
	}
	
	// Check load balancers
	loadBalancers, err := a.ListLoadBalancers(ctx, clusterName)
	if err != nil {
		return fmt.Errorf("failed to list load balancers: %w", err)
	}
	if len(loadBalancers) > 0 {
		return fmt.Errorf("found %d load balancers still associated with cluster %s", len(loadBalancers), clusterName)
	}
	
	a.logger.Info("AWS resource cleanup validated successfully", "cluster", clusterName)
	return nil
}

// GetInstanceDetails returns detailed information about an EC2 instance
func (a *AWSUtil) GetInstanceDetails(instance types.Instance) map[string]interface{} {
	details := map[string]interface{}{
		"instance_id":   aws.ToString(instance.InstanceId),
		"instance_type": string(instance.InstanceType),
		"state":         string(instance.State.Name),
		"public_ip":     aws.ToString(instance.PublicIpAddress),
		"private_ip":    aws.ToString(instance.PrivateIpAddress),
	}
	
	// Extract useful tags
	tags := make(map[string]string)
	for _, tag := range instance.Tags {
		if tag.Key != nil && tag.Value != nil {
			tags[*tag.Key] = *tag.Value
		}
	}
	details["tags"] = tags
	
	// Add role information if available
	if name, ok := tags["Name"]; ok {
		if strings.Contains(strings.ToLower(name), "control-plane") {
			details["role"] = "control-plane"
		} else if strings.Contains(strings.ToLower(name), "worker") {
			details["role"] = "worker"
		}
	}
	
	return details
}

// WaitForInstancesRunning waits for all cluster instances to be in running state
func (a *AWSUtil) WaitForInstancesRunning(ctx context.Context, clusterName string, expectedCount int) error {
	a.logger.Info("Waiting for instances to be running", "cluster", clusterName, "expected_count", expectedCount)
	
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			instances, err := a.ListEC2Instances(ctx, clusterName)
			if err != nil {
				return fmt.Errorf("failed to list instances: %w", err)
			}
			
			runningInstances := a.FilterRunningInstances(instances)
			if len(runningInstances) >= expectedCount {
				a.logger.Info("All expected instances are running",
					"cluster", clusterName,
					"running_count", len(runningInstances),
				)
				return nil
			}
			
			a.logger.Debug("Waiting for more instances to be running",
				"cluster", clusterName,
				"running_count", len(runningInstances),
				"expected_count", expectedCount,
			)
			
			// Wait before checking again
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(30 * time.Second):
				// Continue checking
			}
		}
	}
}

// CleanupClusterResources attempts to clean up any remaining AWS resources for a cluster
func (a *AWSUtil) CleanupClusterResources(ctx context.Context, clusterName string) error {
	a.logger.Info("Cleaning up AWS resources", "cluster", clusterName)
	
	// Note: This is a safety cleanup function for tests
	// In practice, CAPA should handle all cleanup automatically
	
	// Terminate any running instances
	instances, err := a.ListEC2Instances(ctx, clusterName)
	if err != nil {
		return fmt.Errorf("failed to list instances for cleanup: %w", err)
	}
	
	var instanceIds []string
	for _, instance := range instances {
		if instance.State.Name == types.InstanceStateNameRunning || 
		   instance.State.Name == types.InstanceStateNamePending {
			instanceIds = append(instanceIds, *instance.InstanceId)
		}
	}
	
	if len(instanceIds) > 0 {
		a.logger.Warn("Terminating instances for cleanup",
			"cluster", clusterName,
			"instance_count", len(instanceIds),
		)
		
		_, err = a.ec2Client.TerminateInstances(ctx, &ec2.TerminateInstancesInput{
			InstanceIds: instanceIds,
		})
		if err != nil {
			a.logger.Error("Failed to terminate instances", "error", err)
		}
	}
	
	a.logger.Info("AWS resource cleanup completed", "cluster", clusterName)
	return nil
}

// getAWSRegion gets the AWS region from environment or returns default
func getAWSRegion() string {
	if region := os.Getenv("AWS_REGION"); region != "" {
		return region
	}
	if region := os.Getenv("AWS_DEFAULT_REGION"); region != "" {
		return region
	}
	return "us-west-2" // Default region
}