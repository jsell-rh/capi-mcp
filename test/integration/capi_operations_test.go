package integration

import (
	"context"
	"log/slog"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/capi-mcp/capi-mcp-server/internal/kube"
	"github.com/capi-mcp/capi-mcp-server/internal/service"
	"github.com/capi-mcp/capi-mcp-server/pkg/provider"
	"github.com/capi-mcp/capi-mcp-server/pkg/provider/aws"
)

// CAPIOperationsTestSuite provides integration testing for CAPI operations
// with enhanced fake Kubernetes clients that simulate real CAPI behavior.
type CAPIOperationsTestSuite struct {
	client          client.Client
	clusterService  *service.ClusterService
	providerManager *provider.ProviderManager
	scheme          *runtime.Scheme
	logger          *slog.Logger
	namespace       string
}

// NewCAPIOperationsTestSuite creates a new CAPI operations integration test suite.
func NewCAPIOperationsTestSuite(t *testing.T) *CAPIOperationsTestSuite {
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

	suite := &CAPIOperationsTestSuite{
		providerManager: providerManager,
		scheme:          scheme,
		logger:          logger,
		namespace:       "default",
	}

	return suite
}

// SetupWithResources creates a fake Kubernetes client with the provided resources.
func (s *CAPIOperationsTestSuite) SetupWithResources(t *testing.T, objects ...client.Object) {
	// Create fake client with provided objects and status subresource support
	s.client = fake.NewClientBuilder().
		WithScheme(s.scheme).
		WithObjects(objects...).
		WithStatusSubresource(&clusterv1.Cluster{}, &clusterv1.MachineDeployment{}).
		Build()

	// Create enhanced kube client that uses our fake client
	// Note: In real integration tests, we would create a wrapper that injects the fake client
	// For now, we'll test the operations that can work with the fake client directly
}

// TestCAPIResourceOperations tests CAPI resource operations with fake clients.
func TestCAPIResourceOperations(t *testing.T) {
	suite := NewCAPIOperationsTestSuite(t)
	ctx := context.Background()

	t.Run("cluster lifecycle operations", func(t *testing.T) {
		// Create test resources
		clusterClass := createTestClusterClass()
		cluster := createTestCluster("test-cluster", suite.namespace, clusterv1.ClusterPhaseProvisioning)

		suite.SetupWithResources(t, clusterClass, cluster)

		// Test cluster retrieval
		var retrievedCluster clusterv1.Cluster
		err := suite.client.Get(ctx, types.NamespacedName{
			Name:      "test-cluster",
			Namespace: suite.namespace,
		}, &retrievedCluster)
		require.NoError(t, err)
		assert.Equal(t, "test-cluster", retrievedCluster.Name)
		assert.Equal(t, string(clusterv1.ClusterPhaseProvisioning), retrievedCluster.Status.Phase)

		// Test cluster status update
		retrievedCluster.Status.Phase = string(clusterv1.ClusterPhaseProvisioned)
		retrievedCluster.Status.ControlPlaneReady = true
		retrievedCluster.Status.InfrastructureReady = true

		err = suite.client.Status().Update(ctx, &retrievedCluster)
		require.NoError(t, err)

		// Verify status update
		var updatedCluster clusterv1.Cluster
		err = suite.client.Get(ctx, types.NamespacedName{
			Name:      "test-cluster",
			Namespace: suite.namespace,
		}, &updatedCluster)
		require.NoError(t, err)
		assert.Equal(t, string(clusterv1.ClusterPhaseProvisioned), updatedCluster.Status.Phase)
		assert.True(t, updatedCluster.Status.ControlPlaneReady)
		assert.True(t, updatedCluster.Status.InfrastructureReady)

		// Test cluster deletion
		err = suite.client.Delete(ctx, &retrievedCluster)
		require.NoError(t, err)

		// Verify deletion (should not be found)
		err = suite.client.Get(ctx, types.NamespacedName{
			Name:      "test-cluster",
			Namespace: suite.namespace,
		}, &retrievedCluster)
		assert.Error(t, err)
	})

	t.Run("machine deployment operations", func(t *testing.T) {
		// Create test resources
		cluster := createTestCluster("md-cluster", suite.namespace, clusterv1.ClusterPhaseProvisioned)
		machineDeployment := createTestMachineDeployment("md-cluster-workers", suite.namespace, "md-cluster", 3)

		suite.SetupWithResources(t, cluster, machineDeployment)

		// Test machine deployment retrieval
		var retrievedMD clusterv1.MachineDeployment
		err := suite.client.Get(ctx, types.NamespacedName{
			Name:      "md-cluster-workers",
			Namespace: suite.namespace,
		}, &retrievedMD)
		require.NoError(t, err)
		assert.Equal(t, "md-cluster-workers", retrievedMD.Name)
		assert.Equal(t, int32(3), *retrievedMD.Spec.Replicas)

		// Test scaling operation
		newReplicas := int32(5)
		retrievedMD.Spec.Replicas = &newReplicas

		err = suite.client.Update(ctx, &retrievedMD)
		require.NoError(t, err)

		// Verify scaling
		var scaledMD clusterv1.MachineDeployment
		err = suite.client.Get(ctx, types.NamespacedName{
			Name:      "md-cluster-workers",
			Namespace: suite.namespace,
		}, &scaledMD)
		require.NoError(t, err)
		assert.Equal(t, int32(5), *scaledMD.Spec.Replicas)

		// Simulate status update (as controller would do)
		scaledMD.Status.UpdatedReplicas = newReplicas
		scaledMD.Status.ReadyReplicas = newReplicas

		err = suite.client.Status().Update(ctx, &scaledMD)
		require.NoError(t, err)

		// Verify status update
		var finalMD clusterv1.MachineDeployment
		err = suite.client.Get(ctx, types.NamespacedName{
			Name:      "md-cluster-workers",
			Namespace: suite.namespace,
		}, &finalMD)
		require.NoError(t, err)
		assert.Equal(t, int32(5), finalMD.Status.UpdatedReplicas)
		assert.Equal(t, int32(5), finalMD.Status.ReadyReplicas)
	})

	t.Run("secret operations", func(t *testing.T) {
		// Create test resources
		cluster := createTestCluster("secret-cluster", suite.namespace, clusterv1.ClusterPhaseProvisioned)
		kubeconfigSecret := createTestKubeconfigSecret("secret-cluster", suite.namespace)

		suite.SetupWithResources(t, cluster, kubeconfigSecret)

		// Test secret retrieval
		var retrievedSecret corev1.Secret
		err := suite.client.Get(ctx, types.NamespacedName{
			Name:      "secret-cluster-kubeconfig",
			Namespace: suite.namespace,
		}, &retrievedSecret)
		require.NoError(t, err)
		assert.Equal(t, "secret-cluster-kubeconfig", retrievedSecret.Name)
		assert.Contains(t, retrievedSecret.Data, "value")

		// Test kubeconfig content
		kubeconfigData := retrievedSecret.Data["value"]
		assert.NotEmpty(t, kubeconfigData)
		assert.Contains(t, string(kubeconfigData), "secret-cluster-api.example.com")
		assert.Contains(t, string(kubeconfigData), "apiVersion: v1")
		assert.Contains(t, string(kubeconfigData), "kind: Config")

		// Test secret update
		newKubeconfigData := `apiVersion: v1
kind: Config
clusters:
- cluster:
    server: https://updated-secret-cluster-api.example.com:6443
  name: secret-cluster
contexts:
- context:
    cluster: secret-cluster
  name: secret-cluster
current-context: secret-cluster`

		retrievedSecret.Data["value"] = []byte(newKubeconfigData)
		err = suite.client.Update(ctx, &retrievedSecret)
		require.NoError(t, err)

		// Verify update
		var updatedSecret corev1.Secret
		err = suite.client.Get(ctx, types.NamespacedName{
			Name:      "secret-cluster-kubeconfig",
			Namespace: suite.namespace,
		}, &updatedSecret)
		require.NoError(t, err)
		assert.Contains(t, string(updatedSecret.Data["value"]), "updated-secret-cluster-api.example.com")
	})

	t.Run("cluster class operations", func(t *testing.T) {
		// Create test resources
		clusterClass := createTestClusterClass()

		suite.SetupWithResources(t, clusterClass)

		// Test cluster class retrieval
		var retrievedClusterClass clusterv1.ClusterClass
		err := suite.client.Get(ctx, types.NamespacedName{
			Name:      "aws-cluster-class",
			Namespace: suite.namespace,
		}, &retrievedClusterClass)
		require.NoError(t, err)
		assert.Equal(t, "aws-cluster-class", retrievedClusterClass.Name)
		assert.NotNil(t, retrievedClusterClass.Spec.Infrastructure.Ref)
		assert.Equal(t, "AWSClusterTemplate", retrievedClusterClass.Spec.Infrastructure.Ref.Kind)

		// Test cluster class listing
		var clusterClassList clusterv1.ClusterClassList
		err = suite.client.List(ctx, &clusterClassList, client.InNamespace(suite.namespace))
		require.NoError(t, err)
		assert.Len(t, clusterClassList.Items, 1)
		assert.Equal(t, "aws-cluster-class", clusterClassList.Items[0].Name)
	})
}

// TestAdvancedCAPIOperations tests more complex CAPI operations and workflows.
func TestAdvancedCAPIOperations(t *testing.T) {
	suite := NewCAPIOperationsTestSuite(t)
	ctx := context.Background()

	t.Run("multi-cluster operations", func(t *testing.T) {
		// Create multiple clusters with different states
		clusterClass := createTestClusterClass()
		cluster1 := createTestCluster("cluster-1", suite.namespace, clusterv1.ClusterPhaseProvisioned)
		cluster2 := createTestCluster("cluster-2", suite.namespace, clusterv1.ClusterPhaseProvisioning)
		cluster3 := createTestCluster("cluster-3", suite.namespace, clusterv1.ClusterPhaseFailed)

		suite.SetupWithResources(t, clusterClass, cluster1, cluster2, cluster3)

		// Test listing all clusters
		var clusterList clusterv1.ClusterList
		err := suite.client.List(ctx, &clusterList, client.InNamespace(suite.namespace))
		require.NoError(t, err)
		assert.Len(t, clusterList.Items, 3)

		// Test filtering clusters by phase
		provisionedClusters := 0
		provisioningClusters := 0
		failedClusters := 0

		for _, cluster := range clusterList.Items {
			switch cluster.Status.Phase {
			case string(clusterv1.ClusterPhaseProvisioned):
				provisionedClusters++
				assert.True(t, kube.IsClusterReady(&cluster))
				assert.False(t, kube.IsClusterFailed(&cluster))
			case string(clusterv1.ClusterPhaseProvisioning):
				provisioningClusters++
				assert.False(t, kube.IsClusterReady(&cluster))
				assert.False(t, kube.IsClusterFailed(&cluster))
			case string(clusterv1.ClusterPhaseFailed):
				failedClusters++
				assert.False(t, kube.IsClusterReady(&cluster))
				assert.True(t, kube.IsClusterFailed(&cluster))
			}
		}

		assert.Equal(t, 1, provisionedClusters)
		assert.Equal(t, 1, provisioningClusters)
		assert.Equal(t, 1, failedClusters)
	})

	t.Run("cluster with machine deployments", func(t *testing.T) {
		// Create cluster with multiple machine deployments
		cluster := createTestCluster("multi-md-cluster", suite.namespace, clusterv1.ClusterPhaseProvisioned)
		workerMD := createTestMachineDeployment("multi-md-cluster-workers", suite.namespace, "multi-md-cluster", 3)
		infraMD := createTestMachineDeployment("multi-md-cluster-infra", suite.namespace, "multi-md-cluster", 2)

		suite.SetupWithResources(t, cluster, workerMD, infraMD)

		// Test listing machine deployments for cluster
		var mdList clusterv1.MachineDeploymentList
		err := suite.client.List(ctx, &mdList,
			client.InNamespace(suite.namespace),
			client.MatchingLabels{clusterv1.ClusterNameLabel: "multi-md-cluster"})
		require.NoError(t, err)
		assert.Len(t, mdList.Items, 2)

		// Calculate total node count
		totalNodes := int32(0)
		for _, md := range mdList.Items {
			totalNodes += *md.Spec.Replicas
		}
		assert.Equal(t, int32(5), totalNodes)

		// Test scaling multiple machine deployments
		for i := range mdList.Items {
			currentReplicas := *mdList.Items[i].Spec.Replicas
			newReplicas := currentReplicas + 1
			mdList.Items[i].Spec.Replicas = &newReplicas

			err = suite.client.Update(ctx, &mdList.Items[i])
			require.NoError(t, err)
		}

		// Verify scaling
		var updatedMDList clusterv1.MachineDeploymentList
		err = suite.client.List(ctx, &updatedMDList,
			client.InNamespace(suite.namespace),
			client.MatchingLabels{clusterv1.ClusterNameLabel: "multi-md-cluster"})
		require.NoError(t, err)

		newTotalNodes := int32(0)
		for _, md := range updatedMDList.Items {
			newTotalNodes += *md.Spec.Replicas
		}
		assert.Equal(t, int32(7), newTotalNodes) // 3+1 + 2+1 = 7
	})

	t.Run("cluster deletion workflow", func(t *testing.T) {
		// Create cluster with associated resources
		cluster := createTestCluster("deletion-cluster", suite.namespace, clusterv1.ClusterPhaseProvisioned)
		machineDeployment := createTestMachineDeployment("deletion-cluster-md", suite.namespace, "deletion-cluster", 3)
		kubeconfigSecret := createTestKubeconfigSecret("deletion-cluster", suite.namespace)

		suite.SetupWithResources(t, cluster, machineDeployment, kubeconfigSecret)

		// Verify resources exist
		var existingCluster clusterv1.Cluster
		err := suite.client.Get(ctx, types.NamespacedName{
			Name:      "deletion-cluster",
			Namespace: suite.namespace,
		}, &existingCluster)
		require.NoError(t, err)

		var existingMD clusterv1.MachineDeployment
		err = suite.client.Get(ctx, types.NamespacedName{
			Name:      "deletion-cluster-md",
			Namespace: suite.namespace,
		}, &existingMD)
		require.NoError(t, err)

		var existingSecret corev1.Secret
		err = suite.client.Get(ctx, types.NamespacedName{
			Name:      "deletion-cluster-kubeconfig",
			Namespace: suite.namespace,
		}, &existingSecret)
		require.NoError(t, err)

		// Simulate cluster deletion workflow
		// Step 1: Mark cluster for deletion (skip this as it's immutable in fake client)
		// In real scenarios, deletion timestamp is set by the API server

		// Step 2: Delete associated machine deployments
		err = suite.client.Delete(ctx, &existingMD)
		require.NoError(t, err)

		// Step 3: Delete associated secrets
		err = suite.client.Delete(ctx, &existingSecret)
		require.NoError(t, err)

		// Step 4: Delete cluster
		err = suite.client.Delete(ctx, &existingCluster)
		require.NoError(t, err)

		// Verify deletion
		err = suite.client.Get(ctx, types.NamespacedName{
			Name:      "deletion-cluster",
			Namespace: suite.namespace,
		}, &existingCluster)
		assert.Error(t, err) // Should not be found

		err = suite.client.Get(ctx, types.NamespacedName{
			Name:      "deletion-cluster-md",
			Namespace: suite.namespace,
		}, &existingMD)
		assert.Error(t, err) // Should not be found

		err = suite.client.Get(ctx, types.NamespacedName{
			Name:      "deletion-cluster-kubeconfig",
			Namespace: suite.namespace,
		}, &existingSecret)
		assert.Error(t, err) // Should not be found
	})
}

// TestCAPIResourceValidation tests validation of CAPI resources.
func TestCAPIResourceValidation(t *testing.T) {
	suite := NewCAPIOperationsTestSuite(t)
	ctx := context.Background()

	t.Run("cluster validation", func(t *testing.T) {
		// Test cluster with valid topology
		validCluster := createTestCluster("valid-cluster", suite.namespace, clusterv1.ClusterPhaseProvisioning)
		suite.SetupWithResources(t, validCluster)

		var retrievedCluster clusterv1.Cluster
		err := suite.client.Get(ctx, types.NamespacedName{
			Name:      "valid-cluster",
			Namespace: suite.namespace,
		}, &retrievedCluster)
		require.NoError(t, err)

		// Validate cluster structure
		assert.NotNil(t, retrievedCluster.Spec.Topology)
		assert.Equal(t, "aws-cluster-class", retrievedCluster.Spec.Topology.Class)
		assert.Equal(t, "v1.31.0", retrievedCluster.Spec.Topology.Version)
		assert.NotEmpty(t, retrievedCluster.Spec.ControlPlaneEndpoint.Host)
		assert.Equal(t, int32(6443), retrievedCluster.Spec.ControlPlaneEndpoint.Port)
	})

	t.Run("machine deployment validation", func(t *testing.T) {
		// Test machine deployment with valid configuration
		validMD := createTestMachineDeployment("valid-md", suite.namespace, "test-cluster", 3)
		suite.SetupWithResources(t, validMD)

		var retrievedMD clusterv1.MachineDeployment
		err := suite.client.Get(ctx, types.NamespacedName{
			Name:      "valid-md",
			Namespace: suite.namespace,
		}, &retrievedMD)
		require.NoError(t, err)

		// Validate machine deployment structure
		assert.Equal(t, int32(3), *retrievedMD.Spec.Replicas)
		assert.Equal(t, "test-cluster", retrievedMD.Spec.Template.Spec.ClusterName)
		assert.Equal(t, "v1.31.0", *retrievedMD.Spec.Template.Spec.Version)
		assert.Equal(t, "test-cluster", retrievedMD.Labels[clusterv1.ClusterNameLabel])
	})

	t.Run("cluster class validation", func(t *testing.T) {
		// Test cluster class with valid structure
		validClusterClass := createTestClusterClass()
		suite.SetupWithResources(t, validClusterClass)

		var retrievedClusterClass clusterv1.ClusterClass
		err := suite.client.Get(ctx, types.NamespacedName{
			Name:      "aws-cluster-class",
			Namespace: suite.namespace,
		}, &retrievedClusterClass)
		require.NoError(t, err)

		// Validate cluster class structure
		assert.NotNil(t, retrievedClusterClass.Spec.Infrastructure.Ref)
		assert.Equal(t, "AWSClusterTemplate", retrievedClusterClass.Spec.Infrastructure.Ref.Kind)
		assert.NotNil(t, retrievedClusterClass.Spec.ControlPlane.LocalObjectTemplate.Ref)
		assert.Equal(t, "KubeadmControlPlaneTemplate", retrievedClusterClass.Spec.ControlPlane.LocalObjectTemplate.Ref.Kind)
		assert.NotNil(t, retrievedClusterClass.Spec.Workers)
		assert.Len(t, retrievedClusterClass.Spec.Workers.MachineDeployments, 1)
	})
}
