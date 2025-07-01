package v1

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClusterTemplate(t *testing.T) {
	template := ClusterTemplate{
		Name:        "aws-cluster-template",
		Namespace:   "default",
		Description: "AWS cluster template for production workloads",
		Provider:    "aws",
		KubernetesVersions: []string{
			"v1.31.0",
			"v1.30.5",
			"v1.29.9",
		},
		Variables: []TemplateVariable{
			{
				Name:        "region",
				Required:    true,
				Type:        "string",
				Description: "AWS region for the cluster",
				Example:     "us-west-2",
			},
			{
				Name:        "instanceType",
				Required:    false,
				Type:        "string",
				Default:     "m5.large",
				Description: "EC2 instance type for worker nodes",
				Example:     "m5.xlarge",
			},
			{
				Name:        "nodeCount",
				Required:    false,
				Type:        "integer",
				Default:     3,
				Description: "Number of worker nodes",
				Example:     5,
			},
		},
		Labels: map[string]string{
			"cluster.x-k8s.io/provider": "aws",
			"environment":               "production",
		},
		Annotations: map[string]string{
			"cluster.x-k8s.io/description": "Production AWS cluster template",
		},
	}

	// Test JSON serialization
	data, err := json.Marshal(template)
	require.NoError(t, err)

	var unmarshaled ClusterTemplate
	err = json.Unmarshal(data, &unmarshaled)
	require.NoError(t, err)

	assert.Equal(t, template.Name, unmarshaled.Name)
	assert.Equal(t, template.Description, unmarshaled.Description)
	assert.Equal(t, template.Provider, unmarshaled.Provider)
	assert.Equal(t, template.KubernetesVersions, unmarshaled.KubernetesVersions)
	assert.Len(t, unmarshaled.Variables, 3)
	assert.Equal(t, template.Labels, unmarshaled.Labels)
	assert.Equal(t, template.Annotations, unmarshaled.Annotations)
}

func TestTemplateVariable(t *testing.T) {
	tests := []struct {
		name     string
		variable TemplateVariable
	}{
		{
			name: "required string variable",
			variable: TemplateVariable{
				Name:        "region",
				Required:    true,
				Type:        "string",
				Description: "AWS region",
				Example:     "us-west-2",
			},
		},
		{
			name: "optional string variable with default",
			variable: TemplateVariable{
				Name:        "instanceType",
				Required:    false,
				Type:        "string",
				Default:     "m5.large",
				Description: "Instance type",
				Example:     "m5.xlarge",
			},
		},
		{
			name: "integer variable",
			variable: TemplateVariable{
				Name:        "nodeCount",
				Required:    true,
				Type:        "integer",
				Default:     3,
				Description: "Number of nodes",
				Example:     5,
			},
		},
		{
			name: "boolean variable",
			variable: TemplateVariable{
				Name:        "enableMonitoring",
				Required:    false,
				Type:        "boolean",
				Default:     true,
				Description: "Enable cluster monitoring",
				Example:     false,
			},
		},
		{
			name: "complex object variable",
			variable: TemplateVariable{
				Name:     "networking",
				Required: false,
				Type:     "object",
				Default: map[string]interface{}{
					"cidr": "10.0.0.0/16",
					"subnets": []string{
						"10.0.1.0/24",
						"10.0.2.0/24",
					},
				},
				Description: "Network configuration",
				Example: map[string]interface{}{
					"cidr": "192.168.0.0/16",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test JSON serialization
			data, err := json.Marshal(tt.variable)
			require.NoError(t, err)

			var unmarshaled TemplateVariable
			err = json.Unmarshal(data, &unmarshaled)
			require.NoError(t, err)

			assert.Equal(t, tt.variable.Name, unmarshaled.Name)
			assert.Equal(t, tt.variable.Required, unmarshaled.Required)
			assert.Equal(t, tt.variable.Type, unmarshaled.Type)
			assert.Equal(t, tt.variable.Description, unmarshaled.Description)

			// Compare default and example values (JSON unmarshaling converts numbers to float64)
			if tt.variable.Default != nil {
				switch tt.variable.Type {
				case "integer":
					// JSON numbers become float64
					assert.Equal(t, float64(tt.variable.Default.(int)), unmarshaled.Default)
				case "object":
					// Complex objects need deep comparison with JSON type conversions
					defaultData, _ := json.Marshal(tt.variable.Default)
					unmarshaledData, _ := json.Marshal(unmarshaled.Default)
					assert.JSONEq(t, string(defaultData), string(unmarshaledData))
				default:
					assert.Equal(t, tt.variable.Default, unmarshaled.Default)
				}
			}
			if tt.variable.Example != nil {
				switch tt.variable.Type {
				case "integer":
					assert.Equal(t, float64(tt.variable.Example.(int)), unmarshaled.Example)
				default:
					assert.Equal(t, tt.variable.Example, unmarshaled.Example)
				}
			}
		})
	}
}

func TestClusterTemplateEdgeCases(t *testing.T) {
	t.Run("minimal template", func(t *testing.T) {
		template := ClusterTemplate{
			Name:      "minimal-template",
			Namespace: "default",
			Provider:  "aws",
		}

		data, err := json.Marshal(template)
		require.NoError(t, err)

		var unmarshaled ClusterTemplate
		err = json.Unmarshal(data, &unmarshaled)
		require.NoError(t, err)

		assert.Equal(t, template.Name, unmarshaled.Name)
		assert.Equal(t, template.Provider, unmarshaled.Provider)
		assert.Nil(t, unmarshaled.KubernetesVersions)
		assert.Nil(t, unmarshaled.Variables)
		assert.Nil(t, unmarshaled.Labels)
		assert.Nil(t, unmarshaled.Annotations)
	})

	t.Run("template with empty collections", func(t *testing.T) {
		template := ClusterTemplate{
			Name:               "empty-collections-template",
			Namespace:          "default",
			Provider:           "aws",
			KubernetesVersions: []string{},
			Variables:          []TemplateVariable{},
			Labels:             map[string]string{},
			Annotations:        map[string]string{},
		}

		data, err := json.Marshal(template)
		require.NoError(t, err)

		var unmarshaled ClusterTemplate
		err = json.Unmarshal(data, &unmarshaled)
		require.NoError(t, err)

		assert.Equal(t, template.Name, unmarshaled.Name)
		assert.Empty(t, unmarshaled.KubernetesVersions)
		assert.Empty(t, unmarshaled.Variables)
		assert.Empty(t, unmarshaled.Labels)
		assert.Empty(t, unmarshaled.Annotations)
	})

	t.Run("template with nil default and example", func(t *testing.T) {
		variable := TemplateVariable{
			Name:        "optionalVar",
			Required:    false,
			Type:        "string",
			Description: "Optional variable with no default",
		}

		data, err := json.Marshal(variable)
		require.NoError(t, err)

		var unmarshaled TemplateVariable
		err = json.Unmarshal(data, &unmarshaled)
		require.NoError(t, err)

		assert.Equal(t, variable.Name, unmarshaled.Name)
		assert.False(t, unmarshaled.Required)
		assert.Nil(t, unmarshaled.Default)
		assert.Nil(t, unmarshaled.Example)
	})
}

func TestTemplateVariableTypes(t *testing.T) {
	t.Run("string array default", func(t *testing.T) {
		variable := TemplateVariable{
			Name:    "subnets",
			Type:    "array",
			Default: []string{"subnet-1", "subnet-2"},
			Example: []string{"subnet-a", "subnet-b", "subnet-c"},
		}

		data, err := json.Marshal(variable)
		require.NoError(t, err)

		var unmarshaled TemplateVariable
		err = json.Unmarshal(data, &unmarshaled)
		require.NoError(t, err)

		// JSON unmarshaling converts []string to []interface{}
		defaultArray, ok := unmarshaled.Default.([]interface{})
		require.True(t, ok)
		assert.Len(t, defaultArray, 2)
		assert.Equal(t, "subnet-1", defaultArray[0])
		assert.Equal(t, "subnet-2", defaultArray[1])
	})

	t.Run("nested object default", func(t *testing.T) {
		variable := TemplateVariable{
			Name: "config",
			Type: "object",
			Default: map[string]interface{}{
				"api": map[string]interface{}{
					"version": "v1",
					"enabled": true,
				},
				"replicas": 3,
			},
		}

		data, err := json.Marshal(variable)
		require.NoError(t, err)

		var unmarshaled TemplateVariable
		err = json.Unmarshal(data, &unmarshaled)
		require.NoError(t, err)

		defaultObj, ok := unmarshaled.Default.(map[string]interface{})
		require.True(t, ok)

		apiObj, ok := defaultObj["api"].(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "v1", apiObj["version"])
		assert.Equal(t, true, apiObj["enabled"])
		assert.Equal(t, float64(3), defaultObj["replicas"]) // JSON numbers become float64
	})
}
