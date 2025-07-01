package aws

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/runtime"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

// AWSProvider implements the Provider interface for Amazon Web Services.
// This implementation provides AWS-specific logic for cluster operations
// using the Cluster API Provider AWS (CAPA).
type AWSProvider struct {
	// region is the default AWS region for operations
	region string
}

// NewAWSProvider creates a new AWS provider instance.
func NewAWSProvider(region string) *AWSProvider {
	if region == "" {
		region = "us-west-2" // Default region
	}

	return &AWSProvider{
		region: region,
	}
}

// Name returns the provider name.
func (p *AWSProvider) Name() string {
	return "aws"
}

// ValidateClusterConfig validates AWS-specific cluster configuration.
func (p *AWSProvider) ValidateClusterConfig(ctx context.Context, variables map[string]interface{}) error {
	// Validate required AWS-specific variables
	if region, ok := variables["region"]; ok {
		if regionStr, ok := region.(string); ok {
			if !p.isValidAWSRegion(regionStr) {
				return fmt.Errorf("invalid AWS region: %s", regionStr)
			}
		} else {
			return fmt.Errorf("region must be a string")
		}
	}

	// Validate instance type if provided
	if instanceType, ok := variables["instanceType"]; ok {
		if instanceTypeStr, ok := instanceType.(string); ok {
			if !p.isValidInstanceType(instanceTypeStr) {
				return fmt.Errorf("invalid AWS instance type: %s", instanceTypeStr)
			}
		} else {
			return fmt.Errorf("instanceType must be a string")
		}
	}

	// Validate node count
	if nodeCount, ok := variables["nodeCount"]; ok {
		switch v := nodeCount.(type) {
		case int:
			if v < 1 || v > 100 {
				return fmt.Errorf("nodeCount must be between 1 and 100, got %d", v)
			}
		case float64:
			intVal := int(v)
			if float64(intVal) != v || intVal < 1 || intVal > 100 {
				return fmt.Errorf("nodeCount must be an integer between 1 and 100, got %f", v)
			}
		default:
			return fmt.Errorf("nodeCount must be an integer")
		}
	}

	return nil
}

// GetSupportedKubernetesVersions returns supported Kubernetes versions for AWS.
func (p *AWSProvider) GetSupportedKubernetesVersions(ctx context.Context) ([]string, error) {
	// These versions should ideally be fetched from the CAPA provider or AWS EKS
	// For now, return a static list of commonly supported versions
	return []string{
		"v1.31.0",
		"v1.30.5",
		"v1.29.9",
		"v1.28.14",
	}, nil
}

// GetDefaultMachineTemplate returns the default AWS machine template.
func (p *AWSProvider) GetDefaultMachineTemplate(ctx context.Context) (runtime.Object, error) {
	// In a real implementation, this would return an AWSMachineTemplate object
	// For now, return nil as this is a stub implementation
	// TODO: Implement actual AWSMachineTemplate creation
	return nil, fmt.Errorf("GetDefaultMachineTemplate not yet implemented for AWS provider")
}

// GetInfrastructureTemplate returns the AWS infrastructure template.
func (p *AWSProvider) GetInfrastructureTemplate(ctx context.Context, variables map[string]interface{}) (runtime.Object, error) {
	// In a real implementation, this would return an AWSCluster object
	// configured with the provided variables
	// TODO: Implement actual AWSCluster template creation
	return nil, fmt.Errorf("GetInfrastructureTemplate not yet implemented for AWS provider")
}

// ValidateInfrastructureReadiness checks AWS infrastructure readiness.
func (p *AWSProvider) ValidateInfrastructureReadiness(ctx context.Context, cluster *clusterv1.Cluster) error {
	// Check if the cluster has an infrastructure reference
	if cluster.Spec.InfrastructureRef == nil {
		return fmt.Errorf("cluster %s has no infrastructure reference", cluster.Name)
	}

	// Verify it's an AWS infrastructure type
	if cluster.Spec.InfrastructureRef.Kind != "AWSCluster" {
		return fmt.Errorf("cluster %s infrastructure is not an AWSCluster (got %s)",
			cluster.Name, cluster.Spec.InfrastructureRef.Kind)
	}

	// In a real implementation, this would check AWS-specific infrastructure status
	// such as VPC readiness, subnet availability, security groups, etc.
	// For now, just check basic cluster status
	if !cluster.Status.InfrastructureReady {
		return fmt.Errorf("AWS infrastructure for cluster %s is not ready", cluster.Name)
	}

	return nil
}

// GetProviderSpecificStatus extracts AWS-specific status information.
func (p *AWSProvider) GetProviderSpecificStatus(ctx context.Context, cluster *clusterv1.Cluster) (map[string]interface{}, error) {
	status := make(map[string]interface{})

	// Extract basic AWS information
	if cluster.Spec.InfrastructureRef != nil {
		status["infrastructureKind"] = cluster.Spec.InfrastructureRef.Kind
		status["infrastructureName"] = cluster.Spec.InfrastructureRef.Name
	}

	// Extract region information from cluster variables or use default
	if cluster.Spec.Topology != nil && cluster.Spec.Topology.Variables != nil {
		for _, variable := range cluster.Spec.Topology.Variables {
			if variable.Name == "region" {
				if variable.Value.Raw != nil {
					var region string
					if err := json.Unmarshal(variable.Value.Raw, &region); err == nil {
						status["region"] = region
					}
				}
			}
		}
	}

	// Use default region if not found in cluster
	if _, hasRegion := status["region"]; !hasRegion {
		status["region"] = p.region
	}

	// Add provider-specific status
	status["provider"] = "aws"
	status["ready"] = cluster.Status.InfrastructureReady

	return status, nil
}

// GetRegions returns a list of AWS regions.
func (p *AWSProvider) GetRegions(ctx context.Context) ([]string, error) {
	// In a real implementation, this would query the AWS API for available regions
	// For now, return a static list of common AWS regions
	return []string{
		"us-east-1",      // N. Virginia
		"us-east-2",      // Ohio
		"us-west-1",      // N. California
		"us-west-2",      // Oregon
		"ca-central-1",   // Canada
		"eu-west-1",      // Ireland
		"eu-west-2",      // London
		"eu-west-3",      // Paris
		"eu-central-1",   // Frankfurt
		"eu-north-1",     // Stockholm
		"ap-northeast-1", // Tokyo
		"ap-northeast-2", // Seoul
		"ap-southeast-1", // Singapore
		"ap-southeast-2", // Sydney
		"ap-south-1",     // Mumbai
		"sa-east-1",      // São Paulo
	}, nil
}

// GetInstanceTypes returns AWS instance types for a given region.
func (p *AWSProvider) GetInstanceTypes(ctx context.Context, region string) ([]string, error) {
	// Validate region
	if !p.isValidAWSRegion(region) {
		return nil, fmt.Errorf("invalid AWS region: %s", region)
	}

	// In a real implementation, this would query the AWS EC2 API for available instance types
	// For now, return a static list of common instance types
	return []string{
		// General Purpose
		"t3.micro", "t3.small", "t3.medium", "t3.large", "t3.xlarge", "t3.2xlarge",
		"m5.large", "m5.xlarge", "m5.2xlarge", "m5.4xlarge", "m5.8xlarge", "m5.12xlarge",
		"m6i.large", "m6i.xlarge", "m6i.2xlarge", "m6i.4xlarge", "m6i.8xlarge",

		// Compute Optimized
		"c5.large", "c5.xlarge", "c5.2xlarge", "c5.4xlarge", "c5.9xlarge", "c5.18xlarge",
		"c6i.large", "c6i.xlarge", "c6i.2xlarge", "c6i.4xlarge", "c6i.8xlarge",

		// Memory Optimized
		"r5.large", "r5.xlarge", "r5.2xlarge", "r5.4xlarge", "r5.8xlarge", "r5.12xlarge",
		"r6i.large", "r6i.xlarge", "r6i.2xlarge", "r6i.4xlarge", "r6i.8xlarge",
	}, nil
}

// isValidAWSRegion checks if the provided region is a valid AWS region.
func (p *AWSProvider) isValidAWSRegion(region string) bool {
	// Simple validation - check if it matches AWS region pattern
	// AWS regions follow the pattern: {region}-{direction}-{number}
	// e.g., us-west-2, eu-central-1, ap-southeast-1
	parts := strings.Split(region, "-")
	if len(parts) != 3 { // Must be exactly 3 parts
		return false
	}

	// Additional validation could be added here to check against
	// an authoritative list of AWS regions
	validPrefixes := []string{"us", "eu", "ap", "ca", "sa", "af", "me"}
	for _, prefix := range validPrefixes {
		if parts[0] == prefix {
			return true
		}
	}

	return false
}

// isValidInstanceType checks if the provided instance type is valid.
func (p *AWSProvider) isValidInstanceType(instanceType string) bool {
	// Simple validation - check if it matches AWS instance type pattern
	// AWS instance types follow the pattern: {family}{generation}.{size}
	// e.g., m5.large, c5.xlarge, t3.micro
	parts := strings.Split(instanceType, ".")
	if len(parts) != 2 {
		return false
	}

	// Check if family part contains both letters and numbers
	family := parts[0]
	if len(family) < 2 {
		return false
	}

	// Validate family format: letters followed by numbers (e.g., m5, c6i, t3)
	hasLetter := false
	hasNumber := false
	for _, char := range family {
		if char >= 'a' && char <= 'z' {
			hasLetter = true
		} else if char >= '0' && char <= '9' {
			hasNumber = true
		}
	}
	if !hasLetter || !hasNumber {
		return false
	}

	// Basic validation - first part should be family+generation, second part should be size
	validSizes := []string{"nano", "micro", "small", "medium", "large", "xlarge", "2xlarge",
		"3xlarge", "4xlarge", "8xlarge", "9xlarge", "12xlarge", "16xlarge", "18xlarge", "24xlarge"}

	for _, size := range validSizes {
		if parts[1] == size {
			return true
		}
	}

	return false
}
