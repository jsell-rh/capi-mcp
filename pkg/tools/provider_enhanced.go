package tools

import (
	"context"
	"encoding/json"
	"fmt"


	api "github.com/capi-mcp/capi-mcp-server/api/v1"
	"github.com/capi-mcp/capi-mcp-server/internal/errors"
	"github.com/capi-mcp/capi-mcp-server/internal/logging"
	"github.com/capi-mcp/capi-mcp-server/internal/service"
	"github.com/capi-mcp/capi-mcp-server/internal/validation"
)

// EnhancedProvider handles MCP tool registration and execution with enhanced error handling.
type EnhancedProvider struct {
	logger         *logging.Logger
	clusterService interface{} // Can be either ClusterService or EnhancedClusterService
	validator      *validation.Validator
}

// NewEnhancedProvider creates a new enhanced tool provider instance.
func NewEnhancedProvider(logger *logging.Logger, clusterService interface{}) *EnhancedProvider {
	return &EnhancedProvider{
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