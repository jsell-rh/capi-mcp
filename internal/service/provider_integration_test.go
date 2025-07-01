package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/capi-mcp/capi-mcp-server/pkg/provider"
	"github.com/capi-mcp/capi-mcp-server/pkg/provider/aws"
)

func TestProviderIntegration(t *testing.T) {
	// Create provider manager with AWS provider
	providerManager := provider.NewProviderManager()
	awsProvider := aws.NewAWSProvider("us-west-2")
	providerManager.RegisterProvider(awsProvider)

	// Create service with provider manager
	service := setupTestServiceWithProviders(providerManager)

	t.Run("extractProviderName", func(t *testing.T) {
		tests := []struct {
			name         string
			variables    map[string]interface{}
			templateName string
			expected     string
		}{
			{
				name:         "explicit provider in variables",
				variables:    map[string]interface{}{"provider": "aws"},
				templateName: "test-template",
				expected:     "aws",
			},
			{
				name:         "aws in template name",
				variables:    map[string]interface{}{},
				templateName: "aws-cluster-template",
				expected:     "aws",
			},
			{
				name:         "azure in template name",
				variables:    map[string]interface{}{},
				templateName: "azure-cluster-class",
				expected:     "azure",
			},
			{
				name:         "gcp in template name",
				variables:    map[string]interface{}{},
				templateName: "google-cluster-template",
				expected:     "gcp",
			},
			{
				name:         "default to aws",
				variables:    map[string]interface{}{},
				templateName: "unknown-template",
				expected:     "aws",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				result := service.extractProviderName(tt.variables, tt.templateName)
				assert.Equal(t, tt.expected, result)
			})
		}
	})

	t.Run("provider validation directly", func(t *testing.T) {
		ctx := context.Background()

		t.Run("valid AWS configuration", func(t *testing.T) {
			variables := map[string]interface{}{
				"region":       "us-west-2",
				"instanceType": "m5.large",
				"nodeCount":    3,
			}

			prov, exists := providerManager.GetProvider("aws")
			require.True(t, exists)
			
			err := prov.ValidateClusterConfig(ctx, variables)
			assert.NoError(t, err)
		})

		t.Run("invalid AWS region", func(t *testing.T) {
			variables := map[string]interface{}{
				"region": "invalid-region",
			}

			prov, exists := providerManager.GetProvider("aws")
			require.True(t, exists)
			
			err := prov.ValidateClusterConfig(ctx, variables)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "invalid AWS region")
		})

		t.Run("invalid instance type", func(t *testing.T) {
			variables := map[string]interface{}{
				"region":       "us-west-2",
				"instanceType": "invalid-type",
			}

			prov, exists := providerManager.GetProvider("aws")
			require.True(t, exists)
			
			err := prov.ValidateClusterConfig(ctx, variables)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "invalid AWS instance type")
		})

		t.Run("invalid node count", func(t *testing.T) {
			variables := map[string]interface{}{
				"region":    "us-west-2",
				"nodeCount": -1,
			}

			prov, exists := providerManager.GetProvider("aws")
			require.True(t, exists)
			
			err := prov.ValidateClusterConfig(ctx, variables)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "nodeCount must be between 1 and 100")
		})
	})
}

func setupTestServiceWithProviders(providerManager *provider.ProviderManager) *ClusterService {
	// Use the same setup as the main test but with custom provider manager
	service := setupTestService()
	service.providerManager = providerManager
	return service
}