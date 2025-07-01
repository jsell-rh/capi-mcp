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

// RegisterTools registers all available tools with the MCP server with enhanced error handling.
func (p *EnhancedProvider) RegisterTools() error {
	logger := p.logger.WithOperation("RegisterTools")
	logger.Info("Registering MCP tools")
	
	tools := map[string]struct {
		handler mcp.ToolFunc
		schema  interface{}
		desc    string
	}{
		"list_clusters": {
			handler: p.wrapToolHandler("list_clusters", p.handleListClusters),
			schema:  api.ListClustersInput{},
			desc:    "Lists all managed Kubernetes clusters and their current status",
		},
		"get_cluster": {
			handler: p.wrapToolHandler("get_cluster", p.handleGetCluster),
			schema:  api.GetClusterInput{},
			desc:    "Gets detailed information about a specific cluster",
		},
		"create_cluster": {
			handler: p.wrapToolHandler("create_cluster", p.handleCreateCluster),
			schema:  api.CreateClusterInput{},
			desc:    "Creates a new Kubernetes cluster from a pre-defined template",
		},
		"delete_cluster": {
			handler: p.wrapToolHandler("delete_cluster", p.handleDeleteCluster),
			schema:  api.DeleteClusterInput{},
			desc:    "Deletes a Kubernetes cluster and all its resources",
		},
		"scale_cluster": {
			handler: p.wrapToolHandler("scale_cluster", p.handleScaleCluster),
			schema:  api.ScaleClusterInput{},
			desc:    "Scales the number of worker nodes in a cluster",
		},
		"get_cluster_kubeconfig": {
			handler: p.wrapToolHandler("get_cluster_kubeconfig", p.handleGetClusterKubeconfig),
			schema:  api.GetClusterKubeconfigInput{},
			desc:    "Retrieves the kubeconfig file for connecting to a cluster",
		},
		"get_cluster_nodes": {
			handler: p.wrapToolHandler("get_cluster_nodes", p.handleGetClusterNodes),
			schema:  api.GetClusterNodesInput{},
			desc:    "Lists all nodes in a specific cluster with their status",
		},
	}
	
	// Register each tool
	registeredCount := 0
	for name, tool := range tools {
		if err := p.mcpServer.RegisterTool(name, tool.desc, tool.schema, tool.handler); err != nil {
			logger.WithError(err).Error("Failed to register tool", 
				logging.FieldTool, name,
			)
			return errors.Wrap(err, errors.CodeInternal, fmt.Sprintf("failed to register tool %s", name))
		}
		registeredCount++
		logger.Debug("Registered tool", logging.FieldTool, name)
	}
	
	logger.Info("Successfully registered all tools", "count", registeredCount)
	return nil
}

// wrapToolHandler wraps a tool handler with logging and error handling
func (p *EnhancedProvider) wrapToolHandler(toolName string, handler func(context.Context, map[string]interface{}) (interface{}, error)) mcp.ToolFunc {
	return func(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
		// Add tool context to logger
		toolLogger := p.logger.WithContext(ctx).With(
			logging.FieldTool, toolName,
		)
		
		// Log tool invocation
		result, err := toolLogger.LogToolCall(ctx, toolName, input, func() (interface{}, error) {
			return handler(ctx, input)
		})
		
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
	// Parse and validate input
	var getInput api.GetClusterInput
	if err := parseInput(input, &getInput); err != nil {
		return nil, errors.Wrap(err, errors.CodeInvalidInput, "invalid input parameters")
	}
	
	// Validate cluster name
	if err := p.validator.ValidateClusterName(getInput.ClusterName); err != nil {
		return nil, err
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
	// Parse and validate input
	var createInput api.CreateClusterInput
	if err := parseInput(input, &createInput); err != nil {
		return nil, errors.Wrap(err, errors.CodeInvalidInput, "invalid input parameters")
	}
	
	// Validate cluster name
	if err := p.validator.ValidateClusterName(createInput.ClusterName); err != nil {
		return nil, err
	}
	
	// Validate Kubernetes version
	if err := p.validator.ValidateKubernetesVersion(createInput.KubernetesVersion); err != nil {
		return nil, err
	}
	
	// Validate variables if present
	if createInput.Variables != nil {
		if err := p.validator.ValidateClusterVariables(createInput.Variables); err != nil {
			return nil, err
		}
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
	// Parse and validate input
	var deleteInput api.DeleteClusterInput
	if err := parseInput(input, &deleteInput); err != nil {
		return nil, errors.Wrap(err, errors.CodeInvalidInput, "invalid input parameters")
	}
	
	// Validate cluster name
	if err := p.validator.ValidateClusterName(deleteInput.ClusterName); err != nil {
		return nil, err
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
	// Parse and validate input
	var scaleInput api.ScaleClusterInput
	if err := parseInput(input, &scaleInput); err != nil {
		return nil, errors.Wrap(err, errors.CodeInvalidInput, "invalid input parameters")
	}
	
	// Validate cluster name
	if err := p.validator.ValidateClusterName(scaleInput.ClusterName); err != nil {
		return nil, err
	}
	
	// Validate replica count
	if err := p.validator.ValidateReplicaCount(int32(scaleInput.Replicas)); err != nil {
		return nil, err
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
	// Parse and validate input
	var kubeconfigInput api.GetClusterKubeconfigInput
	if err := parseInput(input, &kubeconfigInput); err != nil {
		return nil, errors.Wrap(err, errors.CodeInvalidInput, "invalid input parameters")
	}
	
	// Validate cluster name
	if err := p.validator.ValidateClusterName(kubeconfigInput.ClusterName); err != nil {
		return nil, err
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
	// Parse and validate input
	var nodesInput api.GetClusterNodesInput
	if err := parseInput(input, &nodesInput); err != nil {
		return nil, errors.Wrap(err, errors.CodeInvalidInput, "invalid input parameters")
	}
	
	// Validate cluster name
	if err := p.validator.ValidateClusterName(nodesInput.ClusterName); err != nil {
		return nil, err
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
			"providerStatus": val.ProviderStatus,
		}, nil
	case *api.CreateClusterOutput:
		return map[string]interface{}{
			"success": val.Success,
			"cluster": val.Cluster,
			"message": val.Message,
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