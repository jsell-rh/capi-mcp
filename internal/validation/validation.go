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
	// Check for required common variables
	for key, value := range variables {
		switch key {
		case "nodeCount":
			count, ok := toInt32(value)
			if !ok {
				return errors.New(errors.CodeInvalidInput, "nodeCount must be a number").
					WithDetails("field", "nodeCount")
			}
			if err := v.ValidateReplicaCount(count); err != nil {
				return errors.Wrap(err, errors.CodeInvalidInput, "invalid nodeCount")
			}
			
		case "region":
			region, ok := value.(string)
			if !ok || region == "" {
				return errors.New(errors.CodeInvalidInput, "region must be a non-empty string").
					WithDetails("field", "region")
			}
			
		case "instanceType":
			instanceType, ok := value.(string)
			if !ok || instanceType == "" {
				return errors.New(errors.CodeInvalidInput, "instanceType must be a non-empty string").
					WithDetails("field", "instanceType")
			}
		}
	}
	
	return nil
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