package provider

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
)

// mockProvider implements the Provider interface for testing
type mockProvider struct {
	name string
}

func (m *mockProvider) Name() string {
	return m.name
}

func (m *mockProvider) ValidateClusterConfig(ctx context.Context, variables map[string]interface{}) error {
	return nil
}

func (m *mockProvider) GetSupportedKubernetesVersions(ctx context.Context) ([]string, error) {
	return []string{"v1.31.0", "v1.30.5"}, nil
}

func (m *mockProvider) GetDefaultMachineTemplate(ctx context.Context) (runtime.Object, error) {
	return nil, nil
}

func (m *mockProvider) GetInfrastructureTemplate(ctx context.Context, variables map[string]interface{}) (runtime.Object, error) {
	return nil, nil
}

func (m *mockProvider) ValidateInfrastructureReadiness(ctx context.Context, cluster *clusterv1.Cluster) error {
	return nil
}

func (m *mockProvider) GetProviderSpecificStatus(ctx context.Context, cluster *clusterv1.Cluster) (map[string]interface{}, error) {
	return map[string]interface{}{"provider": m.name}, nil
}

func (m *mockProvider) GetRegions(ctx context.Context) ([]string, error) {
	return []string{"region-1", "region-2"}, nil
}

func (m *mockProvider) GetInstanceTypes(ctx context.Context, region string) ([]string, error) {
	return []string{"type-1", "type-2"}, nil
}

func TestNewProviderManager(t *testing.T) {
	pm := NewProviderManager()
	assert.NotNil(t, pm)
	assert.NotNil(t, pm.providers)
	assert.Empty(t, pm.providers)
}

func TestProviderManager_RegisterProvider(t *testing.T) {
	pm := NewProviderManager()
	provider := &mockProvider{name: "test-provider"}

	pm.RegisterProvider(provider)

	assert.Len(t, pm.providers, 1)
	assert.Contains(t, pm.providers, "test-provider")
	assert.Equal(t, provider, pm.providers["test-provider"])
}

func TestProviderManager_GetProvider(t *testing.T) {
	pm := NewProviderManager()
	provider := &mockProvider{name: "test-provider"}
	pm.RegisterProvider(provider)

	t.Run("existing provider", func(t *testing.T) {
		result, exists := pm.GetProvider("test-provider")
		assert.True(t, exists)
		assert.Equal(t, provider, result)
	})

	t.Run("non-existent provider", func(t *testing.T) {
		result, exists := pm.GetProvider("non-existent")
		assert.False(t, exists)
		assert.Nil(t, result)
	})
}

func TestProviderManager_ListProviders(t *testing.T) {
	pm := NewProviderManager()

	t.Run("empty manager", func(t *testing.T) {
		providers := pm.ListProviders()
		assert.Empty(t, providers)
	})

	t.Run("with providers", func(t *testing.T) {
		provider1 := &mockProvider{name: "provider-1"}
		provider2 := &mockProvider{name: "provider-2"}
		
		pm.RegisterProvider(provider1)
		pm.RegisterProvider(provider2)

		providers := pm.ListProviders()
		assert.Len(t, providers, 2)
		assert.Contains(t, providers, "provider-1")
		assert.Contains(t, providers, "provider-2")
	})
}

func TestProviderManager_MultipleProviders(t *testing.T) {
	pm := NewProviderManager()
	
	// Register multiple providers
	awsProvider := &mockProvider{name: "aws"}
	azureProvider := &mockProvider{name: "azure"}
	gcpProvider := &mockProvider{name: "gcp"}
	
	pm.RegisterProvider(awsProvider)
	pm.RegisterProvider(azureProvider)
	pm.RegisterProvider(gcpProvider)
	
	// Verify all providers are registered
	providers := pm.ListProviders()
	assert.Len(t, providers, 3)
	
	// Verify each provider can be retrieved
	aws, exists := pm.GetProvider("aws")
	require.True(t, exists)
	assert.Equal(t, "aws", aws.Name())
	
	azure, exists := pm.GetProvider("azure")
	require.True(t, exists)
	assert.Equal(t, "azure", azure.Name())
	
	gcp, exists := pm.GetProvider("gcp")
	require.True(t, exists)
	assert.Equal(t, "gcp", gcp.Name())
}

func TestProviderManager_OverwriteProvider(t *testing.T) {
	pm := NewProviderManager()
	
	// Register initial provider
	provider1 := &mockProvider{name: "test"}
	pm.RegisterProvider(provider1)
	
	// Register provider with same name (should overwrite)
	provider2 := &mockProvider{name: "test"}
	pm.RegisterProvider(provider2)
	
	// Verify only one provider with the name exists and it's the second one
	providers := pm.ListProviders()
	assert.Len(t, providers, 1)
	
	result, exists := pm.GetProvider("test")
	require.True(t, exists)
	assert.Equal(t, provider2, result) // Should be the second provider
}