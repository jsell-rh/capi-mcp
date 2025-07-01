package validation

import (
	"fmt"
	"net"
	"regexp"
	"strings"

	"github.com/capi-mcp/capi-mcp-server/internal/errors"
)

var (
	// DNS subdomain regex (RFC 1123)
	dnsSubdomainRegex = regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$`)
	
	// Kubernetes version regex
	kubernetesVersionRegex = regexp.MustCompile(`^v\d+\.\d+\.\d+(-[\w\.-]+)?$`)
	
	// Resource name regex
	resourceNameRegex = regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`)
)

// Validator provides input validation functions
type Validator struct{}

// NewValidator creates a new validator instance
func NewValidator() *Validator {
	return &Validator{}
}

// ValidateClusterName validates a cluster name
func (v *Validator) ValidateClusterName(name string) error {
	if name == "" {
		return errors.New(errors.CodeInvalidInput, "cluster name cannot be empty")
	}
	
	if len(name) > 63 {
		return errors.New(errors.CodeInvalidInput, "cluster name must be 63 characters or less")
	}
	
	if !resourceNameRegex.MatchString(name) {
		return errors.New(errors.CodeInvalidInput, 
			"cluster name must consist of lowercase alphanumeric characters or '-', and must start and end with an alphanumeric character")
	}
	
	return nil
}

// ValidateNamespace validates a namespace name
func (v *Validator) ValidateNamespace(namespace string) error {
	if namespace == "" {
		return errors.New(errors.CodeInvalidInput, "namespace cannot be empty")
	}
	
	if len(namespace) > 63 {
		return errors.New(errors.CodeInvalidInput, "namespace must be 63 characters or less")
	}
	
	if !resourceNameRegex.MatchString(namespace) {
		return errors.New(errors.CodeInvalidInput,
			"namespace must consist of lowercase alphanumeric characters or '-', and must start and end with an alphanumeric character")
	}
	
	return nil
}

// ValidateKubernetesVersion validates a Kubernetes version string
func (v *Validator) ValidateKubernetesVersion(version string) error {
	if version == "" {
		return errors.New(errors.CodeInvalidInput, "kubernetes version cannot be empty")
	}
	
	if !kubernetesVersionRegex.MatchString(version) {
		return errors.New(errors.CodeInvalidInput,
			"kubernetes version must be in format 'vX.Y.Z' (e.g., v1.28.0)")
	}
	
	// Extract major and minor version
	parts := strings.Split(version[1:], ".")
	if len(parts) < 3 {
		return errors.New(errors.CodeInvalidInput, "invalid kubernetes version format")
	}
	
	return nil
}

// ValidateMachineDeploymentName validates a MachineDeployment name
func (v *Validator) ValidateMachineDeploymentName(name string) error {
	if name == "" {
		return errors.New(errors.CodeInvalidInput, "machine deployment name cannot be empty")
	}
	
	if len(name) > 253 {
		return errors.New(errors.CodeInvalidInput, "machine deployment name must be 253 characters or less")
	}
	
	if !dnsSubdomainRegex.MatchString(name) {
		return errors.New(errors.CodeInvalidInput,
			"machine deployment name must be a valid DNS subdomain")
	}
	
	return nil
}

// ValidateReplicaCount validates the number of replicas
func (v *Validator) ValidateReplicaCount(replicas int32) error {
	if replicas < 0 {
		return errors.New(errors.CodeInvalidInput, "replica count cannot be negative")
	}
	
	if replicas > 100 {
		return errors.New(errors.CodeInvalidInput, "replica count cannot exceed 100")
	}
	
	return nil
}

// ValidateAPIKey validates an API key format
func (v *Validator) ValidateAPIKey(apiKey string) error {
	if apiKey == "" {
		return errors.New(errors.CodeInvalidInput, "API key cannot be empty")
	}
	
	if len(apiKey) < 32 {
		return errors.New(errors.CodeInvalidInput, "API key must be at least 32 characters")
	}
	
	// Check for common weak patterns
	if strings.ToLower(apiKey) == apiKey || strings.ToUpper(apiKey) == apiKey {
		return errors.New(errors.CodeInvalidInput, "API key must contain mixed case characters")
	}
	
	return nil
}

// ValidateClusterVariables validates cluster creation variables
func (v *Validator) ValidateClusterVariables(variables map[string]interface{}) error {
	if variables == nil {
		return errors.New(errors.CodeInvalidInput, "cluster variables cannot be nil").
			WithDetails("field", "variables")
	}
	
	// Track validation errors for comprehensive feedback
	var validationErrors []error
	
	// Check for required common variables
	for key, value := range variables {
		switch key {
		case "nodeCount":
			if err := v.validateNodeCount(value); err != nil {
				validationErrors = append(validationErrors, err)
			}
			
		case "region":
			if err := v.validateRegion(value); err != nil {
				validationErrors = append(validationErrors, err)
			}
			
		case "instanceType", "controlPlaneInstanceType", "workerInstanceType":
			if err := v.validateInstanceType(key, value); err != nil {
				validationErrors = append(validationErrors, err)
			}
			
		case "vpcCIDR", "subnetCIDR":
			if err := v.validateCIDR(key, value); err != nil {
				validationErrors = append(validationErrors, err)
			}
			
		case "sshKeyName":
			if err := v.validateSSHKeyName(value); err != nil {
				validationErrors = append(validationErrors, err)
			}
			
		// Additional variables that should be validated
		case "kubernetesVersion":
			if version, ok := value.(string); ok {
				if err := v.ValidateKubernetesVersion(version); err != nil {
					validationErrors = append(validationErrors, err)
				}
			}
		}
	}
	
	// Return combined validation errors if any
	if len(validationErrors) > 0 {
		return v.combineValidationErrors(validationErrors)
	}
	
	return nil
}

// validateNodeCount validates node count with detailed error messages
func (v *Validator) validateNodeCount(value interface{}) error {
	count, ok := toInt32(value)
	if !ok {
		return errors.New(errors.CodeInvalidInput, 
			"nodeCount must be a positive integer (e.g., 2, 5, 10)").
			WithDetails("field", "nodeCount").
			WithDetails("provided_type", fmt.Sprintf("%T", value))
	}
	
	if err := v.ValidateReplicaCount(count); err != nil {
		// Enhance the error message with more context
		if count < 0 {
			return errors.New(errors.CodeInvalidInput, 
				"nodeCount cannot be negative - clusters need at least 0 worker nodes").
				WithDetails("field", "nodeCount").
				WithDetails("provided_value", count)
		}
		if count > 100 {
			return errors.New(errors.CodeInvalidInput, 
				"nodeCount cannot exceed 100 - this limit prevents excessive resource usage").
				WithDetails("field", "nodeCount").
				WithDetails("provided_value", count).
				WithDetails("max_allowed", 100)
		}
	}
	
	return nil
}

// validateRegion validates AWS region with helpful suggestions
func (v *Validator) validateRegion(value interface{}) error {
	region, ok := value.(string)
	if !ok {
		return errors.New(errors.CodeInvalidInput, 
			"region must be a string (e.g., 'us-west-2', 'eu-central-1')").
			WithDetails("field", "region").
			WithDetails("provided_type", fmt.Sprintf("%T", value))
	}
	
	if region == "" {
		return errors.New(errors.CodeInvalidInput, 
			"region cannot be empty - specify an AWS region like 'us-west-2' or 'eu-central-1'").
			WithDetails("field", "region")
	}
	
	// Validate AWS region format
	if err := v.ValidateAWSRegion(region); err != nil {
		return err
	}
	
	return nil
}

// validateInstanceType validates instance types with helpful examples
func (v *Validator) validateInstanceType(fieldName string, value interface{}) error {
	instanceType, ok := value.(string)
	if !ok {
		return errors.New(errors.CodeInvalidInput, 
			fmt.Sprintf("%s must be a string (e.g., 't3.medium', 'm5.large')", fieldName)).
			WithDetails("field", fieldName).
			WithDetails("provided_type", fmt.Sprintf("%T", value))
	}
	
	if instanceType == "" {
		return errors.New(errors.CodeInvalidInput, 
			fmt.Sprintf("%s cannot be empty - specify an EC2 instance type like 't3.medium' or 'm5.large'", fieldName)).
			WithDetails("field", fieldName)
	}
	
	// Validate AWS instance type format
	if err := v.ValidateAWSInstanceType(instanceType); err != nil {
		return errors.New(errors.CodeInvalidInput, 
			fmt.Sprintf("%s '%s' is not a valid EC2 instance type - use formats like 't3.medium', 'm5.large', 'c5.xlarge'", fieldName, instanceType)).
			WithDetails("field", fieldName).
			WithDetails("provided_value", instanceType)
	}
	
	return nil
}

// validateCIDR validates CIDR blocks with examples
func (v *Validator) validateCIDR(fieldName string, value interface{}) error {
	cidr, ok := value.(string)
	if !ok {
		return errors.New(errors.CodeInvalidInput, 
			fmt.Sprintf("%s must be a string in CIDR format (e.g., '10.0.0.0/16')", fieldName)).
			WithDetails("field", fieldName).
			WithDetails("provided_type", fmt.Sprintf("%T", value))
	}
	
	if cidr == "" {
		return errors.New(errors.CodeInvalidInput, 
			fmt.Sprintf("%s cannot be empty - specify a CIDR block like '10.0.0.0/16' or '192.168.1.0/24'", fieldName)).
			WithDetails("field", fieldName)
	}
	
	if err := v.ValidateCIDR(cidr); err != nil {
		return errors.New(errors.CodeInvalidInput, 
			fmt.Sprintf("%s '%s' is not a valid CIDR block - use format like '10.0.0.0/16' or '192.168.1.0/24'", fieldName, cidr)).
			WithDetails("field", fieldName).
			WithDetails("provided_value", cidr)
	}
	
	return nil
}

// validateSSHKeyName validates SSH key names
func (v *Validator) validateSSHKeyName(value interface{}) error {
	if value == nil {
		return nil // SSH key is optional
	}
	
	keyName, ok := value.(string)
	if !ok {
		return errors.New(errors.CodeInvalidInput, 
			"sshKeyName must be a string (name of your EC2 key pair)").
			WithDetails("field", "sshKeyName").
			WithDetails("provided_type", fmt.Sprintf("%T", value))
	}
	
	if keyName == "" {
		return nil // Empty string is acceptable (no SSH key)
	}
	
	// Validate EC2 key pair name format
	if err := v.ValidateEC2KeyName(keyName); err != nil {
		return errors.New(errors.CodeInvalidInput, 
			fmt.Sprintf("sshKeyName '%s' is not a valid EC2 key pair name - use only alphanumeric characters, spaces, and ._-:+=@", keyName)).
			WithDetails("field", "sshKeyName").
			WithDetails("provided_value", keyName)
	}
	
	return nil
}

// combineValidationErrors combines multiple validation errors into a single descriptive error
func (v *Validator) combineValidationErrors(validationErrors []error) error {
	if len(validationErrors) == 1 {
		return validationErrors[0]
	}
	
	var errorMessages []string
	var allDetails = make(map[string]interface{})
	
	for i, err := range validationErrors {
		errorMessages = append(errorMessages, fmt.Sprintf("%d. %s", i+1, err.Error()))
		
		// Combine details from all errors
		if e, ok := err.(*errors.Error); ok && e.Details != nil {
			for k, v := range e.Details {
				allDetails[k] = v
			}
		}
	}
	
	combinedMessage := fmt.Sprintf("Multiple validation errors:\n%s", strings.Join(errorMessages, "\n"))
	
	return errors.New(errors.CodeInvalidInput, combinedMessage).
		WithDetailsMap(allDetails)
}

// ValidateIPAddress validates an IP address
func (v *Validator) ValidateIPAddress(ip string) error {
	if ip == "" {
		return errors.New(errors.CodeInvalidInput, "IP address cannot be empty")
	}
	
	if net.ParseIP(ip) == nil {
		return errors.New(errors.CodeInvalidInput, fmt.Sprintf("invalid IP address: %s", ip))
	}
	
	return nil
}

// ValidatePort validates a port number
func (v *Validator) ValidatePort(port int) error {
	if port < 1 || port > 65535 {
		return errors.New(errors.CodeInvalidInput, "port must be between 1 and 65535")
	}
	
	return nil
}

// ValidateDNSName validates a DNS hostname
func (v *Validator) ValidateDNSName(name string) error {
	if name == "" {
		return errors.New(errors.CodeInvalidInput, "DNS name cannot be empty")
	}
	
	if len(name) > 253 {
		return errors.New(errors.CodeInvalidInput, "DNS name must be 253 characters or less")
	}
	
	if !dnsSubdomainRegex.MatchString(strings.ToLower(name)) {
		return errors.New(errors.CodeInvalidInput, "invalid DNS name format")
	}
	
	return nil
}

// Helper functions

// toInt32 converts various numeric types to int32
func toInt32(v interface{}) (int32, bool) {
	switch val := v.(type) {
	case int:
		return int32(val), true
	case int32:
		return val, true
	case int64:
		return int32(val), true
	case float32:
		return int32(val), true
	case float64:
		return int32(val), true
	default:
		return 0, false
	}
}

// SanitizeClusterName sanitizes a cluster name to make it valid
func SanitizeClusterName(name string) string {
	// Convert to lowercase
	name = strings.ToLower(name)
	
	// Replace invalid characters with hyphens
	sanitized := make([]rune, 0, len(name))
	for _, ch := range name {
		if (ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') || ch == '-' {
			sanitized = append(sanitized, ch)
		} else if len(sanitized) > 0 && sanitized[len(sanitized)-1] != '-' {
			sanitized = append(sanitized, '-')
		}
	}
	
	// Trim hyphens from start and end
	result := strings.Trim(string(sanitized), "-")
	
	// Ensure it starts with a letter if it starts with a number
	if len(result) > 0 && result[0] >= '0' && result[0] <= '9' {
		result = "cluster-" + result
	}
	
	// Truncate to max length
	if len(result) > 63 {
		result = result[:63]
		// Ensure it doesn't end with a hyphen after truncation
		result = strings.TrimRight(result, "-")
	}
	
	// If empty after sanitization, use a default
	if result == "" {
		result = "cluster"
	}
	
	return result
}

// ValidateAWSRegion validates AWS region format and known regions
func (v *Validator) ValidateAWSRegion(region string) error {
	if region == "" {
		return errors.New(errors.CodeInvalidInput, "AWS region cannot be empty")
	}
	
	// AWS region format: 2-3 letter region + dash + direction + dash + number
	awsRegionRegex := regexp.MustCompile(`^[a-z]{2,3}-[a-z]+-\d+$`)
	if !awsRegionRegex.MatchString(region) {
		return errors.New(errors.CodeInvalidInput, 
			fmt.Sprintf("'%s' is not a valid AWS region format - use format like 'us-west-2' or 'eu-central-1'", region))
	}
	
	// List of common valid AWS regions (not exhaustive, but covers most cases)
	validRegions := map[string]bool{
		"us-east-1":      true, // US East (N. Virginia)
		"us-east-2":      true, // US East (Ohio)
		"us-west-1":      true, // US West (N. California)
		"us-west-2":      true, // US West (Oregon)
		"eu-west-1":      true, // Europe (Ireland)
		"eu-west-2":      true, // Europe (London)
		"eu-west-3":      true, // Europe (Paris)
		"eu-central-1":   true, // Europe (Frankfurt)
		"eu-north-1":     true, // Europe (Stockholm)
		"ap-south-1":     true, // Asia Pacific (Mumbai)
		"ap-southeast-1": true, // Asia Pacific (Singapore)
		"ap-southeast-2": true, // Asia Pacific (Sydney)
		"ap-northeast-1": true, // Asia Pacific (Tokyo)
		"ap-northeast-2": true, // Asia Pacific (Seoul)
		"ap-northeast-3": true, // Asia Pacific (Osaka)
		"ca-central-1":   true, // Canada (Central)
		"sa-east-1":      true, // South America (São Paulo)
	}
	
	if !validRegions[region] {
		return errors.New(errors.CodeInvalidInput, 
			fmt.Sprintf("'%s' is not a known AWS region - common regions include us-west-2, eu-central-1, ap-southeast-1", region))
	}
	
	return nil
}

// ValidateAWSInstanceType validates AWS EC2 instance type format
func (v *Validator) ValidateAWSInstanceType(instanceType string) error {
	if instanceType == "" {
		return errors.New(errors.CodeInvalidInput, "instance type cannot be empty")
	}
	
	// AWS instance type format: family + generation + size
	// Examples: t3.micro, m5.large, c5.4xlarge, r5d.24xlarge
	instanceTypeRegex := regexp.MustCompile(`^[a-z][0-9]+[a-z]*\.(nano|micro|small|medium|large|xlarge|[0-9]+xlarge)$`)
	if !instanceTypeRegex.MatchString(instanceType) {
		return errors.New(errors.CodeInvalidInput, 
			fmt.Sprintf("'%s' is not a valid EC2 instance type format - use formats like 't3.medium', 'm5.large'", instanceType))
	}
	
	return nil
}

// ValidateCIDR validates CIDR block format
func (v *Validator) ValidateCIDR(cidr string) error {
	if cidr == "" {
		return errors.New(errors.CodeInvalidInput, "CIDR block cannot be empty")
	}
	
	// Parse CIDR
	_, ipNet, err := net.ParseCIDR(cidr)
	if err != nil {
		return errors.New(errors.CodeInvalidInput, 
			fmt.Sprintf("'%s' is not a valid CIDR block - use format like '10.0.0.0/16'", cidr))
	}
	
	// Additional validation for reasonable CIDR ranges
	ones, bits := ipNet.Mask.Size()
	if bits != 32 && bits != 128 {
		return errors.New(errors.CodeInvalidInput, "CIDR block must be IPv4 or IPv6")
	}
	
	// For IPv4, check for reasonable subnet sizes
	if bits == 32 {
		if ones < 8 {
			return errors.New(errors.CodeInvalidInput, 
				"CIDR block subnet mask too large - use /8 or smaller (e.g., /16, /24)")
		}
		if ones > 28 {
			return errors.New(errors.CodeInvalidInput, 
				"CIDR block subnet mask too small - use /28 or larger (e.g., /24, /16)")
		}
	}
	
	return nil
}

// ValidateEC2KeyName validates EC2 key pair name format
func (v *Validator) ValidateEC2KeyName(keyName string) error {
	if keyName == "" {
		return errors.New(errors.CodeInvalidInput, "EC2 key name cannot be empty")
	}
	
	if len(keyName) > 255 {
		return errors.New(errors.CodeInvalidInput, "EC2 key name must be 255 characters or less")
	}
	
	// EC2 key names can contain alphanumeric characters, spaces, and ._-:+=@
	ec2KeyNameRegex := regexp.MustCompile(`^[a-zA-Z0-9 ._\-:+=@]+$`)
	if !ec2KeyNameRegex.MatchString(keyName) {
		return errors.New(errors.CodeInvalidInput, 
			"EC2 key name contains invalid characters - use only alphanumeric characters, spaces, and ._-:+=@")
	}
	
	return nil
}

// ValidateTemplateName validates ClusterClass template name
func (v *Validator) ValidateTemplateName(templateName string) error {
	if templateName == "" {
		return errors.New(errors.CodeInvalidInput, "template name cannot be empty")
	}
	
	if len(templateName) > 253 {
		return errors.New(errors.CodeInvalidInput, "template name must be 253 characters or less")
	}
	
	if !dnsSubdomainRegex.MatchString(templateName) {
		return errors.New(errors.CodeInvalidInput,
			"template name must be a valid DNS subdomain (lowercase letters, numbers, dots, and hyphens)")
	}
	
	return nil
}

// ValidateNodePoolName validates node pool/MachineDeployment name with better error messages
func (v *Validator) ValidateNodePoolName(nodePoolName string) error {
	if nodePoolName == "" {
		return errors.New(errors.CodeInvalidInput, 
			"node pool name cannot be empty - specify a name like 'workers' or 'default-worker'")
	}
	
	if len(nodePoolName) > 253 {
		return errors.New(errors.CodeInvalidInput, 
			"node pool name must be 253 characters or less")
	}
	
	if !dnsSubdomainRegex.MatchString(nodePoolName) {
		return errors.New(errors.CodeInvalidInput,
			"node pool name must be a valid DNS subdomain - use lowercase letters, numbers, dots, and hyphens only")
	}
	
	return nil
}

// ValidateCreateClusterInput validates the complete create cluster input
func (v *Validator) ValidateCreateClusterInput(input map[string]interface{}) error {
	var validationErrors []error
	
	// Validate cluster name
	if clusterName, ok := input["clusterName"].(string); ok {
		if err := v.ValidateClusterName(clusterName); err != nil {
			validationErrors = append(validationErrors, err)
		}
	} else {
		validationErrors = append(validationErrors, 
			errors.New(errors.CodeInvalidInput, "clusterName is required and must be a string").
				WithDetails("field", "clusterName"))
	}
	
	// Validate template name
	if templateName, ok := input["templateName"].(string); ok {
		if err := v.ValidateTemplateName(templateName); err != nil {
			validationErrors = append(validationErrors, err)
		}
	} else {
		validationErrors = append(validationErrors, 
			errors.New(errors.CodeInvalidInput, "templateName is required and must be a string").
				WithDetails("field", "templateName"))
	}
	
	// Validate Kubernetes version
	if kubernetesVersion, ok := input["kubernetesVersion"].(string); ok {
		if err := v.ValidateKubernetesVersion(kubernetesVersion); err != nil {
			validationErrors = append(validationErrors, err)
		}
	} else {
		validationErrors = append(validationErrors, 
			errors.New(errors.CodeInvalidInput, "kubernetesVersion is required and must be a string in format 'vX.Y.Z'").
				WithDetails("field", "kubernetesVersion"))
	}
	
	// Validate variables if present
	if variables, ok := input["variables"].(map[string]interface{}); ok {
		if err := v.ValidateClusterVariables(variables); err != nil {
			validationErrors = append(validationErrors, err)
		}
	}
	
	// Return combined validation errors if any
	if len(validationErrors) > 0 {
		return v.combineValidationErrors(validationErrors)
	}
	
	return nil
}

// ValidateScaleClusterInput validates the complete scale cluster input
func (v *Validator) ValidateScaleClusterInput(input map[string]interface{}) error {
	var validationErrors []error
	
	// Validate cluster name
	if clusterName, ok := input["clusterName"].(string); ok {
		if err := v.ValidateClusterName(clusterName); err != nil {
			validationErrors = append(validationErrors, err)
		}
	} else {
		validationErrors = append(validationErrors, 
			errors.New(errors.CodeInvalidInput, "clusterName is required and must be a string").
				WithDetails("field", "clusterName"))
	}
	
	// Validate node pool name
	if nodePoolName, ok := input["nodePoolName"].(string); ok {
		if err := v.ValidateNodePoolName(nodePoolName); err != nil {
			validationErrors = append(validationErrors, err)
		}
	} else {
		validationErrors = append(validationErrors, 
			errors.New(errors.CodeInvalidInput, "nodePoolName is required and must be a string").
				WithDetails("field", "nodePoolName"))
	}
	
	// Validate replicas
	if replicas, ok := toInt32(input["replicas"]); ok {
		if err := v.ValidateReplicaCount(replicas); err != nil {
			validationErrors = append(validationErrors, err)
		}
	} else {
		validationErrors = append(validationErrors, 
			errors.New(errors.CodeInvalidInput, "replicas is required and must be a number between 0 and 100").
				WithDetails("field", "replicas"))
	}
	
	// Return combined validation errors if any
	if len(validationErrors) > 0 {
		return v.combineValidationErrors(validationErrors)
	}
	
	return nil
}