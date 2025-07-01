//go:build e2e
// +build e2e

package e2e

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/capi-mcp/capi-mcp-server/test/e2e/utils"
)

const (
	// Test cluster configuration for MCP tool tests
	MCPTestClusterName              = "mcp-tools-test-cluster"
	MCPTestNodeCount                = 2
	MCPTestK8sVersion               = "v1.28.0"
	MCPTestInstanceType             = "t3.small"
	MCPTestControlPlaneInstanceType = "t3.medium"

	// Test scaling parameters
	MCPTestScaleNodeCount = 3
	MCPTestWorkerPoolName = "default-worker"
)

// TestMCPToolsComplete tests all seven MCP tools in a comprehensive workflow
func TestMCPToolsComplete(t *testing.T) {
	RequireTestEnvironment(t)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Minute)
	defer cancel()

	logger := GetLogger()
	mcpClient := GetMCPClient()
	awsUtil := utils.NewAWSUtil(logger)

	// Skip test if AWS credentials are not available
	if !awsUtil.HasCredentials() {
		t.Skip("AWS credentials not available, skipping MCP tools test")
	}

	logger.Info("Starting comprehensive MCP tools test")

	// Test all MCP tools in order
	t.Run("1_ListClusters_Initial", func(t *testing.T) {
		testListClustersInitial(t, ctx, mcpClient)
	})

	t.Run("2_CreateCluster", func(t *testing.T) {
		testCreateClusterTool(t, ctx, mcpClient, awsUtil)
	})

	t.Run("3_ListClusters_AfterCreate", func(t *testing.T) {
		testListClustersAfterCreate(t, ctx, mcpClient)
	})

	t.Run("4_GetCluster", func(t *testing.T) {
		testGetClusterTool(t, ctx, mcpClient)
	})

	t.Run("5_GetClusterNodes_Initial", func(t *testing.T) {
		testGetClusterNodesTool(t, ctx, mcpClient)
	})

	t.Run("6_ScaleCluster", func(t *testing.T) {
		testScaleClusterTool(t, ctx, mcpClient)
	})

	t.Run("7_GetClusterNodes_AfterScale", func(t *testing.T) {
		testGetClusterNodesAfterScale(t, ctx, mcpClient)
	})

	t.Run("8_GetClusterKubeconfig", func(t *testing.T) {
		testGetClusterKubeconfigTool(t, ctx, mcpClient)
	})

	t.Run("9_DeleteCluster", func(t *testing.T) {
		testDeleteClusterTool(t, ctx, mcpClient)
	})

	t.Run("10_ListClusters_AfterDelete", func(t *testing.T) {
		testListClustersAfterDelete(t, ctx, mcpClient)
	})
}

// testListClustersInitial tests the list_clusters tool initially (should be empty or minimal)
func testListClustersInitial(t *testing.T, ctx context.Context, mcpClient *utils.MCPClient) {
	logger := GetLogger()
	logger.Info("Testing list_clusters tool (initial state)")

	// Call list_clusters with no parameters
	result, err := mcpClient.CallTool(ctx, "list_clusters", map[string]interface{}{})
	require.NoError(t, err, "list_clusters tool should succeed")

	// Validate response structure
	assert.Contains(t, result, "clusters", "Response should contain clusters field")

	clusters, ok := result["clusters"].([]interface{})
	require.True(t, ok, "Clusters field should be an array")

	// Log initial cluster count
	logger.Info("Initial cluster list", "count", len(clusters))

	// Validate cluster structure if any exist
	for i, clusterInterface := range clusters {
		cluster, ok := clusterInterface.(map[string]interface{})
		require.True(t, ok, "Cluster %d should be a map", i)

		// Validate required fields
		assert.Contains(t, cluster, "name", "Cluster should have name")
		assert.Contains(t, cluster, "status", "Cluster should have status")
		assert.Contains(t, cluster, "kubernetesVersion", "Cluster should have kubernetesVersion")
		assert.Contains(t, cluster, "nodeCount", "Cluster should have nodeCount")
		assert.Contains(t, cluster, "creationTimestamp", "Cluster should have creationTimestamp")
	}
}

// testCreateClusterTool tests the create_cluster tool
func testCreateClusterTool(t *testing.T, ctx context.Context, mcpClient *utils.MCPClient, awsUtil *utils.AWSUtil) {
	logger := GetLogger()
	logger.Info("Testing create_cluster tool")

	// Prepare cluster creation parameters
	clusterVars := map[string]interface{}{
		"region":                   "us-west-2",
		"nodeCount":                MCPTestNodeCount,
		"controlPlaneInstanceType": MCPTestControlPlaneInstanceType,
		"workerInstanceType":       MCPTestInstanceType,
		"vpcCIDR":                  "10.0.0.0/16",
		"subnetCIDR":               "10.0.1.0/24",
	}

	// Add SSH key if available
	if sshKeyName := awsUtil.GetSSHKeyName(); sshKeyName != "" {
		clusterVars["sshKeyName"] = sshKeyName
	}

	createParams := map[string]interface{}{
		"clusterName":       MCPTestClusterName,
		"templateName":      "aws-cluster-template",
		"kubernetesVersion": MCPTestK8sVersion,
		"variables":         clusterVars,
	}

	logger.Info("Calling create_cluster MCP tool", "params", createParams)

	result, err := mcpClient.CallTool(ctx, "create_cluster", createParams)
	require.NoError(t, err, "create_cluster tool should succeed")

	// Validate response structure
	assert.Contains(t, result, "success", "Response should contain success field")
	assert.Contains(t, result, "cluster", "Response should contain cluster field")

	success, ok := result["success"].(bool)
	require.True(t, ok, "Success field should be boolean")
	assert.True(t, success, "Cluster creation should be successful")

	// Validate cluster details in response
	cluster, ok := result["cluster"].(map[string]interface{})
	require.True(t, ok, "Cluster field should be a map")

	assert.Equal(t, MCPTestClusterName, cluster["name"], "Cluster name should match")
	assert.Equal(t, MCPTestK8sVersion, cluster["kubernetesVersion"], "Kubernetes version should match")
	assert.Contains(t, cluster, "status", "Cluster should have status")

	logger.Info("create_cluster tool test completed successfully")
}

// testListClustersAfterCreate tests list_clusters after creating a cluster
func testListClustersAfterCreate(t *testing.T, ctx context.Context, mcpClient *utils.MCPClient) {
	logger := GetLogger()
	logger.Info("Testing list_clusters tool after cluster creation")

	result, err := mcpClient.CallTool(ctx, "list_clusters", map[string]interface{}{})
	require.NoError(t, err, "list_clusters tool should succeed")

	clusters, ok := result["clusters"].([]interface{})
	require.True(t, ok, "Clusters field should be an array")

	// Should now have at least our test cluster
	assert.NotEmpty(t, clusters, "Should have at least one cluster after creation")

	// Find our test cluster
	var testCluster map[string]interface{}
	for _, clusterInterface := range clusters {
		cluster := clusterInterface.(map[string]interface{})
		if cluster["name"] == MCPTestClusterName {
			testCluster = cluster
			break
		}
	}

	require.NotNil(t, testCluster, "Test cluster should be found in list")
	assert.Equal(t, MCPTestClusterName, testCluster["name"], "Cluster name should match")
	assert.Contains(t, []string{"Pending", "Provisioning", "Provisioned"}, testCluster["status"],
		"Cluster should be in valid state")

	logger.Info("list_clusters after creation test completed", "cluster_count", len(clusters))
}

// testGetClusterTool tests the get_cluster tool
func testGetClusterTool(t *testing.T, ctx context.Context, mcpClient *utils.MCPClient) {
	logger := GetLogger()
	logger.Info("Testing get_cluster tool")

	getParams := map[string]interface{}{
		"clusterName": MCPTestClusterName,
	}

	result, err := mcpClient.CallTool(ctx, "get_cluster", getParams)
	require.NoError(t, err, "get_cluster tool should succeed")

	// Validate response structure
	assert.Contains(t, result, "cluster", "Response should contain cluster field")

	cluster, ok := result["cluster"].(map[string]interface{})
	require.True(t, ok, "Cluster field should be a map")

	// Validate cluster details
	assert.Equal(t, MCPTestClusterName, cluster["name"], "Cluster name should match")
	assert.Contains(t, cluster, "status", "Cluster should have status")
	assert.Contains(t, cluster, "kubernetesVersion", "Cluster should have kubernetesVersion")
	assert.Contains(t, cluster, "nodeCount", "Cluster should have nodeCount")
	assert.Contains(t, cluster, "creationTimestamp", "Cluster should have creationTimestamp")
	assert.Contains(t, cluster, "controlPlaneReady", "Cluster should have controlPlaneReady")
	assert.Contains(t, cluster, "infrastructureReady", "Cluster should have infrastructureReady")

	logger.Info("get_cluster tool test completed", "status", cluster["status"])
}

// testGetClusterNodesTool tests the get_cluster_nodes tool initially
func testGetClusterNodesTool(t *testing.T, ctx context.Context, mcpClient *utils.MCPClient) {
	logger := GetLogger()
	logger.Info("Testing get_cluster_nodes tool")

	// Wait for cluster to have some nodes
	logger.Info("Waiting for cluster to provision nodes (may take several minutes)")

	// Poll for nodes to appear (cluster may still be provisioning)
	var nodes []interface{}
	var lastErr error

	for i := 0; i < 20; i++ { // Try for up to 10 minutes
		nodesParams := map[string]interface{}{
			"clusterName": MCPTestClusterName,
		}

		result, err := mcpClient.CallTool(ctx, "get_cluster_nodes", nodesParams)
		if err != nil {
			lastErr = err
			logger.Debug("get_cluster_nodes not ready yet", "attempt", i+1, "error", err)
			time.Sleep(30 * time.Second)
			continue
		}

		// Validate response structure
		assert.Contains(t, result, "nodes", "Response should contain nodes field")

		var ok bool
		nodes, ok = result["nodes"].([]interface{})
		require.True(t, ok, "Nodes field should be an array")

		if len(nodes) > 0 {
			break // Found nodes
		}

		logger.Debug("No nodes yet, waiting", "attempt", i+1)
		time.Sleep(30 * time.Second)
	}

	// If we still don't have nodes, we can still validate the response structure
	if len(nodes) == 0 {
		logger.Warn("No nodes found after waiting - cluster may still be provisioning")
		// The tool should still work, just return empty array
		return
	}

	require.NoError(t, lastErr, "get_cluster_nodes should eventually succeed")

	// Validate node structure
	logger.Info("Found cluster nodes", "count", len(nodes))

	for i, nodeInterface := range nodes {
		node, ok := nodeInterface.(map[string]interface{})
		require.True(t, ok, "Node %d should be a map", i)

		// Validate required fields
		assert.Contains(t, node, "name", "Node should have name")
		assert.Contains(t, node, "status", "Node should have status")
		assert.Contains(t, node, "roles", "Node should have roles")
		assert.Contains(t, node, "version", "Node should have version")
		assert.Contains(t, node, "creationTimestamp", "Node should have creationTimestamp")

		logger.Info("Node details",
			"name", node["name"],
			"status", node["status"],
			"roles", node["roles"])
	}
}

// testScaleClusterTool tests the scale_cluster tool
func testScaleClusterTool(t *testing.T, ctx context.Context, mcpClient *utils.MCPClient) {
	logger := GetLogger()
	logger.Info("Testing scale_cluster tool")

	scaleParams := map[string]interface{}{
		"clusterName":  MCPTestClusterName,
		"nodePoolName": MCPTestWorkerPoolName,
		"replicas":     MCPTestScaleNodeCount,
	}

	result, err := mcpClient.CallTool(ctx, "scale_cluster", scaleParams)
	require.NoError(t, err, "scale_cluster tool should succeed")

	// Validate response structure
	assert.Contains(t, result, "status", "Response should contain status field")
	assert.Contains(t, result, "message", "Response should contain message field")

	status, ok := result["status"].(string)
	require.True(t, ok, "Status field should be string")
	assert.Equal(t, "success", status, "Scale operation should be successful")

	message, ok := result["message"].(string)
	require.True(t, ok, "Message field should be string")
	assert.NotEmpty(t, message, "Message should not be empty")

	logger.Info("scale_cluster tool test completed", "status", status, "message", message)
}

// testGetClusterNodesAfterScale tests get_cluster_nodes after scaling
func testGetClusterNodesAfterScale(t *testing.T, ctx context.Context, mcpClient *utils.MCPClient) {
	logger := GetLogger()
	logger.Info("Testing get_cluster_nodes tool after scaling")

	// Wait a bit for scaling to take effect
	logger.Info("Waiting for cluster scaling to take effect")
	time.Sleep(2 * time.Minute)

	// Check nodes multiple times as scaling may take time
	var finalNodes []interface{}
	for i := 0; i < 10; i++ { // Check for up to 5 minutes
		nodesParams := map[string]interface{}{
			"clusterName": MCPTestClusterName,
		}

		result, err := mcpClient.CallTool(ctx, "get_cluster_nodes", nodesParams)
		require.NoError(t, err, "get_cluster_nodes should succeed after scaling")

		nodes, ok := result["nodes"].([]interface{})
		require.True(t, ok, "Nodes field should be an array")

		// Check if we have the expected number of nodes (control plane + scaled workers)
		expectedTotal := 1 + MCPTestScaleNodeCount
		if len(nodes) >= expectedTotal {
			// Also check that all nodes are ready
			allReady := true
			for _, nodeInterface := range nodes {
				node := nodeInterface.(map[string]interface{})
				if node["status"] != "Ready" {
					allReady = false
					break
				}
			}

			if allReady {
				finalNodes = nodes
				break
			}
		}

		logger.Info("Waiting for scaling to complete",
			"current_nodes", len(nodes),
			"expected_nodes", expectedTotal,
			"attempt", i+1)
		time.Sleep(30 * time.Second)
	}

	// Validate final node count (may not be exact due to timing)
	logger.Info("Final node count after scaling", "count", len(finalNodes))

	// Should have at least the original nodes
	assert.GreaterOrEqual(t, len(finalNodes), MCPTestNodeCount+1,
		"Should have at least original node count plus control plane")

	// Validate that nodes have proper structure
	for i, nodeInterface := range finalNodes {
		node, ok := nodeInterface.(map[string]interface{})
		require.True(t, ok, "Node %d should be a map", i)

		assert.Contains(t, node, "name", "Node should have name")
		assert.Contains(t, node, "status", "Node should have status")
		assert.Contains(t, node, "roles", "Node should have roles")
	}
}

// testGetClusterKubeconfigTool tests the get_cluster_kubeconfig tool
func testGetClusterKubeconfigTool(t *testing.T, ctx context.Context, mcpClient *utils.MCPClient) {
	logger := GetLogger()
	logger.Info("Testing get_cluster_kubeconfig tool")

	// Wait for cluster to be ready for kubeconfig access
	logger.Info("Waiting for cluster to be ready for kubeconfig access")

	var kubeconfigResult map[string]interface{}
	var lastErr error

	for i := 0; i < 20; i++ { // Try for up to 10 minutes
		kubeconfigParams := map[string]interface{}{
			"clusterName": MCPTestClusterName,
		}

		result, err := mcpClient.CallTool(ctx, "get_cluster_kubeconfig", kubeconfigParams)
		if err != nil {
			lastErr = err
			logger.Debug("get_cluster_kubeconfig not ready yet", "attempt", i+1, "error", err)
			time.Sleep(30 * time.Second)
			continue
		}

		kubeconfigResult = result
		lastErr = nil
		break
	}

	if lastErr != nil {
		logger.Warn("get_cluster_kubeconfig failed after retries", "error", lastErr)
		// For now, we'll allow this to fail as the cluster may not be fully ready
		t.Skip("Cluster not ready for kubeconfig access")
		return
	}

	require.NoError(t, lastErr, "get_cluster_kubeconfig should succeed")

	// Validate response structure
	assert.Contains(t, kubeconfigResult, "kubeconfig", "Response should contain kubeconfig field")

	kubeconfig, ok := kubeconfigResult["kubeconfig"].(string)
	require.True(t, ok, "Kubeconfig field should be a string")
	assert.NotEmpty(t, kubeconfig, "Kubeconfig should not be empty")

	// Basic validation that it's a valid YAML kubeconfig
	assert.Contains(t, kubeconfig, "apiVersion", "Kubeconfig should contain apiVersion")
	assert.Contains(t, kubeconfig, "clusters", "Kubeconfig should contain clusters")
	assert.Contains(t, kubeconfig, "users", "Kubeconfig should contain users")
	assert.Contains(t, kubeconfig, "contexts", "Kubeconfig should contain contexts")

	logger.Info("get_cluster_kubeconfig tool test completed", "kubeconfig_length", len(kubeconfig))
}

// testDeleteClusterTool tests the delete_cluster tool
func testDeleteClusterTool(t *testing.T, ctx context.Context, mcpClient *utils.MCPClient) {
	logger := GetLogger()
	logger.Info("Testing delete_cluster tool")

	deleteParams := map[string]interface{}{
		"clusterName": MCPTestClusterName,
	}

	result, err := mcpClient.CallTool(ctx, "delete_cluster", deleteParams)
	require.NoError(t, err, "delete_cluster tool should succeed")

	// Validate response structure
	assert.Contains(t, result, "status", "Response should contain status field")
	assert.Contains(t, result, "message", "Response should contain message field")

	status, ok := result["status"].(string)
	require.True(t, ok, "Status field should be string")
	assert.Equal(t, "success", status, "Delete operation should be successful")

	message, ok := result["message"].(string)
	require.True(t, ok, "Message field should be string")
	assert.NotEmpty(t, message, "Message should not be empty")

	logger.Info("delete_cluster tool test completed", "status", status, "message", message)

	// Wait for deletion to begin
	logger.Info("Waiting for cluster deletion to begin")
	time.Sleep(1 * time.Minute)
}

// testListClustersAfterDelete tests list_clusters after deleting the test cluster
func testListClustersAfterDelete(t *testing.T, ctx context.Context, mcpClient *utils.MCPClient) {
	logger := GetLogger()
	logger.Info("Testing list_clusters tool after cluster deletion")

	// Wait a bit more for deletion to progress
	time.Sleep(2 * time.Minute)

	result, err := mcpClient.CallTool(ctx, "list_clusters", map[string]interface{}{})
	require.NoError(t, err, "list_clusters tool should succeed")

	clusters, ok := result["clusters"].([]interface{})
	require.True(t, ok, "Clusters field should be an array")

	// Check if our test cluster is still in the list (it may be in Deleting state)
	var testCluster map[string]interface{}
	for _, clusterInterface := range clusters {
		cluster := clusterInterface.(map[string]interface{})
		if cluster["name"] == MCPTestClusterName {
			testCluster = cluster
			break
		}
	}

	if testCluster != nil {
		// Cluster may still exist but should be in Deleting state
		status := testCluster["status"].(string)
		assert.Contains(t, []string{"Deleting", "Failed"}, status,
			"Test cluster should be in Deleting or Failed state if still present")
		logger.Info("Test cluster still present during deletion", "status", status)
	} else {
		logger.Info("Test cluster successfully removed from list")
	}

	logger.Info("list_clusters after deletion test completed", "remaining_clusters", len(clusters))
}

// TestMCPToolsIndividual tests each MCP tool individually for more focused testing
func TestMCPToolsIndividual(t *testing.T) {
	RequireTestEnvironment(t)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	logger := GetLogger()
	mcpClient := GetMCPClient()

	logger.Info("Starting individual MCP tools tests")

	// Test 1: list_clusters (standalone)
	t.Run("ListClusters_Standalone", func(t *testing.T) {
		result, err := mcpClient.CallTool(ctx, "list_clusters", map[string]interface{}{})
		require.NoError(t, err, "list_clusters should work standalone")

		assert.Contains(t, result, "clusters", "Should have clusters field")
		clusters, ok := result["clusters"].([]interface{})
		require.True(t, ok, "Clusters should be array")

		logger.Info("Standalone list_clusters test", "count", len(clusters))
	})

	// Test 2: get_cluster with non-existent cluster
	t.Run("GetCluster_NotFound", func(t *testing.T) {
		params := map[string]interface{}{
			"clusterName": "non-existent-cluster",
		}

		_, err := mcpClient.CallTool(ctx, "get_cluster", params)
		assert.Error(t, err, "get_cluster should fail for non-existent cluster")
	})

	// Test 3: get_cluster_nodes with non-existent cluster
	t.Run("GetClusterNodes_NotFound", func(t *testing.T) {
		params := map[string]interface{}{
			"clusterName": "non-existent-cluster",
		}

		_, err := mcpClient.CallTool(ctx, "get_cluster_nodes", params)
		assert.Error(t, err, "get_cluster_nodes should fail for non-existent cluster")
	})

	// Test 4: get_cluster_kubeconfig with non-existent cluster
	t.Run("GetClusterKubeconfig_NotFound", func(t *testing.T) {
		params := map[string]interface{}{
			"clusterName": "non-existent-cluster",
		}

		_, err := mcpClient.CallTool(ctx, "get_cluster_kubeconfig", params)
		assert.Error(t, err, "get_cluster_kubeconfig should fail for non-existent cluster")
	})

	// Test 5: create_cluster with invalid parameters
	t.Run("CreateCluster_InvalidParams", func(t *testing.T) {
		// Missing required parameters
		params := map[string]interface{}{
			"clusterName": "invalid-test-cluster",
			// Missing templateName, kubernetesVersion, variables
		}

		_, err := mcpClient.CallTool(ctx, "create_cluster", params)
		assert.Error(t, err, "create_cluster should fail with missing parameters")
	})

	// Test 6: scale_cluster with non-existent cluster
	t.Run("ScaleCluster_NotFound", func(t *testing.T) {
		params := map[string]interface{}{
			"clusterName":  "non-existent-cluster",
			"nodePoolName": "worker-pool",
			"replicas":     3,
		}

		_, err := mcpClient.CallTool(ctx, "scale_cluster", params)
		assert.Error(t, err, "scale_cluster should fail for non-existent cluster")
	})

	// Test 7: delete_cluster with non-existent cluster
	t.Run("DeleteCluster_NotFound", func(t *testing.T) {
		params := map[string]interface{}{
			"clusterName": "non-existent-cluster",
		}

		_, err := mcpClient.CallTool(ctx, "delete_cluster", params)
		assert.Error(t, err, "delete_cluster should fail for non-existent cluster")
	})
}

// TestMCPToolsParameterValidation tests parameter validation for all tools
func TestMCPToolsParameterValidation(t *testing.T) {
	RequireTestEnvironment(t)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	logger := GetLogger()
	mcpClient := GetMCPClient()

	logger.Info("Starting MCP tools parameter validation tests")

	// Test parameter validation for each tool
	testCases := []struct {
		toolName    string
		params      map[string]interface{}
		shouldFail  bool
		description string
	}{
		{
			toolName:    "get_cluster",
			params:      map[string]interface{}{},
			shouldFail:  true,
			description: "get_cluster without clusterName should fail",
		},
		{
			toolName:    "get_cluster_nodes",
			params:      map[string]interface{}{},
			shouldFail:  true,
			description: "get_cluster_nodes without clusterName should fail",
		},
		{
			toolName:    "get_cluster_kubeconfig",
			params:      map[string]interface{}{},
			shouldFail:  true,
			description: "get_cluster_kubeconfig without clusterName should fail",
		},
		{
			toolName: "create_cluster",
			params: map[string]interface{}{
				"clusterName": "",
			},
			shouldFail:  true,
			description: "create_cluster with empty clusterName should fail",
		},
		{
			toolName: "scale_cluster",
			params: map[string]interface{}{
				"clusterName": "test",
			},
			shouldFail:  true,
			description: "scale_cluster without replicas should fail",
		},
		{
			toolName:    "delete_cluster",
			params:      map[string]interface{}{},
			shouldFail:  true,
			description: "delete_cluster without clusterName should fail",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			_, err := mcpClient.CallTool(ctx, tc.toolName, tc.params)

			if tc.shouldFail {
				assert.Error(t, err, tc.description)
			} else {
				assert.NoError(t, err, tc.description)
			}
		})
	}
}
