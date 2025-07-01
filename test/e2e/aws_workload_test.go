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
	// AWS workload cluster configuration
	WorkloadClusterName = "e2e-aws-cluster"
	WorkloadNodeCount   = 2
	WorkloadK8sVersion  = "v1.28.0"

	// Test instance types
	ControlPlaneInstanceType = "t3.medium"
	WorkerInstanceType       = "t3.small"

	// Test networking
	TestVPCCIDR    = "10.0.0.0/16"
	TestSubnetCIDR = "10.0.1.0/24"
)

// TestAWSWorkloadClusterLifecycle tests the complete lifecycle of an AWS workload cluster
func TestAWSWorkloadClusterLifecycle(t *testing.T) {
	RequireTestEnvironment(t)

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Minute)
	defer cancel()

	logger := GetLogger()
	mcpClient := GetMCPClient()
	awsUtil := utils.NewAWSUtil(logger)

	// Skip test if AWS credentials are not available
	if !awsUtil.HasCredentials() {
		t.Skip("AWS credentials not available, skipping AWS workload cluster test")
	}

	logger.Info("Starting AWS workload cluster lifecycle test")

	// Test phases
	t.Run("CreateWorkloadCluster", func(t *testing.T) {
		testCreateAWSWorkloadCluster(t, ctx, mcpClient, awsUtil)
	})

	t.Run("ValidateClusterProvisioning", func(t *testing.T) {
		testValidateAWSClusterProvisioning(t, ctx, mcpClient, awsUtil)
	})

	t.Run("TestClusterConnectivity", func(t *testing.T) {
		testAWSClusterConnectivity(t, ctx, mcpClient, awsUtil)
	})

	t.Run("ScaleClusterNodes", func(t *testing.T) {
		testScaleAWSClusterNodes(t, ctx, mcpClient, awsUtil)
	})

	t.Run("DeleteWorkloadCluster", func(t *testing.T) {
		testDeleteAWSWorkloadCluster(t, ctx, mcpClient, awsUtil)
	})

	t.Run("ValidateAWSCleanup", func(t *testing.T) {
		testValidateAWSResourceCleanup(t, ctx, awsUtil)
	})
}

// testCreateAWSWorkloadCluster tests creating an AWS workload cluster
func testCreateAWSWorkloadCluster(t *testing.T, ctx context.Context, mcpClient *utils.MCPClient, awsUtil *utils.AWSUtil) {
	logger := GetLogger()
	logger.Info("Testing AWS workload cluster creation")

	// Prepare cluster creation parameters
	clusterVars := map[string]interface{}{
		"region":                   "us-west-2",
		"nodeCount":                WorkloadNodeCount,
		"controlPlaneInstanceType": ControlPlaneInstanceType,
		"workerInstanceType":       WorkerInstanceType,
		"vpcCIDR":                  TestVPCCIDR,
		"subnetCIDR":               TestSubnetCIDR,
	}

	// Add SSH key if available
	if sshKeyName := awsUtil.GetSSHKeyName(); sshKeyName != "" {
		clusterVars["sshKeyName"] = sshKeyName
	}

	// Create cluster using MCP
	createParams := map[string]interface{}{
		"clusterName":       WorkloadClusterName,
		"templateName":      "aws-cluster-template", // Assuming this exists in test environment
		"kubernetesVersion": WorkloadK8sVersion,
		"variables":         clusterVars,
	}

	logger.Info("Calling create_cluster MCP tool", "params", createParams)

	result, err := mcpClient.CallTool(ctx, "create_cluster", createParams)
	require.NoError(t, err, "create_cluster tool should succeed")

	// Validate response
	assert.Contains(t, result, "success", "Response should contain success field")
	assert.Contains(t, result, "cluster", "Response should contain cluster field")

	success, ok := result["success"].(bool)
	require.True(t, ok, "Success field should be boolean")
	assert.True(t, success, "Cluster creation should be successful")

	logger.Info("AWS workload cluster creation initiated successfully")
}

// testValidateAWSClusterProvisioning validates that AWS resources are being created
func testValidateAWSClusterProvisioning(t *testing.T, ctx context.Context, mcpClient *utils.MCPClient, awsUtil *utils.AWSUtil) {
	logger := GetLogger()
	logger.Info("Validating AWS cluster provisioning")

	// Wait for cluster to reach Provisioning phase
	clusterUtil := GetClusterUtil()

	err := clusterUtil.WaitForClusterPhase(ctx, WorkloadClusterName, TestNamespace, "Provisioning", 5*time.Minute)
	require.NoError(t, err, "Cluster should reach Provisioning phase")

	// Validate AWS infrastructure is being created

	// Check for VPC creation
	logger.Info("Checking for VPC creation")
	vpcs, err := awsUtil.ListVPCs(ctx, WorkloadClusterName)
	require.NoError(t, err, "Should be able to list VPCs")
	assert.NotEmpty(t, vpcs, "VPC should be created for cluster")

	// Check for security groups
	logger.Info("Checking for security group creation")
	securityGroups, err := awsUtil.ListSecurityGroups(ctx, WorkloadClusterName)
	require.NoError(t, err, "Should be able to list security groups")
	assert.NotEmpty(t, securityGroups, "Security groups should be created")

	// Check for EC2 instances (may not be ready yet, but should be launching)
	logger.Info("Checking for EC2 instance creation")
	instances, err := awsUtil.ListEC2Instances(ctx, WorkloadClusterName)
	require.NoError(t, err, "Should be able to list EC2 instances")

	// We expect at least the control plane instance
	assert.NotEmpty(t, instances, "At least control plane instance should be launching")

	logger.Info("AWS infrastructure provisioning validated",
		"vpcs", len(vpcs),
		"security_groups", len(securityGroups),
		"instances", len(instances),
	)
}

// testAWSClusterConnectivity tests that the cluster becomes accessible
func testAWSClusterConnectivity(t *testing.T, ctx context.Context, mcpClient *utils.MCPClient, awsUtil *utils.AWSUtil) {
	logger := GetLogger()
	logger.Info("Testing AWS cluster connectivity")

	// Wait for cluster to become ready (this may take 10-15 minutes)
	clusterUtil := GetClusterUtil()

	logger.Info("Waiting for cluster to become ready (this may take up to 15 minutes)")
	err := clusterUtil.WaitForClusterPhase(ctx, WorkloadClusterName, TestNamespace, "Provisioned", ClusterCreateTimeout)
	require.NoError(t, err, "Cluster should become ready within timeout")

	// Get cluster details using MCP
	getParams := map[string]interface{}{
		"clusterName": WorkloadClusterName,
	}

	result, err := mcpClient.CallTool(ctx, "get_cluster", getParams)
	require.NoError(t, err, "get_cluster tool should succeed")

	// Validate cluster details
	assert.Contains(t, result, "cluster", "Response should contain cluster details")

	clusterInfo, ok := result["cluster"].(map[string]interface{})
	require.True(t, ok, "Cluster field should be a map")

	assert.Equal(t, WorkloadClusterName, clusterInfo["name"], "Cluster name should match")
	assert.Equal(t, "Ready", clusterInfo["status"], "Cluster should be ready")

	// Test kubeconfig retrieval
	kubeconfigParams := map[string]interface{}{
		"clusterName": WorkloadClusterName,
	}

	kubeconfigResult, err := mcpClient.CallTool(ctx, "get_cluster_kubeconfig", kubeconfigParams)
	require.NoError(t, err, "get_cluster_kubeconfig tool should succeed")

	assert.Contains(t, kubeconfigResult, "kubeconfig", "Response should contain kubeconfig")

	kubeconfig, ok := kubeconfigResult["kubeconfig"].(string)
	require.True(t, ok, "Kubeconfig should be a string")
	assert.NotEmpty(t, kubeconfig, "Kubeconfig should not be empty")
	assert.Contains(t, kubeconfig, "apiVersion", "Kubeconfig should be valid YAML")

	// Test node listing
	nodesParams := map[string]interface{}{
		"clusterName": WorkloadClusterName,
	}

	nodesResult, err := mcpClient.CallTool(ctx, "get_cluster_nodes", nodesParams)
	require.NoError(t, err, "get_cluster_nodes tool should succeed")

	assert.Contains(t, nodesResult, "nodes", "Response should contain nodes")

	nodes, ok := nodesResult["nodes"].([]interface{})
	require.True(t, ok, "Nodes should be an array")

	// We expect control plane + worker nodes
	expectedNodeCount := 1 + WorkloadNodeCount
	assert.Len(t, nodes, expectedNodeCount, "Should have correct number of nodes")

	// Validate node properties
	for i, nodeInterface := range nodes {
		node, ok := nodeInterface.(map[string]interface{})
		require.True(t, ok, "Node %d should be a map", i)

		assert.Contains(t, node, "name", "Node should have name")
		assert.Contains(t, node, "status", "Node should have status")
		assert.Contains(t, node, "roles", "Node should have roles")

		status, ok := node["status"].(string)
		require.True(t, ok, "Node status should be string")
		assert.Equal(t, "Ready", status, "Node should be ready")
	}

	logger.Info("AWS cluster connectivity validated successfully",
		"cluster_status", clusterInfo["status"],
		"node_count", len(nodes),
	)
}

// testScaleAWSClusterNodes tests scaling the AWS cluster
func testScaleAWSClusterNodes(t *testing.T, ctx context.Context, mcpClient *utils.MCPClient, awsUtil *utils.AWSUtil) {
	logger := GetLogger()
	logger.Info("Testing AWS cluster node scaling")

	// Scale up to 3 worker nodes
	newNodeCount := 3
	scaleParams := map[string]interface{}{
		"clusterName":  WorkloadClusterName,
		"nodePoolName": "worker-pool", // Assuming this is the default worker pool name
		"replicas":     newNodeCount,
	}

	result, err := mcpClient.CallTool(ctx, "scale_cluster", scaleParams)
	require.NoError(t, err, "scale_cluster tool should succeed")

	assert.Contains(t, result, "status", "Response should contain status")
	assert.Contains(t, result, "message", "Response should contain message")

	// Wait for scaling to complete
	logger.Info("Waiting for cluster scaling to complete")
	time.Sleep(2 * time.Minute) // Give initial time for scaling to start

	// Validate that new nodes are being created
	for i := 0; i < 10; i++ { // Check for up to 5 minutes
		nodesParams := map[string]interface{}{
			"clusterName": WorkloadClusterName,
		}

		nodesResult, err := mcpClient.CallTool(ctx, "get_cluster_nodes", nodesParams)
		require.NoError(t, err, "get_cluster_nodes should succeed during scaling")

		nodes, ok := nodesResult["nodes"].([]interface{})
		require.True(t, ok, "Nodes should be an array")

		expectedTotalNodes := 1 + newNodeCount // control plane + workers
		if len(nodes) == expectedTotalNodes {
			// Check that all nodes are ready
			allReady := true
			for _, nodeInterface := range nodes {
				node := nodeInterface.(map[string]interface{})
				if node["status"] != "Ready" {
					allReady = false
					break
				}
			}

			if allReady {
				logger.Info("Cluster scaling completed successfully", "total_nodes", len(nodes))
				break
			}
		}

		logger.Info("Waiting for scaling to complete", "current_nodes", len(nodes), "expected_nodes", expectedTotalNodes)
		time.Sleep(30 * time.Second)
	}

	// Final validation
	nodesParams := map[string]interface{}{
		"clusterName": WorkloadClusterName,
	}

	finalNodesResult, err := mcpClient.CallTool(ctx, "get_cluster_nodes", nodesParams)
	require.NoError(t, err, "Final get_cluster_nodes should succeed")

	finalNodes, ok := finalNodesResult["nodes"].([]interface{})
	require.True(t, ok, "Final nodes should be an array")

	expectedTotalNodes := 1 + newNodeCount
	assert.Len(t, finalNodes, expectedTotalNodes, "Should have scaled to correct number of nodes")

	logger.Info("AWS cluster scaling test completed successfully")
}

// testDeleteAWSWorkloadCluster tests deleting the AWS workload cluster
func testDeleteAWSWorkloadCluster(t *testing.T, ctx context.Context, mcpClient *utils.MCPClient, awsUtil *utils.AWSUtil) {
	logger := GetLogger()
	logger.Info("Testing AWS workload cluster deletion")

	// Delete cluster using MCP
	deleteParams := map[string]interface{}{
		"clusterName": WorkloadClusterName,
	}

	result, err := mcpClient.CallTool(ctx, "delete_cluster", deleteParams)
	require.NoError(t, err, "delete_cluster tool should succeed")

	assert.Contains(t, result, "status", "Response should contain status")
	assert.Contains(t, result, "message", "Response should contain message")

	// Wait for cluster deletion to complete
	logger.Info("Waiting for cluster deletion to complete")
	clusterUtil := GetClusterUtil()

	err = clusterUtil.WaitForClusterDeletion(ctx, WorkloadClusterName, TestNamespace, ClusterDeleteTimeout)
	require.NoError(t, err, "Cluster should be deleted within timeout")

	logger.Info("AWS workload cluster deletion completed successfully")
}

// testValidateAWSResourceCleanup validates that AWS resources are properly cleaned up
func testValidateAWSResourceCleanup(t *testing.T, ctx context.Context, awsUtil *utils.AWSUtil) {
	logger := GetLogger()
	logger.Info("Validating AWS resource cleanup")

	// Give AWS some time to clean up resources
	logger.Info("Waiting for AWS resource cleanup to complete")
	time.Sleep(2 * time.Minute)

	// Check that VPCs are cleaned up
	vpcs, err := awsUtil.ListVPCs(ctx, WorkloadClusterName)
	require.NoError(t, err, "Should be able to list VPCs")

	// Filter out default VPCs
	clusterVPCs := awsUtil.FilterClusterVPCs(vpcs, WorkloadClusterName)
	assert.Empty(t, clusterVPCs, "Cluster VPCs should be cleaned up")

	// Check that security groups are cleaned up
	securityGroups, err := awsUtil.ListSecurityGroups(ctx, WorkloadClusterName)
	require.NoError(t, err, "Should be able to list security groups")

	clusterSecurityGroups := awsUtil.FilterClusterSecurityGroups(securityGroups, WorkloadClusterName)
	assert.Empty(t, clusterSecurityGroups, "Cluster security groups should be cleaned up")

	// Check that EC2 instances are terminated
	instances, err := awsUtil.ListEC2Instances(ctx, WorkloadClusterName)
	require.NoError(t, err, "Should be able to list EC2 instances")

	runningInstances := awsUtil.FilterRunningInstances(instances)
	assert.Empty(t, runningInstances, "All cluster instances should be terminated")

	// Check for load balancers (if any were created)
	loadBalancers, err := awsUtil.ListLoadBalancers(ctx, WorkloadClusterName)
	require.NoError(t, err, "Should be able to list load balancers")
	assert.Empty(t, loadBalancers, "Load balancers should be cleaned up")

	logger.Info("AWS resource cleanup validation completed successfully")
}

// TestAWSProviderValidation tests AWS-specific provider validation
func TestAWSProviderValidation(t *testing.T) {
	RequireTestEnvironment(t)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	logger := GetLogger()
	mcpClient := GetMCPClient()
	awsUtil := utils.NewAWSUtil(logger)

	if !awsUtil.HasCredentials() {
		t.Skip("AWS credentials not available, skipping AWS provider validation test")
	}

	logger.Info("Testing AWS provider validation")

	t.Run("ValidRegion", func(t *testing.T) {
		testAWSValidRegion(t, ctx, mcpClient)
	})

	t.Run("InvalidRegion", func(t *testing.T) {
		testAWSInvalidRegion(t, ctx, mcpClient)
	})

	t.Run("ValidInstanceType", func(t *testing.T) {
		testAWSValidInstanceType(t, ctx, mcpClient)
	})

	t.Run("InvalidInstanceType", func(t *testing.T) {
		testAWSInvalidInstanceType(t, ctx, mcpClient)
	})

	t.Run("InvalidNodeCount", func(t *testing.T) {
		testAWSInvalidNodeCount(t, ctx, mcpClient)
	})
}

// testAWSValidRegion tests cluster creation with valid AWS region
func testAWSValidRegion(t *testing.T, ctx context.Context, mcpClient *utils.MCPClient) {
	clusterName := "test-valid-region"
	defer cleanupTestCluster(t, ctx, mcpClient, clusterName)

	clusterVars := map[string]interface{}{
		"region":       "us-west-2", // Valid region
		"nodeCount":    1,
		"instanceType": "t3.small",
	}

	createParams := map[string]interface{}{
		"clusterName":       clusterName,
		"templateName":      "aws-cluster-template",
		"kubernetesVersion": "v1.28.0",
		"variables":         clusterVars,
	}

	result, err := mcpClient.CallTool(ctx, "create_cluster", createParams)
	require.NoError(t, err, "Valid region should be accepted")

	success, ok := result["success"].(bool)
	require.True(t, ok, "Success field should be boolean")
	assert.True(t, success, "Cluster creation should succeed with valid region")
}

// testAWSInvalidRegion tests cluster creation with invalid AWS region
func testAWSInvalidRegion(t *testing.T, ctx context.Context, mcpClient *utils.MCPClient) {
	clusterVars := map[string]interface{}{
		"region":       "invalid-region", // Invalid region
		"nodeCount":    1,
		"instanceType": "t3.small",
	}

	createParams := map[string]interface{}{
		"clusterName":       "test-invalid-region",
		"templateName":      "aws-cluster-template",
		"kubernetesVersion": "v1.28.0",
		"variables":         clusterVars,
	}

	_, err := mcpClient.CallTool(ctx, "create_cluster", createParams)
	assert.Error(t, err, "Invalid region should be rejected")
	assert.Contains(t, err.Error(), "region", "Error should mention region")
}

// testAWSValidInstanceType tests cluster creation with valid instance type
func testAWSValidInstanceType(t *testing.T, ctx context.Context, mcpClient *utils.MCPClient) {
	clusterName := "test-valid-instance"
	defer cleanupTestCluster(t, ctx, mcpClient, clusterName)

	clusterVars := map[string]interface{}{
		"region":       "us-west-2",
		"nodeCount":    1,
		"instanceType": "t3.medium", // Valid instance type
	}

	createParams := map[string]interface{}{
		"clusterName":       clusterName,
		"templateName":      "aws-cluster-template",
		"kubernetesVersion": "v1.28.0",
		"variables":         clusterVars,
	}

	result, err := mcpClient.CallTool(ctx, "create_cluster", createParams)
	require.NoError(t, err, "Valid instance type should be accepted")

	success, ok := result["success"].(bool)
	require.True(t, ok, "Success field should be boolean")
	assert.True(t, success, "Cluster creation should succeed with valid instance type")
}

// testAWSInvalidInstanceType tests cluster creation with invalid instance type
func testAWSInvalidInstanceType(t *testing.T, ctx context.Context, mcpClient *utils.MCPClient) {
	clusterVars := map[string]interface{}{
		"region":       "us-west-2",
		"nodeCount":    1,
		"instanceType": "invalid.instance.type", // Invalid instance type
	}

	createParams := map[string]interface{}{
		"clusterName":       "test-invalid-instance",
		"templateName":      "aws-cluster-template",
		"kubernetesVersion": "v1.28.0",
		"variables":         clusterVars,
	}

	_, err := mcpClient.CallTool(ctx, "create_cluster", createParams)
	assert.Error(t, err, "Invalid instance type should be rejected")
	assert.Contains(t, err.Error(), "instance", "Error should mention instance type")
}

// testAWSInvalidNodeCount tests cluster creation with invalid node count
func testAWSInvalidNodeCount(t *testing.T, ctx context.Context, mcpClient *utils.MCPClient) {
	clusterVars := map[string]interface{}{
		"region":       "us-west-2",
		"nodeCount":    -1, // Invalid node count
		"instanceType": "t3.small",
	}

	createParams := map[string]interface{}{
		"clusterName":       "test-invalid-nodecount",
		"templateName":      "aws-cluster-template",
		"kubernetesVersion": "v1.28.0",
		"variables":         clusterVars,
	}

	_, err := mcpClient.CallTool(ctx, "create_cluster", createParams)
	assert.Error(t, err, "Invalid node count should be rejected")
	assert.Contains(t, err.Error(), "node", "Error should mention node count")
}

// cleanupTestCluster cleans up a test cluster if it was created
func cleanupTestCluster(t *testing.T, ctx context.Context, mcpClient *utils.MCPClient, clusterName string) {
	deleteParams := map[string]interface{}{
		"clusterName": clusterName,
	}

	// Attempt to delete, but don't fail the test if it doesn't exist
	_, err := mcpClient.CallTool(ctx, "delete_cluster", deleteParams)
	if err != nil {
		t.Logf("Failed to cleanup test cluster %s: %v", clusterName, err)
	}
}
