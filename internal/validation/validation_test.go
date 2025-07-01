package validation

import (
	"testing"

	"github.com/capi-mcp/capi-mcp-server/internal/errors"
)

func TestValidator_ValidateClusterName(t *testing.T) {
	v := NewValidator()

	tests := []struct {
		name        string
		input       string
		expectError bool
		errorCode   errors.ErrorCode
	}{
		{
			name:        "valid cluster name",
			input:       "my-cluster",
			expectError: false,
		},
		{
			name:        "valid with numbers",
			input:       "cluster-123",
			expectError: false,
		},
		{
			name:        "valid single character",
			input:       "a",
			expectError: false,
		},
		{
			name:        "empty name",
			input:       "",
			expectError: true,
			errorCode:   errors.CodeInvalidInput,
		},
		{
			name:        "too long",
			input:       "a-very-long-cluster-name-that-exceeds-the-maximum-allowed-length-of-sixty-three-characters",
			expectError: true,
			errorCode:   errors.CodeInvalidInput,
		},
		{
			name:        "starts with hyphen",
			input:       "-cluster",
			expectError: true,
			errorCode:   errors.CodeInvalidInput,
		},
		{
			name:        "ends with hyphen",
			input:       "cluster-",
			expectError: true,
			errorCode:   errors.CodeInvalidInput,
		},
		{
			name:        "contains uppercase",
			input:       "My-Cluster",
			expectError: true,
			errorCode:   errors.CodeInvalidInput,
		},
		{
			name:        "contains underscore",
			input:       "my_cluster",
			expectError: true,
			errorCode:   errors.CodeInvalidInput,
		},
		{
			name:        "contains special characters",
			input:       "my@cluster",
			expectError: true,
			errorCode:   errors.CodeInvalidInput,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := v.ValidateClusterName(tt.input)
			
			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
					return
				}
				if customErr, ok := err.(*errors.Error); ok {
					if customErr.Code != tt.errorCode {
						t.Errorf("Expected error code %v, got %v", tt.errorCode, customErr.Code)
					}
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}
			}
		})
	}
}

func TestValidator_ValidateKubernetesVersion(t *testing.T) {
	v := NewValidator()

	tests := []struct {
		name        string
		input       string
		expectError bool
	}{
		{
			name:        "valid version",
			input:       "v1.28.0",
			expectError: false,
		},
		{
			name:        "valid with patch",
			input:       "v1.27.5",
			expectError: false,
		},
		{
			name:        "valid with pre-release",
			input:       "v1.29.0-alpha.1",
			expectError: false,
		},
		{
			name:        "valid with build metadata",
			input:       "v1.28.0-rc.1.el8",
			expectError: false,
		},
		{
			name:        "empty version",
			input:       "",
			expectError: true,
		},
		{
			name:        "missing v prefix",
			input:       "1.28.0",
			expectError: true,
		},
		{
			name:        "invalid format",
			input:       "v1.28",
			expectError: true,
		},
		{
			name:        "non-numeric major",
			input:       "va.28.0",
			expectError: true,
		},
		{
			name:        "too many dots",
			input:       "v1.28.0.1",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := v.ValidateKubernetesVersion(tt.input)
			
			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}
			}
		})
	}
}

func TestValidator_ValidateReplicaCount(t *testing.T) {
	v := NewValidator()

	tests := []struct {
		name        string
		input       int32
		expectError bool
	}{
		{
			name:        "valid count",
			input:       3,
			expectError: false,
		},
		{
			name:        "zero replicas",
			input:       0,
			expectError: false,
		},
		{
			name:        "maximum replicas",
			input:       100,
			expectError: false,
		},
		{
			name:        "negative replicas",
			input:       -1,
			expectError: true,
		},
		{
			name:        "too many replicas",
			input:       101,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := v.ValidateReplicaCount(tt.input)
			
			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}
			}
		})
	}
}

func TestValidator_ValidateClusterVariables(t *testing.T) {
	v := NewValidator()

	tests := []struct {
		name        string
		input       map[string]interface{}
		expectError bool
	}{
		{
			name: "valid variables",
			input: map[string]interface{}{
				"nodeCount":    3,
				"region":       "us-west-2",
				"instanceType": "t3.medium",
			},
			expectError: false,
		},
		{
			name: "valid with float nodeCount",
			input: map[string]interface{}{
				"nodeCount": 3.0,
				"region":    "us-east-1",
			},
			expectError: false,
		},
		{
			name: "invalid nodeCount type",
			input: map[string]interface{}{
				"nodeCount": "three",
			},
			expectError: true,
		},
		{
			name: "negative nodeCount",
			input: map[string]interface{}{
				"nodeCount": -1,
			},
			expectError: true,
		},
		{
			name: "empty region",
			input: map[string]interface{}{
				"region": "",
			},
			expectError: true,
		},
		{
			name: "non-string region",
			input: map[string]interface{}{
				"region": 123,
			},
			expectError: true,
		},
		{
			name: "empty instanceType",
			input: map[string]interface{}{
				"instanceType": "",
			},
			expectError: true,
		},
		{
			name: "non-string instanceType",
			input: map[string]interface{}{
				"instanceType": 123,
			},
			expectError: true,
		},
		{
			name:        "empty variables",
			input:       map[string]interface{}{},
			expectError: false,
		},
		{
			name:        "nil variables",
			input:       nil,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := v.ValidateClusterVariables(tt.input)
			
			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}
			}
		})
	}
}

func TestValidator_ValidateIPAddress(t *testing.T) {
	v := NewValidator()

	tests := []struct {
		name        string
		input       string
		expectError bool
	}{
		{
			name:        "valid IPv4",
			input:       "192.168.1.1",
			expectError: false,
		},
		{
			name:        "valid IPv6",
			input:       "2001:db8::1",
			expectError: false,
		},
		{
			name:        "empty IP",
			input:       "",
			expectError: true,
		},
		{
			name:        "invalid IPv4",
			input:       "256.256.256.256",
			expectError: true,
		},
		{
			name:        "invalid format",
			input:       "not-an-ip",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := v.ValidateIPAddress(tt.input)
			
			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}
			}
		})
	}
}

func TestValidator_ValidatePort(t *testing.T) {
	v := NewValidator()

	tests := []struct {
		name        string
		input       int
		expectError bool
	}{
		{
			name:        "valid port",
			input:       8080,
			expectError: false,
		},
		{
			name:        "minimum port",
			input:       1,
			expectError: false,
		},
		{
			name:        "maximum port",
			input:       65535,
			expectError: false,
		},
		{
			name:        "zero port",
			input:       0,
			expectError: true,
		},
		{
			name:        "negative port",
			input:       -1,
			expectError: true,
		},
		{
			name:        "port too high",
			input:       65536,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := v.ValidatePort(tt.input)
			
			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}
			}
		})
	}
}

func TestSanitizeClusterName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "valid name unchanged",
			input:    "my-cluster",
			expected: "my-cluster",
		},
		{
			name:     "uppercase converted",
			input:    "My-Cluster",
			expected: "my-cluster",
		},
		{
			name:     "underscores to hyphens",
			input:    "my_cluster_name",
			expected: "my-cluster-name",
		},
		{
			name:     "special characters removed",
			input:    "my@cluster#name!",
			expected: "my-cluster-name",
		},
		{
			name:     "starts with number",
			input:    "123-cluster",
			expected: "cluster-123-cluster",
		},
		{
			name:     "multiple hyphens preserved",
			input:    "my---cluster",
			expected: "my---cluster", // Current implementation doesn't collapse multiple hyphens
		},
		{
			name:     "trailing hyphens removed",
			input:    "my-cluster--",
			expected: "my-cluster",
		},
		{
			name:     "leading hyphens removed",
			input:    "--my-cluster",
			expected: "my-cluster",
		},
		{
			name:     "too long truncated",
			input:    "a-very-long-cluster-name-that-exceeds-the-maximum-allowed-length",
			expected: "a-very-long-cluster-name-that-exceeds-the-maximum-allowed-lengt", // 63 chars
		},
		{
			name:     "empty input",
			input:    "",
			expected: "cluster",
		},
		{
			name:     "only special characters",
			input:    "@#$%",
			expected: "cluster",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SanitizeClusterName(tt.input); got != tt.expected {
				t.Errorf("SanitizeClusterName() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestToInt32(t *testing.T) {
	tests := []struct {
		name        string
		input       interface{}
		expected    int32
		expectValid bool
	}{
		{
			name:        "int",
			input:       42,
			expected:    42,
			expectValid: true,
		},
		{
			name:        "int32",
			input:       int32(42),
			expected:    42,
			expectValid: true,
		},
		{
			name:        "int64",
			input:       int64(42),
			expected:    42,
			expectValid: true,
		},
		{
			name:        "float32",
			input:       float32(42.0),
			expected:    42,
			expectValid: true,
		},
		{
			name:        "float64",
			input:       float64(42.0),
			expected:    42,
			expectValid: true,
		},
		{
			name:        "string",
			input:       "42",
			expected:    0,
			expectValid: false,
		},
		{
			name:        "nil",
			input:       nil,
			expected:    0,
			expectValid: false,
		},
		{
			name:        "bool",
			input:       true,
			expected:    0,
			expectValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, valid := toInt32(tt.input)
			
			if valid != tt.expectValid {
				t.Errorf("toInt32() valid = %v, want %v", valid, tt.expectValid)
			}
			
			if tt.expectValid && got != tt.expected {
				t.Errorf("toInt32() value = %v, want %v", got, tt.expected)
			}
		})
	}
}