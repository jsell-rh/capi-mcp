// +build e2e

package e2e

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// TestManagementCluster tests the basic functionality of the management cluster
func TestManagementCluster(t *testing.T) {
	RequireTestEnvironment(t)
	
	ctx, cancel := context.WithTimeout(context.Background(), DefaultTimeout)
	defer cancel()
	
	t.Run("kubernetes client connectivity", func(t *testing.T) {
		kubeClient := GetKubeClient()
		require.NotNil(t, kubeClient)
		
		// Test basic connectivity by listing namespaces
		namespaces := &corev1.NamespaceList{}
		err := kubeClient.List(ctx, namespaces)
		require.NoError(t, err)
		
		// Should have at least the default system namespaces
		assert.GreaterOrEqual(t, len(namespaces.Items), 4)
		
		// Check for required namespaces
		namespaceNames := make(map[string]bool)
		for _, ns := range namespaces.Items {
			namespaceNames[ns.Name] = true
		}
		
		assert.True(t, namespaceNames["default"])
		assert.True(t, namespaceNames["kube-system"])
		assert.True(t, namespaceNames["capi-system"])
		assert.True(t, namespaceNames["capa-system"])
		
		GetLogger().Info("Kubernetes client connectivity test passed",
			"namespaces_found", len(namespaces.Items))
	})
	
	t.Run("capi components health", func(t *testing.T) {
		kubeClient := GetKubeClient()
		
		// Check CAPI system pods
		capiPods := &corev1.PodList{}
		err := kubeClient.List(ctx, capiPods, client.InNamespace("capi-system"))
		require.NoError(t, err)
		assert.Greater(t, len(capiPods.Items), 0, "CAPI system should have running pods")
		
		// Check that all CAPI pods are running
		for _, pod := range capiPods.Items {
			assert.Equal(t, corev1.PodPhase("Running"), pod.Status.Phase,
				"CAPI pod %s should be running", pod.Name)
		}
		
		// Check CAPA system pods
		capaPods := &corev1.PodList{}
		err = kubeClient.List(ctx, capaPods, client.InNamespace("capa-system"))
		require.NoError(t, err)
		assert.Greater(t, len(capaPods.Items), 0, "CAPA system should have running pods")
		
		// Check that all CAPA pods are running
		for _, pod := range capaPods.Items {
			assert.Equal(t, corev1.PodPhase("Running"), pod.Status.Phase,
				"CAPA pod %s should be running", pod.Name)
		}
		
		GetLogger().Info("CAPI components health check passed",
			"capi_pods", len(capiPods.Items),
			"capa_pods", len(capaPods.Items))
	})
	
	t.Run("cluster classes available", func(t *testing.T) {
		clusterUtil := GetClusterUtil()
		
		// List available ClusterClasses
		clusterClasses, err := clusterUtil.GetClusterClasses(ctx, TestNamespace)
		require.NoError(t, err)
		
		// Should have at least our test ClusterClass
		assert.Greater(t, len(clusterClasses), 0, "Should have at least one ClusterClass")
		
		// Check for our test ClusterClass
		foundTestClass := false
		for _, cc := range clusterClasses {
			if cc.Name == "aws-test-cluster-class" {
				foundTestClass = true
				
				// Validate the ClusterClass structure
				assert.NotNil(t, cc.Spec.Infrastructure.Ref)
				assert.Equal(t, "AWSClusterTemplate", cc.Spec.Infrastructure.Ref.Kind)
				assert.NotNil(t, cc.Spec.ControlPlane.Ref)
				assert.Equal(t, "KubeadmControlPlaneTemplate", cc.Spec.ControlPlane.Ref.Kind)
				assert.NotNil(t, cc.Spec.Workers)
				assert.Greater(t, len(cc.Spec.Workers.MachineDeployments), 0)
				
				break
			}
		}
		
		assert.True(t, foundTestClass, "Test ClusterClass 'aws-test-cluster-class' should be available")
		
		GetLogger().Info("ClusterClass availability test passed",
			"cluster_classes_found", len(clusterClasses))
	})
	
	t.Run("mcp server health", func(t *testing.T) {
		mcpClient := GetMCPClient()
		require.NotNil(t, mcpClient)
		
		// Test MCP server connectivity
		err := mcpClient.TestConnection()
		require.NoError(t, err, "MCP server should be accessible")
		
		GetLogger().Info("MCP server health check passed")
	})
	
	t.Run("mcp server basic tool functionality", func(t *testing.T) {
		mcpClient := GetMCPClient()
		
		// Test list_clusters tool (should return empty list initially)
		listResult, err := mcpClient.ListClusters(ctx)
		require.NoError(t, err, "list_clusters tool should work")
		
		// Initially should have no clusters
		assert.NotNil(t, listResult)
		assert.Equal(t, 0, len(listResult.Clusters), "Should have no clusters initially")
		
		GetLogger().Info("MCP server basic tool functionality test passed")
	})
}

// TestManagementClusterStability tests the stability of the management cluster under load
func TestManagementClusterStability(t *testing.T) {
	RequireTestEnvironment(t)
	
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	
	t.Run("repeated api calls", func(t *testing.T) {
		clusterUtil := GetClusterUtil()
		
		// Make repeated API calls to test stability
		for i := 0; i < 10; i++ {
			// List clusters
			clusters, err := clusterUtil.ListClusters(ctx, TestNamespace)
			require.NoError(t, err, "API call %d should succeed", i+1)
			assert.NotNil(t, clusters)
			
			// List ClusterClasses
			clusterClasses, err := clusterUtil.GetClusterClasses(ctx, TestNamespace)
			require.NoError(t, err, "API call %d should succeed", i+1)
			assert.Greater(t, len(clusterClasses), 0)
			
			// Brief pause between calls
			time.Sleep(100 * time.Millisecond)
		}
		
		GetLogger().Info("Repeated API calls test passed")
	})
	
	t.Run("mcp server stability", func(t *testing.T) {
		mcpClient := GetMCPClient()
		
		// Make repeated MCP calls to test stability
		for i := 0; i < 5; i++ {
			// Test connection
			err := mcpClient.TestConnection()
			require.NoError(t, err, "MCP connection %d should succeed", i+1)
			
			// List clusters
			listResult, err := mcpClient.ListClusters(ctx)
			require.NoError(t, err, "MCP call %d should succeed", i+1)
			assert.NotNil(t, listResult)
			
			// Brief pause between calls
			time.Sleep(500 * time.Millisecond)
		}
		
		GetLogger().Info("MCP server stability test passed")
	})
}

// TestManagementClusterResources tests the availability of required Kubernetes resources
func TestManagementClusterResources(t *testing.T) {
	RequireTestEnvironment(t)
	
	ctx, cancel := context.WithTimeout(context.Background(), DefaultTimeout)
	defer cancel()
	
	t.Run("capi crds installed", func(t *testing.T) {
		kubeClient := GetKubeClient()
		
		// List of CRDs that should be installed
		requiredCRDs := []string{
			"clusters.cluster.x-k8s.io",
			"clusterclasses.cluster.x-k8s.io", 
			"machinedeployments.cluster.x-k8s.io",
			"machines.cluster.x-k8s.io",
			"awsclusters.infrastructure.cluster.x-k8s.io",
			"awsmachines.infrastructure.cluster.x-k8s.io",
		}
		
		// Check each CRD
		for _, crdName := range requiredCRDs {
			crd := &apiextensionsv1.CustomResourceDefinition{}
			err := kubeClient.Get(ctx, client.ObjectKey{Name: crdName}, crd)
			require.NoError(t, err, "CRD %s should be installed", crdName)
			
			// Check that CRD is established
			established := false
			for _, condition := range crd.Status.Conditions {
				if condition.Type == apiextensionsv1.Established && condition.Status == apiextensionsv1.ConditionTrue {
					established = true
					break
				}
			}
			assert.True(t, established, "CRD %s should be established", crdName)
		}
		
		GetLogger().Info("CAPI CRDs installation test passed",
			"required_crds", len(requiredCRDs))
	})
	
	t.Run("rbac configuration", func(t *testing.T) {
		kubeClient := GetKubeClient()
		
		// Check that MCP server ServiceAccount exists
		sa := &corev1.ServiceAccount{}
		err := kubeClient.Get(ctx, client.ObjectKey{
			Name:      MCPServerName,
			Namespace: MCPServerNamespace,
		}, sa)
		require.NoError(t, err, "MCP server ServiceAccount should exist")
		
		// Check that ClusterRole exists
		clusterRole := &rbacv1.ClusterRole{}
		err = kubeClient.Get(ctx, client.ObjectKey{
			Name: MCPServerName,
		}, clusterRole)
		require.NoError(t, err, "MCP server ClusterRole should exist")
		
		// Verify key permissions
		hasClusterPermissions := false
		for _, rule := range clusterRole.Rules {
			for _, apiGroup := range rule.APIGroups {
				if apiGroup == "cluster.x-k8s.io" {
					hasClusterPermissions = true
					break
				}
			}
		}
		assert.True(t, hasClusterPermissions, "ClusterRole should have CAPI permissions")
		
		GetLogger().Info("RBAC configuration test passed")
	})
}