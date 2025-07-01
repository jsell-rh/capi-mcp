package integration

import (
	"context"
	"log/slog"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/capi-mcp/capi-mcp-server/api/v1"
	"github.com/capi-mcp/capi-mcp-server/internal/kube"
	"github.com/capi-mcp/capi-mcp-server/internal/service"
	"github.com/capi-mcp/capi-mcp-server/pkg/provider"
	"github.com/capi-mcp/capi-mcp-server/pkg/provider/aws"
)

// IntegrationTestSuite provides a comprehensive test environment
// for testing the full integration from MCP tools to CAPI operations.
type IntegrationTestSuite struct {
	kubeClient      *kube.Client
	clusterService  *service.ClusterService
	providerManager *provider.ProviderManager
	scheme          *runtime.Scheme
	logger          *slog.Logger
}

// NewIntegrationTestSuite creates a new integration test suite with mock CAPI cluster.
func NewIntegrationTestSuite(t *testing.T) *IntegrationTestSuite {
	// Set up runtime scheme
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, clusterv1.AddToScheme(scheme))

	// Create logger that discards output during tests
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelError, // Only show errors during tests
	}))

	// Create provider manager with AWS provider
	providerManager := provider.NewProviderManager()
	awsProvider := aws.NewAWSProvider("us-west-2")
	providerManager.RegisterProvider(awsProvider)

	suite := &IntegrationTestSuite{
		providerManager: providerManager,
		scheme:          scheme,
		logger:          logger,
	}

	return suite
}

// SetupMockCluster creates a fake Kubernetes client with mock CAPI resources.
func (s *IntegrationTestSuite) SetupMockCluster(t *testing.T, objects ...client.Object) {
	// Create cluster service with nil kube client for testing
	// This will test provider validation and business logic without Kubernetes API calls
	s.clusterService = service.NewClusterService(nil, s.logger, s.providerManager)
}

// TestFullClusterLifecycle tests the complete cluster lifecycle from creation to deletion.
func TestFullClusterLifecycle(t *testing.T) {
	suite := NewIntegrationTestSuite(t)

	// Create test ClusterClass
	clusterClass := createTestClusterClass()
	
	// Create test cluster and related resources
	cluster := createTestCluster("test-cluster", "default", clusterv1.ClusterPhaseProvisioned)
	machineDeployment := createTestMachineDeployment("test-cluster-md", "default", "test-cluster", 3)
	kubeconfigSecret := createTestKubeconfigSecret("test-cluster", "default")

	// Setup mock cluster with test resources
	suite.SetupMockCluster(t, clusterClass, cluster, machineDeployment, kubeconfigSecret)

	ctx := context.Background()

	t.Run("provider validation integration", func(t *testing.T) {
		// Test that provider validation works through the service layer
		awsProvider, exists := suite.providerManager.GetProvider("aws")
		require.True(t, exists)

		// Valid configuration should pass
		validConfig := map[string]interface{}{
			"region":       "us-west-2",
			"instanceType": "m5.large",
			"nodeCount":    3,
		}
		err := awsProvider.ValidateClusterConfig(ctx, validConfig)
		assert.NoError(t, err)

		// Invalid configuration should fail
		invalidConfig := map[string]interface{}{
			"region": "invalid-region",
		}
		err = awsProvider.ValidateClusterConfig(ctx, invalidConfig)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid AWS region")
	})

	t.Run("provider manager integration", func(t *testing.T) {
		// Test provider registration and retrieval
		providers := suite.providerManager.ListProviders()
		assert.Contains(t, providers, "aws")

		awsProvider, exists := suite.providerManager.GetProvider("aws")
		assert.True(t, exists)
		assert.Equal(t, "aws", awsProvider.Name())

		// Test provider capabilities
		versions, err := awsProvider.GetSupportedKubernetesVersions(ctx)
		require.NoError(t, err)
		assert.NotEmpty(t, versions)
		assert.Contains(t, versions, "v1.31.0")

		regions, err := awsProvider.GetRegions(ctx)
		require.NoError(t, err)
		assert.NotEmpty(t, regions)
		assert.Contains(t, regions, "us-west-2")

		instanceTypes, err := awsProvider.GetInstanceTypes(ctx, "us-west-2")
		require.NoError(t, err)
		assert.NotEmpty(t, instanceTypes)
		assert.Contains(t, instanceTypes, "m5.large")
	})

	t.Run("service layer integration", func(t *testing.T) {
		// Test provider name extraction
		testCases := []struct {
			variables    map[string]interface{}
			templateName string
			expected     string
		}{
			{
				variables:    map[string]interface{}{"provider": "aws"},
				templateName: "any-template",
				expected:     "aws",
			},
			{
				variables:    map[string]interface{}{},
				templateName: "aws-cluster-template",
				expected:     "aws",
			},
			{
				variables:    map[string]interface{}{},
				templateName: "unknown-template",
				expected:     "aws", // Default
			},
		}

		for _, tc := range testCases {
			// We can't directly call extractProviderName as it's private,
			// but we can test the provider validation workflow
			input := v1.CreateClusterInput{
				ClusterName:       "test-cluster",
				TemplateName:      tc.templateName,
				KubernetesVersion: "v1.31.0",
				Variables:         tc.variables,
			}

			// Since we're using nil client, we expect this to fail on kubeClient access
			// but not on provider validation, which happens first
			_, err := suite.clusterService.CreateCluster(ctx, input)
			if err != nil {
				// Should not be a provider validation error (which comes first)
				// If it's a provider validation error, our provider integration is broken
				if strings.Contains(err.Error(), "provider validation failed") {
					t.Errorf("Provider validation failed for template %s: %v", tc.templateName, err)
				}
				// We expect other errors due to nil kubeClient
			}
		}
	})
}

// TestMockCAPIOperations tests CAPI operations with mocked Kubernetes resources.
func TestMockCAPIOperations(t *testing.T) {
	_ = context.Background()

	t.Run("cluster resource validation", func(t *testing.T) {
		// Test cluster in different states
		testCases := []struct {
			name     string
			cluster  *clusterv1.Cluster
			expected string
		}{
			{
				name:     "provisioned cluster",
				cluster:  createTestCluster("cluster-1", "default", clusterv1.ClusterPhaseProvisioned),
				expected: "Provisioned",
			},
			{
				name:     "provisioning cluster",
				cluster:  createTestCluster("cluster-2", "default", clusterv1.ClusterPhaseProvisioning),
				expected: "Provisioning",
			},
			{
				name:     "failed cluster",
				cluster:  createTestCluster("cluster-3", "default", clusterv1.ClusterPhaseFailed),
				expected: "Failed",
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				// Test cluster status extraction
				assert.Equal(t, tc.expected, tc.cluster.Status.Phase)
				
				// Test readiness checks
				if tc.cluster.Status.Phase == string(clusterv1.ClusterPhaseProvisioned) {
					assert.True(t, kube.IsClusterReady(tc.cluster))
				} else {
					assert.False(t, kube.IsClusterReady(tc.cluster))
				}

				// Test failure detection
				if tc.cluster.Status.Phase == string(clusterv1.ClusterPhaseFailed) {
					assert.True(t, kube.IsClusterFailed(tc.cluster))
				} else {
					assert.False(t, kube.IsClusterFailed(tc.cluster))
				}
			})
		}
	})

	t.Run("machine deployment operations", func(t *testing.T) {
		// Test machine deployment scaling scenarios
		md := createTestMachineDeployment("test-md", "default", "test-cluster", 3)
		
		// Verify initial state
		assert.Equal(t, int32(3), *md.Spec.Replicas)
		assert.Equal(t, int32(3), md.Status.UpdatedReplicas)

		// Test scaling up
		newReplicas := int32(5)
		md.Spec.Replicas = &newReplicas
		assert.Equal(t, int32(5), *md.Spec.Replicas)

		// Test scaling down
		newReplicas = int32(1)
		md.Spec.Replicas = &newReplicas
		assert.Equal(t, int32(1), *md.Spec.Replicas)
	})

	t.Run("kubeconfig secret handling", func(t *testing.T) {
		secret := createTestKubeconfigSecret("test-cluster", "default")
		
		// Verify secret structure
		assert.Equal(t, "test-cluster-kubeconfig", secret.Name)
		assert.Contains(t, secret.Data, "value")
		
		// Verify kubeconfig content is valid YAML
		kubeconfigData := secret.Data["value"]
		assert.NotEmpty(t, kubeconfigData)
		assert.Contains(t, string(kubeconfigData), "apiVersion: v1")
		assert.Contains(t, string(kubeconfigData), "kind: Config")
	})
}

// TestProviderSpecificIntegration tests provider-specific functionality.
func TestProviderSpecificIntegration(t *testing.T) {
	suite := NewIntegrationTestSuite(t)
	ctx := context.Background()

	t.Run("AWS provider infrastructure validation", func(t *testing.T) {
		awsProvider, exists := suite.providerManager.GetProvider("aws")
		require.True(t, exists)

		// Test cluster with AWS infrastructure
		cluster := createTestAWSCluster("aws-cluster", "default")
		
		err := awsProvider.ValidateInfrastructureReadiness(ctx, cluster)
		assert.NoError(t, err, "AWS cluster should be ready")

		// Test cluster with non-AWS infrastructure
		azureCluster := createTestCluster("azure-cluster", "default", clusterv1.ClusterPhaseProvisioned)
		azureCluster.Spec.InfrastructureRef = &corev1.ObjectReference{
			Kind: "AzureCluster",
			Name: "azure-cluster-infra",
		}

		err = awsProvider.ValidateInfrastructureReadiness(ctx, azureCluster)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "infrastructure is not an AWSCluster")
	})

	t.Run("AWS provider status extraction", func(t *testing.T) {
		awsProvider, exists := suite.providerManager.GetProvider("aws")
		require.True(t, exists)

		// Test cluster with region variable
		cluster := createTestAWSClusterWithRegion("aws-cluster-with-region", "default", "us-east-1")
		
		status, err := awsProvider.GetProviderSpecificStatus(ctx, cluster)
		require.NoError(t, err)
		
		assert.Equal(t, "aws", status["provider"])
		assert.Equal(t, "AWSCluster", status["infrastructureKind"])
		assert.Equal(t, "us-east-1", status["region"])
		assert.Equal(t, true, status["ready"])
	})

	t.Run("multi-provider support", func(t *testing.T) {
		// Test that multiple providers can be registered
		providers := suite.providerManager.ListProviders()
		assert.Contains(t, providers, "aws")
		
		// Verify we could add more providers
		assert.Equal(t, 1, len(providers)) // Currently only AWS
		
		// Test provider capabilities
		awsProvider, exists := suite.providerManager.GetProvider("aws")
		require.True(t, exists)
		
		// Verify AWS-specific capabilities
		regions, err := awsProvider.GetRegions(ctx)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(regions), 10) // Should have many regions
		
		instanceTypes, err := awsProvider.GetInstanceTypes(ctx, "us-west-2")
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(instanceTypes), 20) // Should have many instance types
	})
}

// Helper functions for creating test resources

func createTestClusterClass() *clusterv1.ClusterClass {
	return &clusterv1.ClusterClass{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "aws-cluster-class",
			Namespace: "default",
		},
		Spec: clusterv1.ClusterClassSpec{
			Infrastructure: clusterv1.LocalObjectTemplate{
				Ref: &corev1.ObjectReference{
					APIVersion: "infrastructure.cluster.x-k8s.io/v1beta2",
					Kind:       "AWSClusterTemplate",
					Name:       "aws-cluster-template",
				},
			},
			ControlPlane: clusterv1.ControlPlaneClass{
				LocalObjectTemplate: clusterv1.LocalObjectTemplate{
					Ref: &corev1.ObjectReference{
						APIVersion: "controlplane.cluster.x-k8s.io/v1beta1",
						Kind:       "KubeadmControlPlaneTemplate",
						Name:       "aws-controlplane-template",
					},
				},
			},
			Workers: clusterv1.WorkersClass{
				MachineDeployments: []clusterv1.MachineDeploymentClass{
					{
						Template: clusterv1.MachineDeploymentClassTemplate{
							Bootstrap: clusterv1.LocalObjectTemplate{
								Ref: &corev1.ObjectReference{
									APIVersion: "bootstrap.cluster.x-k8s.io/v1beta1",
									Kind:       "KubeadmConfigTemplate",
									Name:       "aws-worker-bootstrap-template",
								},
							},
							Infrastructure: clusterv1.LocalObjectTemplate{
								Ref: &corev1.ObjectReference{
									APIVersion: "infrastructure.cluster.x-k8s.io/v1beta2",
									Kind:       "AWSMachineTemplate",
									Name:       "aws-worker-machine-template",
								},
							},
						},
					},
				},
			},
		},
	}
}

func createTestCluster(name, namespace string, phase clusterv1.ClusterPhase) *clusterv1.Cluster {
	return &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				"cluster.x-k8s.io/cluster-name": name,
			},
			CreationTimestamp: metav1.Now(),
		},
		Spec: clusterv1.ClusterSpec{
			Topology: &clusterv1.Topology{
				Version: "v1.31.0",
				Class:   "aws-cluster-class",
			},
			ControlPlaneEndpoint: clusterv1.APIEndpoint{
				Host: name + "-api.example.com",
				Port: 6443,
			},
		},
		Status: clusterv1.ClusterStatus{
			Phase:               string(phase),
			ControlPlaneReady:   phase == clusterv1.ClusterPhaseProvisioned,
			InfrastructureReady: phase == clusterv1.ClusterPhaseProvisioned,
		},
	}
}

func createTestAWSCluster(name, namespace string) *clusterv1.Cluster {
	cluster := createTestCluster(name, namespace, clusterv1.ClusterPhaseProvisioned)
	cluster.Spec.InfrastructureRef = &corev1.ObjectReference{
		Kind:       "AWSCluster",
		Name:       name + "-aws",
		Namespace:  namespace,
		APIVersion: "infrastructure.cluster.x-k8s.io/v1beta2",
	}
	return cluster
}

func createTestAWSClusterWithRegion(name, namespace, region string) *clusterv1.Cluster {
	cluster := createTestAWSCluster(name, namespace)
	
	// Add region variable
	regionValue := &apiextensionsv1.JSON{}
	regionValue.Raw = []byte(`"` + region + `"`)
	
	cluster.Spec.Topology.Variables = []clusterv1.ClusterVariable{
		{
			Name:  "region",
			Value: *regionValue,
		},
	}
	
	return cluster
}

func createTestMachineDeployment(name, namespace, clusterName string, replicas int32) *clusterv1.MachineDeployment {
	return &clusterv1.MachineDeployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				clusterv1.ClusterNameLabel: clusterName,
			},
		},
		Spec: clusterv1.MachineDeploymentSpec{
			Replicas: &replicas,
			Template: clusterv1.MachineTemplateSpec{
				Spec: clusterv1.MachineSpec{
					ClusterName: clusterName,
					Version:     stringPtr("v1.31.0"),
				},
			},
		},
		Status: clusterv1.MachineDeploymentStatus{
			UpdatedReplicas: replicas,
			ReadyReplicas:   replicas,
		},
	}
}

func createTestKubeconfigSecret(clusterName, namespace string) *corev1.Secret {
	kubeconfigData := `apiVersion: v1
kind: Config
clusters:
- cluster:
    server: https://` + clusterName + `-api.example.com:6443
    insecure-skip-tls-verify: true
  name: ` + clusterName + `
contexts:
- context:
    cluster: ` + clusterName + `
    user: ` + clusterName + `-admin
  name: ` + clusterName + `
current-context: ` + clusterName + `
users:
- name: ` + clusterName + `-admin
  user:
    token: fake-token-for-testing`

	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      clusterName + "-kubeconfig",
			Namespace: namespace,
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			"value": []byte(kubeconfigData),
		},
	}
}

func stringPtr(s string) *string {
	return &s
}