package utils

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/capi-mcp/capi-mcp-server/api/v1"
)

// MCPClient provides utilities for communicating with the MCP server in E2E tests
type MCPClient struct {
	baseURL    string
	httpClient *http.Client
	logger     *slog.Logger
	apiKey     string
}

// MCPResponse represents a response from the MCP server
type MCPResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// MCPToolRequest represents a tool invocation request
type MCPToolRequest struct {
	Tool       string      `json:"tool"`
	Parameters interface{} `json:"parameters"`
}

// NewMCPClient creates a new MCP client instance
func NewMCPClient(baseURL string, logger *slog.Logger) (*MCPClient, error) {
	if baseURL == "" {
		return nil, fmt.Errorf("base URL cannot be empty")
	}
	
	if logger == nil {
		return nil, fmt.Errorf("logger cannot be nil")
	}
	
	return &MCPClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		logger: logger,
		apiKey: "test-api-key", // Default test API key
	}, nil
}

// SetAPIKey sets the API key for authentication
func (c *MCPClient) SetAPIKey(apiKey string) {
	c.apiKey = apiKey
}

// SetTimeout sets the HTTP client timeout
func (c *MCPClient) SetTimeout(timeout time.Duration) {
	c.httpClient.Timeout = timeout
}

// TestConnection tests the connection to the MCP server
func (c *MCPClient) TestConnection() error {
	c.logger.Info("Testing MCP server connection", "url", c.baseURL)
	
	req, err := http.NewRequest("GET", c.baseURL+"/health", nil)
	if err != nil {
		return fmt.Errorf("failed to create health check request: %w", err)
	}
	
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("health check request failed: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("health check failed with status %d: %s", resp.StatusCode, string(body))
	}
	
	c.logger.Info("MCP server connection test successful")
	return nil
}

// ListClusters calls the list_clusters tool
func (c *MCPClient) ListClusters(ctx context.Context) (*v1.ListClustersOutput, error) {
	c.logger.Info("Calling list_clusters tool")
	
	request := MCPToolRequest{
		Tool:       "list_clusters",
		Parameters: v1.ListClustersInput{}, // No parameters needed
	}
	
	var response v1.ListClustersOutput
	if err := c.callTool(ctx, request, &response); err != nil {
		return nil, fmt.Errorf("list_clusters failed: %w", err)
	}
	
	return &response, nil
}

// GetCluster calls the get_cluster tool
func (c *MCPClient) GetCluster(ctx context.Context, clusterName string) (*v1.GetClusterOutput, error) {
	c.logger.Info("Calling get_cluster tool", "cluster", clusterName)
	
	request := MCPToolRequest{
		Tool: "get_cluster",
		Parameters: v1.GetClusterInput{
			ClusterName: clusterName,
		},
	}
	
	var response v1.GetClusterOutput
	if err := c.callTool(ctx, request, &response); err != nil {
		return nil, fmt.Errorf("get_cluster failed: %w", err)
	}
	
	return &response, nil
}

// CreateCluster calls the create_cluster tool
func (c *MCPClient) CreateCluster(ctx context.Context, input v1.CreateClusterInput) (*v1.CreateClusterOutput, error) {
	c.logger.Info("Calling create_cluster tool", "cluster", input.ClusterName, "template", input.TemplateName)
	
	request := MCPToolRequest{
		Tool:       "create_cluster",
		Parameters: input,
	}
	
	var response v1.CreateClusterOutput
	if err := c.callTool(ctx, request, &response); err != nil {
		return nil, fmt.Errorf("create_cluster failed: %w", err)
	}
	
	return &response, nil
}

// DeleteCluster calls the delete_cluster tool
func (c *MCPClient) DeleteCluster(ctx context.Context, clusterName string) (*v1.DeleteClusterOutput, error) {
	c.logger.Info("Calling delete_cluster tool", "cluster", clusterName)
	
	request := MCPToolRequest{
		Tool: "delete_cluster",
		Parameters: v1.DeleteClusterInput{
			ClusterName: clusterName,
		},
	}
	
	var response v1.DeleteClusterOutput
	if err := c.callTool(ctx, request, &response); err != nil {
		return nil, fmt.Errorf("delete_cluster failed: %w", err)
	}
	
	return &response, nil
}

// ScaleCluster calls the scale_cluster tool
func (c *MCPClient) ScaleCluster(ctx context.Context, input v1.ScaleClusterInput) (*v1.ScaleClusterOutput, error) {
	c.logger.Info("Calling scale_cluster tool", 
		"cluster", input.ClusterName, 
		"nodePool", input.NodePoolName, 
		"replicas", input.Replicas)
	
	request := MCPToolRequest{
		Tool:       "scale_cluster",
		Parameters: input,
	}
	
	var response v1.ScaleClusterOutput
	if err := c.callTool(ctx, request, &response); err != nil {
		return nil, fmt.Errorf("scale_cluster failed: %w", err)
	}
	
	return &response, nil
}

// GetClusterKubeconfig calls the get_cluster_kubeconfig tool
func (c *MCPClient) GetClusterKubeconfig(ctx context.Context, clusterName string) (*v1.GetClusterKubeconfigOutput, error) {
	c.logger.Info("Calling get_cluster_kubeconfig tool", "cluster", clusterName)
	
	request := MCPToolRequest{
		Tool: "get_cluster_kubeconfig",
		Parameters: v1.GetClusterKubeconfigInput{
			ClusterName: clusterName,
		},
	}
	
	var response v1.GetClusterKubeconfigOutput
	if err := c.callTool(ctx, request, &response); err != nil {
		return nil, fmt.Errorf("get_cluster_kubeconfig failed: %w", err)
	}
	
	return &response, nil
}

// GetClusterNodes calls the get_cluster_nodes tool
func (c *MCPClient) GetClusterNodes(ctx context.Context, clusterName string) (*v1.GetClusterNodesOutput, error) {
	c.logger.Info("Calling get_cluster_nodes tool", "cluster", clusterName)
	
	request := MCPToolRequest{
		Tool: "get_cluster_nodes",
		Parameters: v1.GetClusterNodesInput{
			ClusterName: clusterName,
		},
	}
	
	var response v1.GetClusterNodesOutput
	if err := c.callTool(ctx, request, &response); err != nil {
		return nil, fmt.Errorf("get_cluster_nodes failed: %w", err)
	}
	
	return &response, nil
}

// callTool makes a generic tool call to the MCP server
func (c *MCPClient) callTool(ctx context.Context, request MCPToolRequest, response interface{}) error {
	// Serialize request
	requestBody, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}
	
	c.logger.Debug("Making MCP tool call", 
		"tool", request.Tool,
		"url", c.baseURL+"/tools/call")
	
	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/tools/call", bytes.NewReader(requestBody))
	if err != nil {
		return fmt.Errorf("failed to create HTTP request: %w", err)
	}
	
	// Set headers
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	
	// Make request
	httpResp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("HTTP request failed: %w", err)
	}
	defer httpResp.Body.Close()
	
	// Read response body
	responseBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}
	
	// Check HTTP status
	if httpResp.StatusCode != http.StatusOK {
		return fmt.Errorf("tool call failed with status %d: %s", httpResp.StatusCode, string(responseBody))
	}
	
	// Parse MCP response
	var mcpResp MCPResponse
	if err := json.Unmarshal(responseBody, &mcpResp); err != nil {
		return fmt.Errorf("failed to parse MCP response: %w", err)
	}
	
	// Check for MCP-level errors
	if !mcpResp.Success {
		return fmt.Errorf("MCP tool call failed: %s", mcpResp.Error)
	}
	
	// Marshal and unmarshal data to convert to target type
	dataBytes, err := json.Marshal(mcpResp.Data)
	if err != nil {
		return fmt.Errorf("failed to marshal response data: %w", err)
	}
	
	if err := json.Unmarshal(dataBytes, response); err != nil {
		return fmt.Errorf("failed to unmarshal response data: %w", err)
	}
	
	c.logger.Debug("MCP tool call successful", "tool", request.Tool)
	return nil
}

// WaitForClusterReady waits for a cluster to be ready by polling get_cluster
func (c *MCPClient) WaitForClusterReady(ctx context.Context, clusterName string, timeout time.Duration) error {
	c.logger.Info("Waiting for cluster to be ready via MCP", 
		"cluster", clusterName, 
		"timeout", timeout)
	
	deadline := time.Now().Add(timeout)
	
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		
		cluster, err := c.GetCluster(ctx, clusterName)
		if err != nil {
			c.logger.Warn("Failed to get cluster status", "error", err)
			time.Sleep(30 * time.Second)
			continue
		}
		
		c.logger.Debug("Cluster status check via MCP",
			"cluster", clusterName,
			"phase", cluster.Status.Phase,
			"controlPlaneReady", cluster.Status.ControlPlaneReady,
			"infrastructureReady", cluster.Status.InfrastructureReady)
		
		if cluster.Status.Phase == "Provisioned" &&
			cluster.Status.ControlPlaneReady &&
			cluster.Status.InfrastructureReady {
			c.logger.Info("Cluster is ready via MCP", "cluster", clusterName)
			return nil
		}
		
		if cluster.Status.Phase == "Failed" {
			return fmt.Errorf("cluster failed to provision: %s", clusterName)
		}
		
		time.Sleep(30 * time.Second)
	}
	
	return fmt.Errorf("timeout waiting for cluster %s to be ready", clusterName)
}

// WaitForClusterDeleted waits for a cluster to be deleted by polling get_cluster
func (c *MCPClient) WaitForClusterDeleted(ctx context.Context, clusterName string, timeout time.Duration) error {
	c.logger.Info("Waiting for cluster to be deleted via MCP", 
		"cluster", clusterName, 
		"timeout", timeout)
	
	deadline := time.Now().Add(timeout)
	
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		
		_, err := c.GetCluster(ctx, clusterName)
		if err != nil {
			// Check if this is a "not found" error
			if isNotFoundError(err) {
				c.logger.Info("Cluster has been deleted via MCP", "cluster", clusterName)
				return nil
			}
			
			c.logger.Warn("Error checking cluster deletion status", "error", err)
		}
		
		time.Sleep(10 * time.Second)
	}
	
	return fmt.Errorf("timeout waiting for cluster %s to be deleted", clusterName)
}

// isNotFoundError checks if an error indicates a resource was not found
func isNotFoundError(err error) bool {
	// This is a simple check - in a real implementation, you might want to 
	// parse the error message more carefully or use structured error types
	return err != nil && (
		fmt.Sprintf("%v", err) == "cluster not found" ||
		fmt.Sprintf("%v", err) == "resource not found")
}

// ValidateToolResponse validates that a tool response has the expected structure
func (c *MCPClient) ValidateToolResponse(response interface{}, expectedFields ...string) error {
	responseMap, ok := response.(map[string]interface{})
	if !ok {
		return fmt.Errorf("response is not a map")
	}
	
	for _, field := range expectedFields {
		if _, exists := responseMap[field]; !exists {
			return fmt.Errorf("expected field %s not found in response", field)
		}
	}
	
	return nil
}