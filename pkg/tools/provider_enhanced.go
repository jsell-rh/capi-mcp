package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	api "github.com/capi-mcp/capi-mcp-server/api/v1"
	"github.com/capi-mcp/capi-mcp-server/internal/errors"
	"github.com/capi-mcp/capi-mcp-server/internal/logging"
	"github.com/capi-mcp/capi-mcp-server/internal/service"
	"github.com/capi-mcp/capi-mcp-server/internal/validation"
)

// EnhancedProvider handles MCP tool registration and execution with enhanced error handling.
type EnhancedProvider struct {
	mcpServer      *mcp.Server
	logger         *logging.Logger
	clusterService interface{} // Can be either ClusterService or EnhancedClusterService
	validator      *validation.Validator
}

// NewEnhancedProvider creates a new enhanced tool provider instance.
func NewEnhancedProvider(mcpServer *mcp.Server, logger *logging.Logger, clusterService interface{}) *EnhancedProvider {
	return &EnhancedProvider{
		mcpServer:      mcpServer,
		logger:         logger.WithComponent("tools"),
		clusterService: clusterService,
		validator:      validation.NewValidator(),
	}
}

// GetSupportedTools returns a list of supported tools for this provider.
func (p *EnhancedProvider) GetSupportedTools() []string {
	return []string{
		"list_clusters",
		"get_cluster", 
		"create_cluster",
		"delete_cluster",
		"scale_cluster",
		"get_cluster_kubeconfig",
		"get_cluster_nodes",
	}
}

// RegisterTools registers all supported tools with the MCP server.
func (p *EnhancedProvider) RegisterTools() error {
	if p.mcpServer == nil {
		return errors.New(errors.CodeInternal, "MCP server not initialized")
	}

	// Register tools using proper typed MCP handlers
	p.mcpServer.AddTools(mcp.NewServerTool(
		"list_clusters",
		"List all managed workload clusters and their current status",
		p.handleListClustersTyped,
	))

	p.mcpServer.AddTools(mcp.NewServerTool(
		"get_cluster",
		"Get detailed information for a specific cluster",
		p.handleGetClusterTyped,
		mcp.Input(
			mcp.Property("clusterName", mcp.Required(true), mcp.Description("The name of the cluster to retrieve")),
		),
	))

	p.mcpServer.AddTools(mcp.NewServerTool(
		"create_cluster",
		"Create a new workload cluster from templates",
		p.handleCreateClusterTyped,
		mcp.Input(
			mcp.Property("clusterName", mcp.Required(true), mcp.Description("The name for the new cluster")),
			mcp.Property("templateName", mcp.Required(true), mcp.Description("The cluster template to use")),
			mcp.Property("variables", mcp.Description("Variables to use with the template")),
		),
	))

	p.mcpServer.AddTools(mcp.NewServerTool(
		"delete_cluster",
		"Delete a workload cluster",
		p.handleDeleteClusterTyped,
		mcp.Input(
			mcp.Property("clusterName", mcp.Required(true), mcp.Description("The name of the cluster to delete")),
		),
	))

	p.mcpServer.AddTools(mcp.NewServerTool(
		"scale_cluster",
		"Scale worker nodes in a cluster",
		p.handleScaleClusterTyped,
		mcp.Input(
			mcp.Property("clusterName", mcp.Required(true), mcp.Description("The name of the cluster to scale")),
			mcp.Property("nodePoolName", mcp.Required(true), mcp.Description("The node pool to scale")),
			mcp.Property("replicas", mcp.Required(true), mcp.Description("The desired number of replicas")),
		),
	))

	p.mcpServer.AddTools(mcp.NewServerTool(
		"get_cluster_kubeconfig",
		"Retrieve cluster access credentials",
		p.handleGetClusterKubeconfigTyped,
		mcp.Input(
			mcp.Property("clusterName", mcp.Required(true), mcp.Description("The name of the cluster")),
		),
	))

	p.mcpServer.AddTools(mcp.NewServerTool(
		"get_cluster_nodes",
		"List nodes within a cluster",
		p.handleGetClusterNodesTyped,
		mcp.Input(
			mcp.Property("clusterName", mcp.Required(true), mcp.Description("The name of the cluster")),
		),
	))
	
	p.logger.Info("Registered all MCP tools", "count", 7)
	return nil
}

// Define argument types for enhanced provider (avoid naming conflicts)
type EnhancedEmptyArgs struct{}
type EnhancedListClustersArgs = EnhancedEmptyArgs

type EnhancedGetClusterArgs struct {
	ClusterName string `json:"clusterName"`
}

type EnhancedCreateClusterArgs struct {
	ClusterName  string                 `json:"clusterName"`
	TemplateName string                 `json:"templateName"`
	Variables    map[string]interface{} `json:"variables,omitempty"`
}

type EnhancedDeleteClusterArgs struct {
	ClusterName string `json:"clusterName"`
}

type EnhancedScaleClusterArgs struct {
	ClusterName  string `json:"clusterName"`
	NodePoolName string `json:"nodePoolName"`
	Replicas     int    `json:"replicas"`
}

type EnhancedGetClusterKubeconfigArgs struct {
	ClusterName string `json:"clusterName"`
}

type EnhancedGetClusterNodesArgs struct {
	ClusterName string `json:"clusterName"`
}

// Typed MCP tool handlers

func (p *EnhancedProvider) handleListClustersTyped(ctx context.Context, session *mcp.ServerSession, params *mcp.CallToolParamsFor[EnhancedListClustersArgs]) (*mcp.CallToolResultFor[api.ListClustersOutput], error) {
	p.logger.Info("handling list_clusters")
	
	// Convert to internal map format and call existing handler
	arguments := make(map[string]interface{})
	result, err := p.handleListClusters(ctx, arguments)
	if err != nil {
		return nil, p.sanitizeError(err)
	}
	
	// Convert result to API type - for now just ignore the output data
	// TODO: Figure out proper way to return structured data through MCP SDK
	_ = result // Ignore the result for now
	
	return &mcp.CallToolResultFor[api.ListClustersOutput]{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: "Successfully listed clusters",
			},
		},
	}, nil
}

func (p *EnhancedProvider) handleGetClusterTyped(ctx context.Context, session *mcp.ServerSession, params *mcp.CallToolParamsFor[EnhancedGetClusterArgs]) (*mcp.CallToolResultFor[api.GetClusterOutput], error) {
	p.logger.Info("handling get_cluster", "cluster", params.Arguments.ClusterName)
	
	// Convert to internal map format and call existing handler
	arguments := map[string]interface{}{
		"clusterName": params.Arguments.ClusterName,
	}
	result, err := p.handleGetCluster(ctx, arguments)
	if err != nil {
		return nil, p.sanitizeError(err)
	}
	
	// Convert result to API type - for now just ignore the output data
	_ = result
	
	return &mcp.CallToolResultFor[api.GetClusterOutput]{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: "Successfully retrieved cluster information",
			},
		},
	}, nil
}

func (p *EnhancedProvider) handleCreateClusterTyped(ctx context.Context, session *mcp.ServerSession, params *mcp.CallToolParamsFor[EnhancedCreateClusterArgs]) (*mcp.CallToolResultFor[api.CreateClusterOutput], error) {
	p.logger.Info("handling create_cluster", "cluster", params.Arguments.ClusterName, "template", params.Arguments.TemplateName)
	
	// Convert to internal map format and call existing handler
	arguments := map[string]interface{}{
		"clusterName":  params.Arguments.ClusterName,
		"templateName": params.Arguments.TemplateName,
	}
	if params.Arguments.Variables != nil {
		arguments["variables"] = params.Arguments.Variables
	}
	
	result, err := p.handleCreateCluster(ctx, arguments)
	if err != nil {
		return nil, p.sanitizeError(err)
	}
	
	// Convert result to API type - for now just ignore the output data
	_ = result
	
	return &mcp.CallToolResultFor[api.CreateClusterOutput]{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: "Successfully initiated cluster creation",
			},
		},
	}, nil
}

func (p *EnhancedProvider) handleDeleteClusterTyped(ctx context.Context, session *mcp.ServerSession, params *mcp.CallToolParamsFor[EnhancedDeleteClusterArgs]) (*mcp.CallToolResultFor[api.DeleteClusterOutput], error) {
	p.logger.Info("handling delete_cluster", "cluster", params.Arguments.ClusterName)
	
	// Convert to internal map format and call existing handler
	arguments := map[string]interface{}{
		"clusterName": params.Arguments.ClusterName,
	}
	result, err := p.handleDeleteCluster(ctx, arguments)
	if err != nil {
		return nil, p.sanitizeError(err)
	}
	
	// Convert result to API type - for now just ignore the output data
	_ = result
	
	return &mcp.CallToolResultFor[api.DeleteClusterOutput]{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: "Successfully initiated cluster deletion",
			},
		},
	}, nil
}

func (p *EnhancedProvider) handleScaleClusterTyped(ctx context.Context, session *mcp.ServerSession, params *mcp.CallToolParamsFor[EnhancedScaleClusterArgs]) (*mcp.CallToolResultFor[api.ScaleClusterOutput], error) {
	p.logger.Info("handling scale_cluster", "cluster", params.Arguments.ClusterName, "nodePool", params.Arguments.NodePoolName, "replicas", params.Arguments.Replicas)
	
	// Convert to internal map format and call existing handler
	arguments := map[string]interface{}{
		"clusterName":  params.Arguments.ClusterName,
		"nodePoolName": params.Arguments.NodePoolName,
		"replicas":     params.Arguments.Replicas,
	}
	result, err := p.handleScaleCluster(ctx, arguments)
	if err != nil {
		return nil, p.sanitizeError(err)
	}
	
	// Convert result to API type - for now just ignore the output data
	_ = result
	
	return &mcp.CallToolResultFor[api.ScaleClusterOutput]{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: "Successfully initiated cluster scaling",
			},
		},
	}, nil
}

func (p *EnhancedProvider) handleGetClusterKubeconfigTyped(ctx context.Context, session *mcp.ServerSession, params *mcp.CallToolParamsFor[EnhancedGetClusterKubeconfigArgs]) (*mcp.CallToolResultFor[api.GetClusterKubeconfigOutput], error) {
	p.logger.Info("handling get_cluster_kubeconfig", "cluster", params.Arguments.ClusterName)
	
	// Convert to internal map format and call existing handler
	arguments := map[string]interface{}{
		"clusterName": params.Arguments.ClusterName,
	}
	result, err := p.handleGetClusterKubeconfig(ctx, arguments)
	if err != nil {
		return nil, p.sanitizeError(err)
	}
	
	// Convert result to API type - for now just ignore the output data
	_ = result
	
	return &mcp.CallToolResultFor[api.GetClusterKubeconfigOutput]{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: "Successfully retrieved cluster kubeconfig",
			},
		},
	}, nil
}

func (p *EnhancedProvider) handleGetClusterNodesTyped(ctx context.Context, session *mcp.ServerSession, params *mcp.CallToolParamsFor[EnhancedGetClusterNodesArgs]) (*mcp.CallToolResultFor[api.GetClusterNodesOutput], error) {
	p.logger.Info("handling get_cluster_nodes", "cluster", params.Arguments.ClusterName)
	
	// Convert to internal map format and call existing handler
	arguments := map[string]interface{}{
		"clusterName": params.Arguments.ClusterName,
	}
	result, err := p.handleGetClusterNodes(ctx, arguments)
	if err != nil {
		return nil, p.sanitizeError(err)
	}
	
	// Convert result to API type - for now just ignore the output data
	_ = result
	
	return &mcp.CallToolResultFor[api.GetClusterNodesOutput]{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: "Successfully retrieved cluster nodes",
			},
		},
	}, nil
}

// wrapToolHandler wraps a tool handler with logging and error handling
func (p *EnhancedProvider) wrapToolHandler(toolName string, handler func(context.Context, map[string]interface{}) (interface{}, error)) func(context.Context, map[string]interface{}) (map[string]interface{}, error) {
	return func(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
		// Add tool context to logger
		toolLogger := p.logger.WithContext(ctx).With(
			logging.FieldTool, toolName,
		)
		
		// Log tool invocation
		toolLogger.Info("Tool invocation started")
		result, err := handler(ctx, input)
		if err != nil {
			p.logger.WithError(err).Error("Tool invocation failed", logging.FieldTool, toolName)
		} else {
			toolLogger.Info("Tool invocation completed")
		}
		
		if err != nil {
			// Return user-friendly error
			userErr := p.sanitizeError(err)
			return nil, userErr
		}
		
		// Convert result to map
		resultMap, ok := result.(map[string]interface{})
		if !ok {
			// Try to convert using JSON marshaling
			return convertToMap(result)
		}
		
		return resultMap, nil
	}
}

// sanitizeError converts internal errors to user-friendly errors
func (p *EnhancedProvider) sanitizeError(err error) error {
	if err == nil {
		return nil
	}
	
	// Get error code and user message
	code := errors.GetErrorCode(err)
	userMsg := errors.GetUserMessage(err)
	
	// Create sanitized error with code
	sanitized := errors.New(code, userMsg)
	
	// Add selected details if available
	if e, ok := err.(*errors.Error); ok && e.Details != nil {
		// Only include safe details
		safeDetails := make(map[string]interface{})
		for key, value := range e.Details {
			switch key {
			case "field", "resource", "operation":
				safeDetails[key] = value
			}
		}
		if len(safeDetails) > 0 {
			sanitized.WithDetailsMap(safeDetails)
		}
	}
	
	return sanitized
}

// Tool handler implementations

func (p *EnhancedProvider) handleListClusters(ctx context.Context, input map[string]interface{}) (interface{}, error) {
	// Validate input (list_clusters has no required parameters)
	// But we still parse it to ensure it's valid
	var listInput api.ListClustersInput
	if err := parseInput(input, &listInput); err != nil {
		return nil, errors.Wrap(err, errors.CodeInvalidInput, "invalid input parameters")
	}
	
	// Check if cluster service is available
	if p.clusterService == nil {
		return nil, errors.New(errors.CodeUnavailable, "cluster service not available")
	}
	
	// Call the appropriate service method
	switch svc := p.clusterService.(type) {
	case *service.ClusterService:
		output, err := svc.ListClusters(ctx)
		if err != nil {
			return nil, err
		}
		return convertToMap(output)
		
	case *service.EnhancedClusterService:
		output, err := svc.ListClusters(ctx)
		if err != nil {
			return nil, err
		}
		return convertToMap(output)
		
	default:
		return nil, errors.New(errors.CodeInternal, "unknown cluster service type")
	}
}

func (p *EnhancedProvider) handleGetCluster(ctx context.Context, input map[string]interface{}) (interface{}, error) {
	// Validate cluster name from input
	if err := p.validateClusterNameFromInput(input); err != nil {
		return nil, err
	}
	
	// Parse input after validation
	var getInput api.GetClusterInput
	if err := parseInput(input, &getInput); err != nil {
		return nil, errors.Wrap(err, errors.CodeInvalidInput, "failed to parse validated input")
	}
	
	// Check if cluster service is available
	if p.clusterService == nil {
		return nil, errors.New(errors.CodeUnavailable, "cluster service not available")
	}
	
	// Call the appropriate service method
	switch svc := p.clusterService.(type) {
	case *service.ClusterService:
		output, err := svc.GetCluster(ctx, getInput)
		if err != nil {
			return nil, err
		}
		return convertToMap(output)
		
	case *service.EnhancedClusterService:
		output, err := svc.GetCluster(ctx, getInput)
		if err != nil {
			return nil, err
		}
		return convertToMap(output)
		
	default:
		return nil, errors.New(errors.CodeInternal, "unknown cluster service type")
	}
}

func (p *EnhancedProvider) handleCreateCluster(ctx context.Context, input map[string]interface{}) (interface{}, error) {
	// Comprehensive input validation using the enhanced validator
	if err := p.validator.ValidateCreateClusterInput(input); err != nil {
		return nil, err
	}
	
	// Parse input after validation
	var createInput api.CreateClusterInput
	if err := parseInput(input, &createInput); err != nil {
		return nil, errors.Wrap(err, errors.CodeInvalidInput, "failed to parse validated input")
	}
	
	// Check if cluster service is available
	if p.clusterService == nil {
		return nil, errors.New(errors.CodeUnavailable, "cluster service not available")
	}
	
	// Call the appropriate service method
	switch svc := p.clusterService.(type) {
	case *service.ClusterService:
		output, err := svc.CreateCluster(ctx, createInput)
		if err != nil {
			return nil, err
		}
		return convertToMap(output)
		
	case *service.EnhancedClusterService:
		output, err := svc.CreateCluster(ctx, createInput)
		if err != nil {
			return nil, err
		}
		return convertToMap(output)
		
	default:
		return nil, errors.New(errors.CodeInternal, "unknown cluster service type")
	}
}

func (p *EnhancedProvider) handleDeleteCluster(ctx context.Context, input map[string]interface{}) (interface{}, error) {
	// Validate cluster name from input
	if err := p.validateClusterNameFromInput(input); err != nil {
		return nil, err
	}
	
	// Parse input after validation
	var deleteInput api.DeleteClusterInput
	if err := parseInput(input, &deleteInput); err != nil {
		return nil, errors.Wrap(err, errors.CodeInvalidInput, "failed to parse validated input")
	}
	
	// Check if cluster service is available
	if p.clusterService == nil {
		return nil, errors.New(errors.CodeUnavailable, "cluster service not available")
	}
	
	// Call the appropriate service method
	switch svc := p.clusterService.(type) {
	case *service.ClusterService:
		output, err := svc.DeleteCluster(ctx, deleteInput)
		if err != nil {
			return nil, err
		}
		return convertToMap(output)
		
	case *service.EnhancedClusterService:
		output, err := svc.DeleteCluster(ctx, deleteInput)
		if err != nil {
			return nil, err
		}
		return convertToMap(output)
		
	default:
		return nil, errors.New(errors.CodeInternal, "unknown cluster service type")
	}
}

func (p *EnhancedProvider) handleScaleCluster(ctx context.Context, input map[string]interface{}) (interface{}, error) {
	// Comprehensive input validation using the enhanced validator
	if err := p.validator.ValidateScaleClusterInput(input); err != nil {
		return nil, err
	}
	
	// Parse input after validation
	var scaleInput api.ScaleClusterInput
	if err := parseInput(input, &scaleInput); err != nil {
		return nil, errors.Wrap(err, errors.CodeInvalidInput, "failed to parse validated input")
	}
	
	// Check if cluster service is available
	if p.clusterService == nil {
		return nil, errors.New(errors.CodeUnavailable, "cluster service not available")
	}
	
	// Call the appropriate service method
	switch svc := p.clusterService.(type) {
	case *service.ClusterService:
		output, err := svc.ScaleCluster(ctx, scaleInput)
		if err != nil {
			return nil, err
		}
		return convertToMap(output)
		
	case *service.EnhancedClusterService:
		output, err := svc.ScaleCluster(ctx, scaleInput)
		if err != nil {
			return nil, err
		}
		return convertToMap(output)
		
	default:
		return nil, errors.New(errors.CodeInternal, "unknown cluster service type")
	}
}

func (p *EnhancedProvider) handleGetClusterKubeconfig(ctx context.Context, input map[string]interface{}) (interface{}, error) {
	// Validate cluster name from input
	if err := p.validateClusterNameFromInput(input); err != nil {
		return nil, err
	}
	
	// Parse input after validation
	var kubeconfigInput api.GetClusterKubeconfigInput
	if err := parseInput(input, &kubeconfigInput); err != nil {
		return nil, errors.Wrap(err, errors.CodeInvalidInput, "failed to parse validated input")
	}
	
	// Check if cluster service is available
	if p.clusterService == nil {
		return nil, errors.New(errors.CodeUnavailable, "cluster service not available")
	}
	
	// Call the appropriate service method
	switch svc := p.clusterService.(type) {
	case *service.ClusterService:
		output, err := svc.GetClusterKubeconfig(ctx, kubeconfigInput)
		if err != nil {
			return nil, err
		}
		return convertToMap(output)
		
	case *service.EnhancedClusterService:
		output, err := svc.GetClusterKubeconfig(ctx, kubeconfigInput)
		if err != nil {
			return nil, err
		}
		return convertToMap(output)
		
	default:
		return nil, errors.New(errors.CodeInternal, "unknown cluster service type")
	}
}

func (p *EnhancedProvider) handleGetClusterNodes(ctx context.Context, input map[string]interface{}) (interface{}, error) {
	// Validate cluster name from input
	if err := p.validateClusterNameFromInput(input); err != nil {
		return nil, err
	}
	
	// Parse input after validation
	var nodesInput api.GetClusterNodesInput
	if err := parseInput(input, &nodesInput); err != nil {
		return nil, errors.Wrap(err, errors.CodeInvalidInput, "failed to parse validated input")
	}
	
	// Check if cluster service is available
	if p.clusterService == nil {
		return nil, errors.New(errors.CodeUnavailable, "cluster service not available")
	}
	
	// Call the appropriate service method
	switch svc := p.clusterService.(type) {
	case *service.ClusterService:
		output, err := svc.GetClusterNodes(ctx, nodesInput)
		if err != nil {
			return nil, err
		}
		return convertToMap(output)
		
	case *service.EnhancedClusterService:
		output, err := svc.GetClusterNodes(ctx, nodesInput)
		if err != nil {
			return nil, err
		}
		return convertToMap(output)
		
	default:
		return nil, errors.New(errors.CodeInternal, "unknown cluster service type")
	}
}

// Helper validation functions

// validateClusterNameFromInput validates cluster name from raw input map
func (p *EnhancedProvider) validateClusterNameFromInput(input map[string]interface{}) error {
	clusterName, ok := input["clusterName"].(string)
	if !ok {
		return errors.New(errors.CodeInvalidInput, 
			"clusterName is required and must be a string").
			WithDetails("field", "clusterName").
			WithDetails("provided_type", fmt.Sprintf("%T", input["clusterName"]))
	}
	
	if err := p.validator.ValidateClusterName(clusterName); err != nil {
		return err
	}
	
	return nil
}

// Helper function to convert structs to maps
func convertToMap(v interface{}) (map[string]interface{}, error) {
	// This is a simplified version - in production, use proper JSON marshaling
	// or reflection-based conversion
	switch val := v.(type) {
	case map[string]interface{}:
		return val, nil
	case *api.ListClustersOutput:
		return map[string]interface{}{
			"clusters": val.Clusters,
		}, nil
	case *api.GetClusterOutput:
		return map[string]interface{}{
			"cluster":        val.Cluster,
			// Note: ProviderStatus removed from API structure
		}, nil
	case *api.CreateClusterOutput:
		return map[string]interface{}{
			"cluster_name": val.ClusterName,
			"status":       val.Status,
			"message":      val.Message,
		}, nil
	case *api.DeleteClusterOutput:
		return map[string]interface{}{
			"status":  val.Status,
			"message": val.Message,
		}, nil
	case *api.ScaleClusterOutput:
		return map[string]interface{}{
			"status":       val.Status,
			"message":      val.Message,
			"oldReplicas":  val.OldReplicas,
			"newReplicas":  val.NewReplicas,
		}, nil
	case *api.GetClusterKubeconfigOutput:
		return map[string]interface{}{
			"kubeconfig": val.Kubeconfig,
		}, nil
	case *api.GetClusterNodesOutput:
		return map[string]interface{}{
			"nodes": val.Nodes,
		}, nil
	default:
		return nil, errors.New(errors.CodeInternal, "unsupported output type")
	}
}

// parseInput parses the input map into a target struct
func parseInput(input map[string]interface{}, target interface{}) error {
	// Simple approach: marshal to JSON then unmarshal to struct
	// This handles type conversions automatically
	jsonData, err := json.Marshal(input)
	if err != nil {
		return fmt.Errorf("failed to marshal input: %w", err)
	}
	
	if err := json.Unmarshal(jsonData, target); err != nil {
		return fmt.Errorf("failed to parse input: %w", err)
	}
	
	return nil
}