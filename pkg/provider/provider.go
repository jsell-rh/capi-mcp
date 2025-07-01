package provider

import (
	"context"

	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
)

// Provider defines the interface for infrastructure-specific operations.
// This interface allows the CAPI MCP Server to support multiple cloud providers
// by implementing provider-specific logic while maintaining a consistent API.
//
// Each provider implementation should handle the specifics of their cloud
// platform, such as:
// - Infrastructure resource templates (e.g., AWSCluster, AzureCluster)
// - Provider-specific validation rules
// - Custom cluster configuration parameters
// - Provider-specific status interpretation
type Provider interface {
	// Name returns the unique name of this provider (e.g., "aws", "azure", "gcp").
	Name() string

	// ValidateClusterConfig validates provider-specific cluster configuration
	// before creating a cluster. This allows each provider to enforce their own
	// validation rules for cluster parameters.
	ValidateClusterConfig(ctx context.Context, variables map[string]interface{}) error

	// GetSupportedKubernetesVersions returns a list of Kubernetes versions
	// that this provider supports for new clusters.
	GetSupportedKubernetesVersions(ctx context.Context) ([]string, error)

	// GetDefaultMachineTemplate returns the default machine template specification
	// for this provider. This can be used when creating clusters without
	// explicit machine configuration.
	GetDefaultMachineTemplate(ctx context.Context) (runtime.Object, error)

	// GetInfrastructureTemplate returns the infrastructure template specification
	// for the given cluster variables. This template will be used to create
	// the provider-specific infrastructure resources.
	GetInfrastructureTemplate(ctx context.Context, variables map[string]interface{}) (runtime.Object, error)

	// ValidateInfrastructureReadiness checks if the infrastructure for a cluster
	// is ready and functional. This allows provider-specific health checks.
	ValidateInfrastructureReadiness(ctx context.Context, cluster *clusterv1.Cluster) error

	// GetProviderSpecificStatus extracts and formats provider-specific status
	// information from the cluster and its infrastructure resources.
	GetProviderSpecificStatus(ctx context.Context, cluster *clusterv1.Cluster) (map[string]interface{}, error)

	// GetRegions returns a list of available regions/zones for this provider.
	// This can be used for validation and to provide region options to users.
	GetRegions(ctx context.Context) ([]string, error)

	// GetInstanceTypes returns a list of available instance/machine types
	// for the given region. This helps with validation and user guidance.
	GetInstanceTypes(ctx context.Context, region string) ([]string, error)
}

// ProviderManager manages multiple provider implementations and provides
// a unified interface for accessing provider-specific functionality.
type ProviderManager struct {
	providers map[string]Provider
}

// NewProviderManager creates a new provider manager instance.
func NewProviderManager() *ProviderManager {
	return &ProviderManager{
		providers: make(map[string]Provider),
	}
}

// RegisterProvider adds a provider implementation to the manager.
func (pm *ProviderManager) RegisterProvider(provider Provider) {
	pm.providers[provider.Name()] = provider
}

// GetProvider retrieves a provider by name.
func (pm *ProviderManager) GetProvider(name string) (Provider, bool) {
	provider, exists := pm.providers[name]
	return provider, exists
}

// ListProviders returns a list of all registered provider names.
func (pm *ProviderManager) ListProviders() []string {
	names := make([]string, 0, len(pm.providers))
	for name := range pm.providers {
		names = append(names, name)
	}
	return names
}