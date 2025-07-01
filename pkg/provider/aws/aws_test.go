package aws

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
)

func TestNewAWSProvider(t *testing.T) {
	t.Run("with region", func(t *testing.T) {
		provider := NewAWSProvider("us-east-1")
		assert.NotNil(t, provider)
		assert.Equal(t, "us-east-1", provider.region)
		assert.Equal(t, "aws", provider.Name())
	})

	t.Run("without region (default)", func(t *testing.T) {
		provider := NewAWSProvider("")
		assert.NotNil(t, provider)
		assert.Equal(t, "us-west-2", provider.region)
		assert.Equal(t, "aws", provider.Name())
	})
}

func TestAWSProvider_ValidateClusterConfig(t *testing.T) {
	provider := NewAWSProvider("us-west-2")
	ctx := context.Background()

	t.Run("valid configuration", func(t *testing.T) {
		variables := map[string]interface{}{
			"region":       "us-west-2",
			"instanceType": "m5.large",
			"nodeCount":    3,
		}
		
		err := provider.ValidateClusterConfig(ctx, variables)
		assert.NoError(t, err)
	})

	t.Run("valid configuration with float nodeCount", func(t *testing.T) {
		variables := map[string]interface{}{
			"region":       "us-west-2",
			"instanceType": "m5.large",
			"nodeCount":    float64(3),
		}
		
		err := provider.ValidateClusterConfig(ctx, variables)
		assert.NoError(t, err)
	})

	t.Run("invalid region", func(t *testing.T) {
		variables := map[string]interface{}{
			"region": "invalid-region",
		}
		
		err := provider.ValidateClusterConfig(ctx, variables)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid AWS region")
	})

	t.Run("non-string region", func(t *testing.T) {
		variables := map[string]interface{}{
			"region": 123,
		}
		
		err := provider.ValidateClusterConfig(ctx, variables)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "region must be a string")
	})

	t.Run("invalid instance type", func(t *testing.T) {
		variables := map[string]interface{}{
			"instanceType": "invalid-type",
		}
		
		err := provider.ValidateClusterConfig(ctx, variables)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid AWS instance type")
	})

	t.Run("non-string instance type", func(t *testing.T) {
		variables := map[string]interface{}{
			"instanceType": 123,
		}
		
		err := provider.ValidateClusterConfig(ctx, variables)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "instanceType must be a string")
	})

	t.Run("invalid node count - too low", func(t *testing.T) {
		variables := map[string]interface{}{
			"nodeCount": 0,
		}
		
		err := provider.ValidateClusterConfig(ctx, variables)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "nodeCount must be between 1 and 100")
	})

	t.Run("invalid node count - too high", func(t *testing.T) {
		variables := map[string]interface{}{
			"nodeCount": 101,
		}
		
		err := provider.ValidateClusterConfig(ctx, variables)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "nodeCount must be between 1 and 100")
	})

	t.Run("invalid node count - float with decimals", func(t *testing.T) {
		variables := map[string]interface{}{
			"nodeCount": 3.5,
		}
		
		err := provider.ValidateClusterConfig(ctx, variables)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "nodeCount must be an integer")
	})

	t.Run("invalid node count - non-numeric", func(t *testing.T) {
		variables := map[string]interface{}{
			"nodeCount": "three",
		}
		
		err := provider.ValidateClusterConfig(ctx, variables)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "nodeCount must be an integer")
	})
}

func TestAWSProvider_GetSupportedKubernetesVersions(t *testing.T) {
	provider := NewAWSProvider("us-west-2")
	ctx := context.Background()

	versions, err := provider.GetSupportedKubernetesVersions(ctx)
	require.NoError(t, err)
	assert.NotEmpty(t, versions)
	
	// Check that versions are in expected format
	for _, version := range versions {
		assert.True(t, len(version) > 0)
		assert.True(t, version[0] == 'v') // Should start with 'v'
	}
	
	// Check for some expected versions
	assert.Contains(t, versions, "v1.31.0")
	assert.Contains(t, versions, "v1.30.5")
}

func TestAWSProvider_GetDefaultMachineTemplate(t *testing.T) {
	provider := NewAWSProvider("us-west-2")
	ctx := context.Background()

	template, err := provider.GetDefaultMachineTemplate(ctx)
	// This is currently not implemented, so should return an error
	assert.Error(t, err)
	assert.Nil(t, template)
	assert.Contains(t, err.Error(), "not yet implemented")
}

func TestAWSProvider_GetInfrastructureTemplate(t *testing.T) {
	provider := NewAWSProvider("us-west-2")
	ctx := context.Background()

	variables := map[string]interface{}{
		"region": "us-west-2",
	}

	template, err := provider.GetInfrastructureTemplate(ctx, variables)
	// This is currently not implemented, so should return an error
	assert.Error(t, err)
	assert.Nil(t, template)
	assert.Contains(t, err.Error(), "not yet implemented")
}

func TestAWSProvider_ValidateInfrastructureReadiness(t *testing.T) {
	provider := NewAWSProvider("us-west-2")
	ctx := context.Background()

	t.Run("cluster without infrastructure reference", func(t *testing.T) {
		cluster := &clusterv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-cluster",
			},
		}

		err := provider.ValidateInfrastructureReadiness(ctx, cluster)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "has no infrastructure reference")
	})

	t.Run("cluster with non-AWS infrastructure", func(t *testing.T) {
		cluster := &clusterv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-cluster",
			},
			Spec: clusterv1.ClusterSpec{
				InfrastructureRef: &corev1.ObjectReference{
					Kind: "AzureCluster",
					Name: "test-azure-cluster",
				},
			},
		}

		err := provider.ValidateInfrastructureReadiness(ctx, cluster)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "infrastructure is not an AWSCluster")
	})

	t.Run("AWS cluster not ready", func(t *testing.T) {
		cluster := &clusterv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-cluster",
			},
			Spec: clusterv1.ClusterSpec{
				InfrastructureRef: &corev1.ObjectReference{
					Kind: "AWSCluster",
					Name: "test-aws-cluster",
				},
			},
			Status: clusterv1.ClusterStatus{
				InfrastructureReady: false,
			},
		}

		err := provider.ValidateInfrastructureReadiness(ctx, cluster)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "infrastructure for cluster test-cluster is not ready")
	})

	t.Run("AWS cluster ready", func(t *testing.T) {
		cluster := &clusterv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-cluster",
			},
			Spec: clusterv1.ClusterSpec{
				InfrastructureRef: &corev1.ObjectReference{
					Kind: "AWSCluster",
					Name: "test-aws-cluster",
				},
			},
			Status: clusterv1.ClusterStatus{
				InfrastructureReady: true,
			},
		}

		err := provider.ValidateInfrastructureReadiness(ctx, cluster)
		assert.NoError(t, err)
	})
}

func TestAWSProvider_GetProviderSpecificStatus(t *testing.T) {
	provider := NewAWSProvider("us-west-2")
	ctx := context.Background()

	t.Run("cluster with infrastructure reference and region variable", func(t *testing.T) {
		// Create region variable
		regionValue := &apiextensionsv1.JSON{}
		regionValue.Raw = []byte(`"us-east-1"`)

		cluster := &clusterv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-cluster",
			},
			Spec: clusterv1.ClusterSpec{
				InfrastructureRef: &corev1.ObjectReference{
					Kind: "AWSCluster",
					Name: "test-aws-cluster",
				},
				Topology: &clusterv1.Topology{
					Variables: []clusterv1.ClusterVariable{
						{
							Name:  "region",
							Value: *regionValue,
						},
					},
				},
			},
			Status: clusterv1.ClusterStatus{
				InfrastructureReady: true,
			},
		}

		status, err := provider.GetProviderSpecificStatus(ctx, cluster)
		require.NoError(t, err)
		
		assert.Equal(t, "AWSCluster", status["infrastructureKind"])
		assert.Equal(t, "test-aws-cluster", status["infrastructureName"])
		assert.Equal(t, "us-east-1", status["region"])
		assert.Equal(t, "aws", status["provider"])
		assert.Equal(t, true, status["ready"])
	})

	t.Run("cluster without region variable", func(t *testing.T) {
		cluster := &clusterv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-cluster",
			},
			Spec: clusterv1.ClusterSpec{
				InfrastructureRef: &corev1.ObjectReference{
					Kind: "AWSCluster",
					Name: "test-aws-cluster",
				},
			},
			Status: clusterv1.ClusterStatus{
				InfrastructureReady: false,
			},
		}

		status, err := provider.GetProviderSpecificStatus(ctx, cluster)
		require.NoError(t, err)
		
		assert.Equal(t, "AWSCluster", status["infrastructureKind"])
		assert.Equal(t, "test-aws-cluster", status["infrastructureName"])
		assert.Equal(t, "us-west-2", status["region"]) // Should use provider default
		assert.Equal(t, "aws", status["provider"])
		assert.Equal(t, false, status["ready"])
	})
}

func TestAWSProvider_GetRegions(t *testing.T) {
	provider := NewAWSProvider("us-west-2")
	ctx := context.Background()

	regions, err := provider.GetRegions(ctx)
	require.NoError(t, err)
	assert.NotEmpty(t, regions)

	// Check for some expected regions
	assert.Contains(t, regions, "us-east-1")
	assert.Contains(t, regions, "us-west-2")
	assert.Contains(t, regions, "eu-west-1")
	assert.Contains(t, regions, "ap-southeast-1")
}

func TestAWSProvider_GetInstanceTypes(t *testing.T) {
	provider := NewAWSProvider("us-west-2")
	ctx := context.Background()

	t.Run("valid region", func(t *testing.T) {
		types, err := provider.GetInstanceTypes(ctx, "us-west-2")
		require.NoError(t, err)
		assert.NotEmpty(t, types)

		// Check for some expected instance types
		assert.Contains(t, types, "t3.micro")
		assert.Contains(t, types, "m5.large")
		assert.Contains(t, types, "c5.xlarge")
	})

	t.Run("invalid region", func(t *testing.T) {
		types, err := provider.GetInstanceTypes(ctx, "invalid-region")
		assert.Error(t, err)
		assert.Nil(t, types)
		assert.Contains(t, err.Error(), "invalid AWS region")
	})
}

func TestAWSProvider_isValidAWSRegion(t *testing.T) {
	provider := NewAWSProvider("us-west-2")

	validRegions := []string{
		"us-east-1",
		"us-west-2",
		"eu-central-1",
		"ap-southeast-1",
		"ca-central-1",
		"sa-east-1",
	}

	invalidRegions := []string{
		"invalid",
		"us-east",
		"eu-west",
		"ap",
		"",
		"us-east-1-invalid",
		"xx-west-1",
	}

	for _, region := range validRegions {
		assert.True(t, provider.isValidAWSRegion(region), "Expected %s to be valid", region)
	}

	for _, region := range invalidRegions {
		assert.False(t, provider.isValidAWSRegion(region), "Expected %s to be invalid", region)
	}
}

func TestAWSProvider_isValidInstanceType(t *testing.T) {
	provider := NewAWSProvider("us-west-2")

	validTypes := []string{
		"t3.micro",
		"t3.small",
		"m5.large",
		"m5.xlarge",
		"c5.2xlarge",
		"r5.4xlarge",
		"c6i.18xlarge",
	}

	invalidTypes := []string{
		"invalid",
		"t3",
		"micro",
		"t3.invalid-size",
		"",
		"t3.large.extra",
		"invalid.large",
	}

	for _, instanceType := range validTypes {
		assert.True(t, provider.isValidInstanceType(instanceType), "Expected %s to be valid", instanceType)
	}

	for _, instanceType := range invalidTypes {
		assert.False(t, provider.isValidInstanceType(instanceType), "Expected %s to be invalid", instanceType)
	}
}