package tools

import (
	"context"
	"fmt"
	"log/slog"

	api "github.com/capi-mcp/capi-mcp-server/api/v1"
	"github.com/capi-mcp/capi-mcp-server/internal/service"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Provider handles tool registration and execution.
type Provider struct {
	server         *mcp.Server
	logger         *slog.Logger
	clusterService *service.ClusterService
}

// NewProvider creates a new tool provider.
func NewProvider(server *mcp.Server, logger *slog.Logger, clusterService *service.ClusterService) *Provider {
	return &Provider{
		server:         server,
		logger:         logger,
		clusterService: clusterService,
	}
}

// RegisterTools registers all CAPI tools with the MCP server.
func (p *Provider) RegisterTools() error {
	// Register list_clusters tool
	p.server.AddTools(mcp.NewServerTool(
		"list_clusters",
		`Lists all managed workload clusters and their current status.
Returns a summary of all clusters managed by this CAPI management cluster, including their
current phase (e.g., Provisioned, Provisioning, Failed), Kubernetes version, and node count.
This tool is useful for getting an overview of the infrastructure under management.`,
		p.handleListClusters,
	))

	// Register get_cluster tool
	p.server.AddTools(mcp.NewServerTool(
		"get_cluster",
		`Gets detailed information for a specific cluster.
Retrieves comprehensive details about a single cluster including its status, conditions,
node pools, endpoint information, and infrastructure-specific details. Use this tool
when you need in-depth information about a particular cluster's configuration and state.`,
		p.handleGetCluster,
		mcp.Input(
			mcp.Property("cluster_name", mcp.Required(true), mcp.Description("The name of the cluster to retrieve")),
		),
	))

	// Register create_cluster tool
	p.server.AddTools(mcp.NewServerTool(
		"create_cluster",
		`Creates a new workload cluster from a pre-defined ClusterClass template.
This tool initiates the creation of a new Kubernetes cluster using a safe, administrator-approved
template. The operation is asynchronous and will wait for the cluster to be fully provisioned
before returning (or timeout after 10 minutes).`,
		p.handleCreateCluster,
		mcp.Input(
			mcp.Property("cluster_name", mcp.Required(true), mcp.Description("Unique name for the new cluster")),
			mcp.Property("template_name", mcp.Required(true), mcp.Description("Name of the ClusterClass template to use")),
			mcp.Property("kubernetes_version", mcp.Required(true), mcp.Description("Kubernetes version to deploy (e.g., v1.31.0)")),
			mcp.Property("variables", mcp.Description("Template-specific variables as key-value pairs")),
		),
	))

	// Register delete_cluster tool
	p.server.AddTools(mcp.NewServerTool(
		"delete_cluster",
		`Deletes a specified workload cluster and all its associated resources.
This tool initiates the deletion of a cluster and all its infrastructure. The operation
is asynchronous and will wait for complete deletion before returning.
WARNING: This operation is irreversible and will delete all workloads running on the cluster.`,
		p.handleDeleteCluster,
		mcp.Input(
			mcp.Property("cluster_name", mcp.Required(true), mcp.Description("Name of the cluster to delete")),
		),
	))

	// Register scale_cluster tool
	p.server.AddTools(mcp.NewServerTool(
		"scale_cluster",
		`Scales the number of worker nodes in a specific node pool (MachineDeployment).
Adjusts the replica count for a node pool, allowing you to scale the cluster capacity
up or down. The operation waits for the scaling to complete before returning.`,
		p.handleScaleCluster,
		mcp.Input(
			mcp.Property("cluster_name", mcp.Required(true), mcp.Description("Name of the cluster containing the node pool")),
			mcp.Property("node_pool_name", mcp.Required(true), mcp.Description("Name of the MachineDeployment to scale")),
			mcp.Property("replicas", mcp.Required(true), mcp.Description("Desired number of replicas (must be >= 0)")),
		),
	))

	// Register get_cluster_kubeconfig tool
	p.server.AddTools(mcp.NewServerTool(
		"get_cluster_kubeconfig",
		`Retrieves the kubeconfig file needed to access a workload cluster.
Returns the kubeconfig data that can be used to connect to and manage the specified
cluster using kubectl or other Kubernetes clients.
SECURITY: The returned kubeconfig contains sensitive credentials. Handle with care.`,
		p.handleGetClusterKubeconfig,
		mcp.Input(
			mcp.Property("cluster_name", mcp.Required(true), mcp.Description("Name of the cluster to get kubeconfig for")),
		),
	))

	// Register get_cluster_nodes tool
	p.server.AddTools(mcp.NewServerTool(
		"get_cluster_nodes",
		`Lists the nodes within a specific workload cluster.
Retrieves information about all nodes in the cluster, including their status,
roles, IP addresses, and other metadata. This tool connects to the workload
cluster's API server to gather node information.`,
		p.handleGetClusterNodes,
		mcp.Input(
			mcp.Property("cluster_name", mcp.Required(true), mcp.Description("Name of the cluster to list nodes from")),
		),
	))

	p.logger.Info("registered all CAPI tools")
	return nil
}

// Tool handler implementations (stubs for now)

// EmptyArgs is used for tools that don't require any arguments.
type EmptyArgs struct{}

// ListClustersArgs defines the arguments for list_clusters (empty).
type ListClustersArgs = EmptyArgs

func (p *Provider) handleListClusters(ctx context.Context, session *mcp.ServerSession, params *mcp.CallToolParamsFor[ListClustersArgs]) (*mcp.CallToolResultFor[api.ListClustersOutput], error) {
	p.logger.Info("handling list_clusters")

	if p.clusterService == nil {
		return &mcp.CallToolResultFor[api.ListClustersOutput]{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: "No clusters found (service not initialized)",
				},
			},
		}, nil
	}

	result, err := p.clusterService.ListClusters(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list clusters: %w", err)
	}

	return &mcp.CallToolResultFor[api.ListClustersOutput]{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: fmt.Sprintf("Found %d clusters", len(result.Clusters)),
			},
		},
	}, nil
}

// GetClusterArgs defines the arguments for get_cluster.
type GetClusterArgs struct {
	ClusterName string `json:"cluster_name"`
}

func (p *Provider) handleGetCluster(ctx context.Context, session *mcp.ServerSession, params *mcp.CallToolParamsFor[GetClusterArgs]) (*mcp.CallToolResultFor[api.GetClusterOutput], error) {
	p.logger.Info("handling get_cluster", "cluster_name", params.Arguments.ClusterName)

	if p.clusterService == nil {
		return nil, fmt.Errorf("cluster service not initialized")
	}

	input := api.GetClusterInput{
		ClusterName: params.Arguments.ClusterName,
	}

	result, err := p.clusterService.GetCluster(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to get cluster: %w", err)
	}

	return &mcp.CallToolResultFor[api.GetClusterOutput]{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: fmt.Sprintf("Cluster %s status: %s", result.Cluster.Name, result.Cluster.Status),
			},
		},
	}, nil
}

// CreateClusterArgs defines the arguments for create_cluster.
type CreateClusterArgs struct {
	ClusterName       string                 `json:"cluster_name"`
	TemplateName      string                 `json:"template_name"`
	KubernetesVersion string                 `json:"kubernetes_version"`
	Variables         map[string]interface{} `json:"variables,omitempty"`
}

func (p *Provider) handleCreateCluster(ctx context.Context, session *mcp.ServerSession, params *mcp.CallToolParamsFor[CreateClusterArgs]) (*mcp.CallToolResultFor[api.CreateClusterOutput], error) {
	if p.clusterService == nil {
		return nil, fmt.Errorf("cluster service not initialized")
	}

	p.logger.Info("handling create_cluster",
		"cluster_name", params.Arguments.ClusterName,
		"template_name", params.Arguments.TemplateName,
	)

	input := api.CreateClusterInput{
		ClusterName:       params.Arguments.ClusterName,
		TemplateName:      params.Arguments.TemplateName,
		KubernetesVersion: params.Arguments.KubernetesVersion,
		Variables:         params.Arguments.Variables,
	}

	result, err := p.clusterService.CreateCluster(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to create cluster: %w", err)
	}

	return &mcp.CallToolResultFor[api.CreateClusterOutput]{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: fmt.Sprintf("Cluster %s %s: %s", result.ClusterName, result.Status, result.Message),
			},
		},
	}, nil
}

// DeleteClusterArgs defines the arguments for delete_cluster.
type DeleteClusterArgs struct {
	ClusterName string `json:"cluster_name"`
}

func (p *Provider) handleDeleteCluster(ctx context.Context, session *mcp.ServerSession, params *mcp.CallToolParamsFor[DeleteClusterArgs]) (*mcp.CallToolResultFor[api.DeleteClusterOutput], error) {
	p.logger.Info("handling delete_cluster", "cluster_name", params.Arguments.ClusterName)

	input := api.DeleteClusterInput{
		ClusterName: params.Arguments.ClusterName,
	}

	result, err := p.clusterService.DeleteCluster(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to delete cluster: %w", err)
	}

	return &mcp.CallToolResultFor[api.DeleteClusterOutput]{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: fmt.Sprintf("Cluster deletion %s: %s", result.Status, result.Message),
			},
		},
	}, nil
}

// ScaleClusterArgs defines the arguments for scale_cluster.
type ScaleClusterArgs struct {
	ClusterName  string `json:"cluster_name"`
	NodePoolName string `json:"node_pool_name"`
	Replicas     int    `json:"replicas"`
}

func (p *Provider) handleScaleCluster(ctx context.Context, session *mcp.ServerSession, params *mcp.CallToolParamsFor[ScaleClusterArgs]) (*mcp.CallToolResultFor[api.ScaleClusterOutput], error) {
	p.logger.Info("handling scale_cluster",
		"cluster_name", params.Arguments.ClusterName,
		"node_pool_name", params.Arguments.NodePoolName,
		"replicas", params.Arguments.Replicas,
	)

	input := api.ScaleClusterInput{
		ClusterName:  params.Arguments.ClusterName,
		NodePoolName: params.Arguments.NodePoolName,
		Replicas:     params.Arguments.Replicas,
	}

	result, err := p.clusterService.ScaleCluster(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to scale cluster: %w", err)
	}

	return &mcp.CallToolResultFor[api.ScaleClusterOutput]{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: fmt.Sprintf("Scaling %s: %s (from %d to %d replicas)", result.Status, result.Message, result.OldReplicas, result.NewReplicas),
			},
		},
	}, nil
}

// GetClusterKubeconfigArgs defines the arguments for get_cluster_kubeconfig.
type GetClusterKubeconfigArgs struct {
	ClusterName string `json:"cluster_name"`
}

func (p *Provider) handleGetClusterKubeconfig(ctx context.Context, session *mcp.ServerSession, params *mcp.CallToolParamsFor[GetClusterKubeconfigArgs]) (*mcp.CallToolResultFor[api.GetClusterKubeconfigOutput], error) {
	p.logger.Info("handling get_cluster_kubeconfig", "cluster_name", params.Arguments.ClusterName)

	input := api.GetClusterKubeconfigInput{
		ClusterName: params.Arguments.ClusterName,
	}

	result, err := p.clusterService.GetClusterKubeconfig(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to get kubeconfig: %w", err)
	}

	return &mcp.CallToolResultFor[api.GetClusterKubeconfigOutput]{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: fmt.Sprintf("Retrieved kubeconfig for cluster %s (%d bytes)", params.Arguments.ClusterName, len(result.Kubeconfig)),
			},
		},
	}, nil
}

// GetClusterNodesArgs defines the arguments for get_cluster_nodes.
type GetClusterNodesArgs struct {
	ClusterName string `json:"cluster_name"`
}

func (p *Provider) handleGetClusterNodes(ctx context.Context, session *mcp.ServerSession, params *mcp.CallToolParamsFor[GetClusterNodesArgs]) (*mcp.CallToolResultFor[api.GetClusterNodesOutput], error) {
	p.logger.Info("handling get_cluster_nodes", "cluster_name", params.Arguments.ClusterName)

	input := api.GetClusterNodesInput{
		ClusterName: params.Arguments.ClusterName,
	}

	result, err := p.clusterService.GetClusterNodes(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to get cluster nodes: %w", err)
	}

	return &mcp.CallToolResultFor[api.GetClusterNodesOutput]{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: fmt.Sprintf("Found %d nodes in cluster %s", len(result.Nodes), params.Arguments.ClusterName),
			},
		},
	}, nil
}
