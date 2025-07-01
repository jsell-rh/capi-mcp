// +build e2e

package e2e

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/capi-mcp/capi-mcp-server/test/e2e/utils"
)

const (
	// Stability test configuration
	StabilityClusterName   = "e2e-stability-test"
	StabilityNodeCount     = 1
	StabilityK8sVersion    = "v1.28.0"
	StabilityInstanceType  = "t3.small"
	
	// Stability test timeouts
	StabilityCreateTimeout = 20 * time.Minute
	StabilityDeleteTimeout = 15 * time.Minute
	StabilityRetryCount    = 3
	StabilityRetryDelay    = 30 * time.Second
)

// TestE2EStability tests the stability and reliability of the E2E test suite
func TestE2EStability(t *testing.T) {
	RequireTestEnvironment(t)
	
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Minute)
	defer cancel()
	
	logger := GetLogger()
	mcpClient := GetMCPClient()
	awsUtil := utils.NewAWSUtil(logger)
	
	// Skip test if AWS credentials are not available
	if !awsUtil.HasCredentials() {
		t.Skip("AWS credentials not available, skipping stability test")
	}
	
	logger.Info("Starting E2E stability test")
	
	t.Run("TestEnvironmentHealth", func(t *testing.T) {
		testEnvironmentHealth(t, ctx, mcpClient, awsUtil)
	})
	
	t.Run("TestAWSConnectivity", func(t *testing.T) {
		testAWSConnectivity(t, ctx, awsUtil)
	})
	
	t.Run("TestMCPServerResponsiveness", func(t *testing.T) {
		testMCPServerResponsiveness(t, ctx, mcpClient)
	})
	
	t.Run("TestClusterOperationStability", func(t *testing.T) {
		testClusterOperationStability(t, ctx, mcpClient, awsUtil)
	})
	
	t.Run("TestErrorRecovery", func(t *testing.T) {
		testErrorRecovery(t, ctx, mcpClient)
	})
	
	t.Run("TestPerformanceBaseline", func(t *testing.T) {
		testPerformanceBaseline(t, ctx, mcpClient)
	})
}

// testEnvironmentHealth validates the test environment is healthy
func testEnvironmentHealth(t *testing.T, ctx context.Context, mcpClient *utils.MCPClient, awsUtil *utils.AWSUtil) {
	logger := GetLogger()
	logger.Info("Testing environment health")
	
	// Test 1: Kubernetes cluster health
	kubeClient := GetKubeClient()
	require.NotNil(t, kubeClient, "Kubernetes client should be available")
	
	// Test 2: CAPI controllers health
	clusterUtil := GetClusterUtil()
	require.NotNil(t, clusterUtil, "Cluster utility should be available")
	
	// Test 3: MCP server health
	require.NotNil(t, mcpClient, "MCP client should be available")
	
	// Test 4: AWS utilities health
	require.NotNil(t, awsUtil, "AWS utility should be available")
	assert.True(t, awsUtil.HasCredentials(), "AWS credentials should be available")
	
	// Test 5: Test namespace access
	clusters, err := clusterUtil.ListClusters(ctx, TestNamespace)
	require.NoError(t, err, "Should be able to list clusters in test namespace")
	logger.Info("Environment health check passed", "existing_clusters", len(clusters))
}

// testAWSConnectivity validates AWS connectivity and permissions
func testAWSConnectivity(t *testing.T, ctx context.Context, awsUtil *utils.AWSUtil) {
	logger := GetLogger()
	logger.Info("Testing AWS connectivity")
	
	// Initialize AWS clients
	err := awsUtil.Initialize(ctx)
	require.NoError(t, err, "AWS clients should initialize successfully")
	
	// Test region access
	region := awsUtil.GetRegion()
	assert.NotEmpty(t, region, "AWS region should be configured")
	logger.Info("AWS region configured", "region", region)
	
	// Test VPC listing (should not fail even if empty)
	vpcs, err := awsUtil.ListVPCs(ctx, "test-connectivity")
	require.NoError(t, err, "Should be able to list VPCs")
	logger.Info("VPC list test passed", "vpcs_found", len(vpcs))
	
	// Test security group listing
	sgs, err := awsUtil.ListSecurityGroups(ctx, "test-connectivity")
	require.NoError(t, err, "Should be able to list security groups")
	logger.Info("Security group list test passed", "sgs_found", len(sgs))
	
	// Test EC2 instance listing
	instances, err := awsUtil.ListEC2Instances(ctx, "test-connectivity")
	require.NoError(t, err, "Should be able to list EC2 instances")
	logger.Info("EC2 instance list test passed", "instances_found", len(instances))
	
	logger.Info("AWS connectivity test passed")
}

// testMCPServerResponsiveness tests MCP server responsiveness under load
func testMCPServerResponsiveness(t *testing.T, ctx context.Context, mcpClient *utils.MCPClient) {
	logger := GetLogger()
	logger.Info("Testing MCP server responsiveness")
	
	// Test rapid successive calls
	start := time.Now()
	for i := 0; i < 10; i++ {
		_, err := mcpClient.CallTool(ctx, "list_clusters", map[string]interface{}{})
		require.NoError(t, err, "list_clusters should succeed in rapid succession")
	}
	duration := time.Since(start)
	
	// Should complete within reasonable time (< 5 seconds for 10 calls)
	assert.Less(t, duration, 5*time.Second, "Rapid MCP calls should complete quickly")
	logger.Info("Rapid call test passed", "duration", duration, "calls", 10)
	
	// Test concurrent calls
	start = time.Now()
	errors := make(chan error, 5)
	
	for i := 0; i < 5; i++ {
		go func() {
			_, err := mcpClient.CallTool(ctx, "list_clusters", map[string]interface{}{})
			errors <- err
		}()
	}
	
	// Collect results
	for i := 0; i < 5; i++ {
		err := <-errors
		assert.NoError(t, err, "Concurrent MCP calls should succeed")
	}
	
	concurrentDuration := time.Since(start)
	assert.Less(t, concurrentDuration, 10*time.Second, "Concurrent MCP calls should complete quickly")
	logger.Info("Concurrent call test passed", "duration", concurrentDuration, "concurrent_calls", 5)
}

// testClusterOperationStability tests stability of cluster operations with retries
func testClusterOperationStability(t *testing.T, ctx context.Context, mcpClient *utils.MCPClient, awsUtil *utils.AWSUtil) {
	logger := GetLogger()
	logger.Info("Testing cluster operation stability")
	
	// Ensure cleanup of any existing test cluster
	cleanupStabilityCluster(t, ctx, mcpClient, awsUtil)
	
	// Test cluster creation with retries
	createSuccess := false
	var createErr error
	
	for attempt := 1; attempt <= StabilityRetryCount; attempt++ {
		logger.Info("Attempting cluster creation", "attempt", attempt)
		
		clusterVars := map[string]interface{}{
			"region":                   awsUtil.GetRegion(),
			"nodeCount":                StabilityNodeCount,
			"controlPlaneInstanceType": "t3.medium",
			"workerInstanceType":       StabilityInstanceType,
			"vpcCIDR":                 "10.0.0.0/16",
			"subnetCIDR":              "10.0.1.0/24",
		}
		
		// Add SSH key if available
		if sshKeyName := awsUtil.GetSSHKeyName(); sshKeyName != "" {
			clusterVars["sshKeyName"] = sshKeyName
		}
		
		createParams := map[string]interface{}{
			"clusterName":       StabilityClusterName,
			"templateName":      "aws-cluster-template",
			"kubernetesVersion": StabilityK8sVersion,
			"variables":         clusterVars,
		}
		
		result, err := mcpClient.CallTool(ctx, "create_cluster", createParams)
		if err != nil {
			createErr = err
			logger.Warn("Cluster creation attempt failed", "attempt", attempt, "error", err)
			
			// Cleanup any partial resources
			cleanupStabilityCluster(t, ctx, mcpClient, awsUtil)
			
			if attempt < StabilityRetryCount {
				time.Sleep(StabilityRetryDelay)
				continue
			}
			break
		}
		
		// Validate success response
		success, ok := result["success"].(bool)
		if !ok || !success {
			createErr = fmt.Errorf("cluster creation returned unsuccessful result: %v", result)
			logger.Warn("Cluster creation unsuccessful", "attempt", attempt, "result", result)
			
			cleanupStabilityCluster(t, ctx, mcpClient, awsUtil)
			
			if attempt < StabilityRetryCount {
				time.Sleep(StabilityRetryDelay)
				continue
			}
			break
		}
		
		createSuccess = true
		logger.Info("Cluster creation succeeded", "attempt", attempt)
		break
	}
	
	require.True(t, createSuccess, "Cluster creation should succeed within retry limit: %v", createErr)
	
	// Wait for cluster to reach a stable state
	logger.Info("Waiting for cluster to reach stable state")
	
	clusterUtil := GetClusterUtil()
	stablePhaseReached := false
	
	// Wait for cluster to reach Provisioning or Provisioned state
	for i := 0; i < 10; i++ { // Wait up to 5 minutes
		cluster, err := clusterUtil.GetCluster(ctx, StabilityClusterName, TestNamespace)
		if err != nil {
			logger.Debug("Cluster not found yet", "attempt", i+1, "error", err)
			time.Sleep(30 * time.Second)
			continue
		}
		
		logger.Info("Cluster status check", "phase", cluster.Phase, "attempt", i+1)
		
		if cluster.Phase == "Provisioning" || cluster.Phase == "Provisioned" {
			stablePhaseReached = true
			break
		}
		
		if cluster.Phase == "Failed" {
			t.Fatalf("Cluster reached Failed state: %s", cluster.Phase)
		}
		
		time.Sleep(30 * time.Second)
	}
	
	assert.True(t, stablePhaseReached, "Cluster should reach stable phase within timeout")
	
	// Test cluster deletion with retries
	deleteSuccess := false
	var deleteErr error
	
	for attempt := 1; attempt <= StabilityRetryCount; attempt++ {
		logger.Info("Attempting cluster deletion", "attempt", attempt)
		
		deleteParams := map[string]interface{}{
			"clusterName": StabilityClusterName,
		}
		
		result, err := mcpClient.CallTool(ctx, "delete_cluster", deleteParams)
		if err != nil {
			deleteErr = err
			logger.Warn("Cluster deletion attempt failed", "attempt", attempt, "error", err)
			
			if attempt < StabilityRetryCount {
				time.Sleep(StabilityRetryDelay)
				continue
			}
			break
		}
		
		// Validate success response
		status, ok := result["status"].(string)
		if !ok || status != "success" {
			deleteErr = fmt.Errorf("cluster deletion returned unsuccessful result: %v", result)
			logger.Warn("Cluster deletion unsuccessful", "attempt", attempt, "result", result)
			
			if attempt < StabilityRetryCount {
				time.Sleep(StabilityRetryDelay)
				continue
			}
			break
		}
		
		deleteSuccess = true
		logger.Info("Cluster deletion succeeded", "attempt", attempt)
		break
	}
	
	require.True(t, deleteSuccess, "Cluster deletion should succeed within retry limit: %v", deleteErr)
	
	// Wait for deletion to begin
	time.Sleep(1 * time.Minute)
	
	logger.Info("Cluster operation stability test completed successfully")
}

// testErrorRecovery tests error handling and recovery scenarios
func testErrorRecovery(t *testing.T, ctx context.Context, mcpClient *utils.MCPClient) {
	logger := GetLogger()
	logger.Info("Testing error recovery scenarios")
	
	// Test 1: Invalid tool parameters recovery
	invalidParams := map[string]interface{}{
		"clusterName": "",
	}
	
	_, err := mcpClient.CallTool(ctx, "get_cluster", invalidParams)
	assert.Error(t, err, "Should handle invalid parameters gracefully")
	
	// Test 2: Server should still respond after error
	validResult, err := mcpClient.CallTool(ctx, "list_clusters", map[string]interface{}{})
	require.NoError(t, err, "Server should recover from previous error")
	assert.Contains(t, validResult, "clusters", "Should return valid response after error")
	
	// Test 3: Non-existent cluster handling
	nonExistentParams := map[string]interface{}{
		"clusterName": "definitely-does-not-exist-cluster",
	}
	
	_, err = mcpClient.CallTool(ctx, "get_cluster", nonExistentParams)
	assert.Error(t, err, "Should handle non-existent cluster gracefully")
	
	// Test 4: Server should still respond after non-existent cluster
	validResult2, err := mcpClient.CallTool(ctx, "list_clusters", map[string]interface{}{})
	require.NoError(t, err, "Server should work after non-existent cluster error")
	assert.Contains(t, validResult2, "clusters", "Should return valid response after error")
	
	logger.Info("Error recovery test completed successfully")
}

// testPerformanceBaseline establishes performance baselines for operations
func testPerformanceBaseline(t *testing.T, ctx context.Context, mcpClient *utils.MCPClient) {
	logger := GetLogger()
	logger.Info("Testing performance baseline")
	
	// Test 1: list_clusters performance
	start := time.Now()
	_, err := mcpClient.CallTool(ctx, "list_clusters", map[string]interface{}{})
	listDuration := time.Since(start)
	
	require.NoError(t, err, "list_clusters should succeed")
	assert.Less(t, listDuration, 500*time.Millisecond, "list_clusters should complete within 500ms")
	logger.Info("list_clusters performance", "duration", listDuration)
	
	// Test 2: get_cluster performance (with non-existent cluster for quick response)
	start = time.Now()
	_, _ = mcpClient.CallTool(ctx, "get_cluster", map[string]interface{}{
		"clusterName": "perf-test-cluster",
	})
	getDuration := time.Since(start)
	
	assert.Less(t, getDuration, 500*time.Millisecond, "get_cluster should respond within 500ms")
	logger.Info("get_cluster performance", "duration", getDuration)
	
	// Test 3: Multiple rapid calls performance
	start = time.Now()
	callCount := 20
	for i := 0; i < callCount; i++ {
		_, err := mcpClient.CallTool(ctx, "list_clusters", map[string]interface{}{})
		require.NoError(t, err, "Rapid calls should all succeed")
	}
	rapidDuration := time.Since(start)
	
	avgDuration := rapidDuration / time.Duration(callCount)
	assert.Less(t, avgDuration, 200*time.Millisecond, "Average call duration should be < 200ms")
	logger.Info("Rapid calls performance", 
		"total_duration", rapidDuration,
		"call_count", callCount,
		"avg_duration", avgDuration)
	
	logger.Info("Performance baseline test completed successfully")
}

// cleanupStabilityCluster ensures the stability test cluster is cleaned up
func cleanupStabilityCluster(t *testing.T, ctx context.Context, mcpClient *utils.MCPClient, awsUtil *utils.AWSUtil) {
	logger := GetLogger()
	logger.Info("Cleaning up stability test cluster", "cluster", StabilityClusterName)
	
	// Try to delete cluster via MCP
	deleteParams := map[string]interface{}{
		"clusterName": StabilityClusterName,
	}
	
	_, err := mcpClient.CallTool(ctx, "delete_cluster", deleteParams)
	if err != nil {
		logger.Debug("Failed to delete cluster via MCP (may not exist)", "error", err)
	}
	
	// Wait for deletion to begin
	time.Sleep(30 * time.Second)
	
	// Verify cluster deletion from Kubernetes
	clusterUtil := GetClusterUtil()
	err = clusterUtil.WaitForClusterDeletion(ctx, StabilityClusterName, TestNamespace, 2*time.Minute)
	if err != nil {
		logger.Warn("Cluster deletion verification failed", "error", err)
	}
	
	// Cleanup any remaining AWS resources
	err = awsUtil.CleanupClusterResources(ctx, StabilityClusterName)
	if err != nil {
		logger.Warn("AWS resource cleanup failed", "error", err)
	}
	
	logger.Info("Stability test cluster cleanup completed")
}

// TestE2EEnvironmentValidation validates the complete E2E environment
func TestE2EEnvironmentValidation(t *testing.T) {
	RequireTestEnvironment(t)
	
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	
	logger := GetLogger()
	mcpClient := GetMCPClient()
	awsUtil := utils.NewAWSUtil(logger)
	
	logger.Info("Starting E2E environment validation")
	
	t.Run("ValidateKubernetesAccess", func(t *testing.T) {
		kubeClient := GetKubeClient()
		require.NotNil(t, kubeClient, "Kubernetes client must be available")
		
		clusterUtil := GetClusterUtil()
		clusters, err := clusterUtil.ListClusters(ctx, TestNamespace)
		require.NoError(t, err, "Should be able to list clusters")
		
		logger.Info("Kubernetes access validated", "clusters", len(clusters))
	})
	
	t.Run("ValidateCAPIInstallation", func(t *testing.T) {
		clusterUtil := GetClusterUtil()
		clusterClasses, err := clusterUtil.GetClusterClasses(ctx, TestNamespace)
		require.NoError(t, err, "Should be able to list ClusterClasses")
		
		// Should have at least the test ClusterClass
		assert.NotEmpty(t, clusterClasses, "Should have ClusterClasses available")
		logger.Info("CAPI installation validated", "cluster_classes", len(clusterClasses))
	})
	
	t.Run("ValidateMCPServerDeployment", func(t *testing.T) {
		require.NotNil(t, mcpClient, "MCP client must be available")
		
		// Test basic connectivity
		result, err := mcpClient.CallTool(ctx, "list_clusters", map[string]interface{}{})
		require.NoError(t, err, "MCP server should be responsive")
		assert.Contains(t, result, "clusters", "MCP response should be valid")
		
		logger.Info("MCP server deployment validated")
	})
	
	t.Run("ValidateAWSConfiguration", func(t *testing.T) {
		if !awsUtil.HasCredentials() {
			t.Skip("AWS credentials not available")
		}
		
		err := awsUtil.Initialize(ctx)
		require.NoError(t, err, "AWS initialization should succeed")
		
		region := awsUtil.GetRegion()
		assert.NotEmpty(t, region, "AWS region should be configured")
		
		// Test basic AWS operations
		_, err = awsUtil.ListVPCs(ctx, "validation-test")
		require.NoError(t, err, "Should be able to list VPCs")
		
		logger.Info("AWS configuration validated", "region", region)
	})
	
	logger.Info("E2E environment validation completed successfully")
}