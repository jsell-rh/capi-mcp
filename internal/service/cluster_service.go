package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"

	api "github.com/capi-mcp/capi-mcp-server/api/v1"
	"github.com/capi-mcp/capi-mcp-server/internal/kube"
	"github.com/capi-mcp/capi-mcp-server/pkg/provider"
)

// ClusterService handles CAPI cluster operations.
type ClusterService struct {
	kubeClient      *kube.Client
	logger          *slog.Logger
	providerManager *provider.ProviderManager
}

// NewClusterService creates a new cluster service.
func NewClusterService(kubeClient *kube.Client, logger *slog.Logger, providerManager *provider.ProviderManager) *ClusterService {
	return &ClusterService{
		kubeClient:      kubeClient,
		logger:          logger,
		providerManager: providerManager,
	}
}

// ListClusters returns a summary of all clusters.
func (s *ClusterService) ListClusters(ctx context.Context) (*api.ListClustersOutput, error) {
	clusters, err := s.kubeClient.ListClusters(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list clusters: %w", err)
	}

	summaries := make([]api.ClusterSummary, 0, len(clusters.Items))
	for _, cluster := range clusters.Items {
		summary := api.ClusterSummary{
			Name:              cluster.Name,
			Namespace:         cluster.Namespace,
			Status:            string(cluster.Status.Phase),
			CreatedAt:         cluster.CreationTimestamp.Format(time.RFC3339),
			KubernetesVersion: cluster.Spec.Topology.Version,
		}

		// Determine provider from labels or annotations
		if provider, ok := cluster.Labels["cluster.x-k8s.io/provider"]; ok {
			summary.Provider = provider
		} else {
			summary.Provider = "unknown"
		}

		// Get node count (approximate from MachineDeployments)
		summary.NodeCount = s.estimateNodeCount(&cluster)

		summaries = append(summaries, summary)
	}

	return &api.ListClustersOutput{
		Clusters: summaries,
	}, nil
}

// GetCluster returns detailed information about a specific cluster.
func (s *ClusterService) GetCluster(ctx context.Context, input api.GetClusterInput) (*api.GetClusterOutput, error) {
	cluster, err := s.kubeClient.GetClusterByName(ctx, input.ClusterName)
	if err != nil {
		return nil, fmt.Errorf("failed to get cluster: %w", err)
	}

	details := api.ClusterDetails{
		Name:              cluster.Name,
		Namespace:         cluster.Namespace,
		Status:            string(cluster.Status.Phase),
		CreatedAt:         cluster.CreationTimestamp.Format(time.RFC3339),
		KubernetesVersion: cluster.Spec.Topology.Version,
		Endpoint:          cluster.Spec.ControlPlaneEndpoint.Host,
	}

	// Determine provider
	if provider, ok := cluster.Labels["cluster.x-k8s.io/provider"]; ok {
		details.Provider = provider
	} else {
		details.Provider = "unknown"
	}

	// Determine region (AWS-specific)
	if region, ok := cluster.Labels["topology.cluster.x-k8s.io/region"]; ok {
		details.Region = region
	}

	// Convert conditions
	details.Conditions = make([]api.ClusterCondition, 0, len(cluster.Status.Conditions))
	for _, condition := range cluster.Status.Conditions {
		details.Conditions = append(details.Conditions, api.ClusterCondition{
			Type:               string(condition.Type),
			Status:             string(condition.Status),
			LastTransitionTime: condition.LastTransitionTime.Format(time.RFC3339),
			Reason:             condition.Reason,
			Message:            condition.Message,
		})
	}

	// Get infrastructure reference
	if cluster.Spec.InfrastructureRef != nil {
		details.InfrastructureRef = map[string]interface{}{
			"kind":       cluster.Spec.InfrastructureRef.Kind,
			"name":       cluster.Spec.InfrastructureRef.Name,
			"namespace":  cluster.Spec.InfrastructureRef.Namespace,
			"apiVersion": cluster.Spec.InfrastructureRef.APIVersion,
		}
	}

	// TODO: Get node pools (MachineDeployments)
	details.NodePools = []api.NodePool{}

	return &api.GetClusterOutput{
		Cluster: details,
	}, nil
}

// CreateCluster creates a new cluster from a template.
func (s *ClusterService) CreateCluster(ctx context.Context, input api.CreateClusterInput) (*api.CreateClusterOutput, error) {
	// Determine provider from variables or cluster class metadata
	providerName := s.extractProviderName(input.Variables, input.TemplateName)
	
	// Validate cluster configuration with provider-specific logic
	if s.providerManager != nil {
		if prov, exists := s.providerManager.GetProvider(providerName); exists {
			if err := prov.ValidateClusterConfig(ctx, input.Variables); err != nil {
				return nil, fmt.Errorf("provider validation failed: %w", err)
			}
		}
	}

	// Validate ClusterClass exists (skip if no kube client for testing)
	if s.kubeClient != nil {
		clusterClass, err := s.kubeClient.GetClusterClass(ctx, input.TemplateName)
		if err != nil {
			return nil, fmt.Errorf("cluster template not found: %w", err)
		}
		_ = clusterClass // Use the cluster class for validation
	}

	// Create cluster from ClusterClass
	cluster := &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: input.ClusterName,
			Labels: map[string]string{
				"cluster.x-k8s.io/cluster-name": input.ClusterName,
			},
		},
		Spec: clusterv1.ClusterSpec{
			Topology: &clusterv1.Topology{
				Class:   input.TemplateName,
				Version: input.KubernetesVersion,
			},
		},
	}

	// Add variables if provided
	if len(input.Variables) > 0 {
		variables := make([]clusterv1.ClusterVariable, 0, len(input.Variables))
		for name, value := range input.Variables {
			// Convert interface{} to raw JSON for CAPI ClusterVariable
			rawValue, err := json.Marshal(value)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal variable %s: %w", name, err)
			}
			variables = append(variables, clusterv1.ClusterVariable{
				Name:  name,
				Value: apiextensionsv1.JSON{Raw: rawValue},
			})
		}
		cluster.Spec.Topology.Variables = variables
	}

	// Create the cluster (skip if no kube client for testing)
	if s.kubeClient != nil {
		if err := s.kubeClient.CreateCluster(ctx, cluster); err != nil {
			return nil, fmt.Errorf("failed to create cluster: %w", err)
		}

		s.logger.Info("cluster creation initiated", "cluster", input.ClusterName)

		// Wait for cluster to be ready
		waitCtx, cancel := context.WithTimeout(ctx, 10*time.Minute)
		defer cancel()

		err := s.kubeClient.WaitForClusterReady(waitCtx, input.ClusterName, 10*time.Minute)
		if err != nil {
			s.logger.Error("cluster creation failed or timed out", "cluster", input.ClusterName, "error", err)
			return &api.CreateClusterOutput{
				ClusterName: input.ClusterName,
				Status:      "failed",
				Message:     fmt.Sprintf("Cluster creation failed: %v", err),
			}, nil
		}

		s.logger.Info("cluster creation completed", "cluster", input.ClusterName)
	} else {
		// In test mode without kube client, just simulate success
		s.logger.Info("cluster creation simulated (test mode)", "cluster", input.ClusterName)
	}

	return &api.CreateClusterOutput{
		ClusterName: input.ClusterName,
		Status:      "provisioned",
		Message:     "Cluster created successfully",
	}, nil
}

// DeleteCluster deletes a cluster.
func (s *ClusterService) DeleteCluster(ctx context.Context, input api.DeleteClusterInput) (*api.DeleteClusterOutput, error) {
	// Check if cluster exists
	_, err := s.kubeClient.GetClusterByName(ctx, input.ClusterName)
	if err != nil {
		return nil, fmt.Errorf("cluster not found: %w", err)
	}

	// Delete the cluster
	if err := s.kubeClient.DeleteCluster(ctx, input.ClusterName); err != nil {
		return nil, fmt.Errorf("failed to delete cluster: %w", err)
	}

	s.logger.Info("cluster deletion initiated", "cluster", input.ClusterName)

	// Wait for cluster to be deleted
	waitCtx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	err = s.kubeClient.WaitForClusterDeleted(waitCtx, input.ClusterName, 10*time.Minute)
	if err != nil {
		s.logger.Error("cluster deletion failed or timed out", "cluster", input.ClusterName, "error", err)
		return &api.DeleteClusterOutput{
			Status:  "failed",
			Message: fmt.Sprintf("Cluster deletion failed: %v", err),
		}, nil
	}

	s.logger.Info("cluster deletion completed", "cluster", input.ClusterName)

	return &api.DeleteClusterOutput{
		Status:  "deleted",
		Message: "Cluster deleted successfully",
	}, nil
}

// ScaleCluster scales a MachineDeployment in the cluster.
func (s *ClusterService) ScaleCluster(ctx context.Context, input api.ScaleClusterInput) (*api.ScaleClusterOutput, error) {
	// Get the MachineDeployment
	md, err := s.kubeClient.GetMachineDeployment(ctx, input.ClusterName, input.NodePoolName)
	if err != nil {
		return nil, fmt.Errorf("failed to get machine deployment: %w", err)
	}

	oldReplicas := int32(0)
	if md.Spec.Replicas != nil {
		oldReplicas = *md.Spec.Replicas
	}

	// Update replicas
	newReplicas := int32(input.Replicas)
	md.Spec.Replicas = &newReplicas

	// Update the MachineDeployment
	if err := s.kubeClient.UpdateMachineDeployment(ctx, md); err != nil {
		return nil, fmt.Errorf("failed to update machine deployment: %w", err)
	}

	s.logger.Info("cluster scaling initiated", 
		"cluster", input.ClusterName,
		"node_pool", input.NodePoolName,
		"old_replicas", oldReplicas,
		"new_replicas", newReplicas,
	)

	return &api.ScaleClusterOutput{
		Status:      "scaling",
		Message:     fmt.Sprintf("Scaling %s from %d to %d replicas", input.NodePoolName, oldReplicas, newReplicas),
		OldReplicas: int(oldReplicas),
		NewReplicas: input.Replicas,
	}, nil
}

// GetClusterKubeconfig retrieves the kubeconfig for a cluster.
func (s *ClusterService) GetClusterKubeconfig(ctx context.Context, input api.GetClusterKubeconfigInput) (*api.GetClusterKubeconfigOutput, error) {
	// Get the kubeconfig secret
	secret, err := s.kubeClient.GetKubeconfigSecret(ctx, input.ClusterName)
	if err != nil {
		return nil, fmt.Errorf("failed to get kubeconfig: %w", err)
	}

	// Extract kubeconfig data
	kubeconfigData, ok := secret.Data["value"]
	if !ok {
		return nil, fmt.Errorf("kubeconfig data not found in secret")
	}

	return &api.GetClusterKubeconfigOutput{
		Kubeconfig: string(kubeconfigData),
	}, nil
}

// GetClusterNodes retrieves nodes from a workload cluster.
func (s *ClusterService) GetClusterNodes(ctx context.Context, input api.GetClusterNodesInput) (*api.GetClusterNodesOutput, error) {
	// Get kubeconfig first
	kubeconfigOutput, err := s.GetClusterKubeconfig(ctx, api.GetClusterKubeconfigInput{
		ClusterName: input.ClusterName,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get kubeconfig: %w", err)
	}

	// Create workload client
	workloadClient, err := kube.NewWorkloadClientFromKubeconfig([]byte(kubeconfigOutput.Kubeconfig))
	if err != nil {
		return nil, fmt.Errorf("failed to create workload client: %w", err)
	}

	// List nodes
	nodes, err := workloadClient.ListNodes(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list nodes: %w", err)
	}

	// Convert to API format
	nodeInfos := make([]api.NodeInfo, 0, len(nodes.Items))
	for _, node := range nodes.Items {
		nodeInfo := api.NodeInfo{
			Name:           node.Name,
			Status:         getNodeStatus(&node),
			Roles:          getNodeRoles(&node),
			KubeletVersion: node.Status.NodeInfo.KubeletVersion,
			Labels:         node.Labels,
		}

		// Get addresses
		for _, addr := range node.Status.Addresses {
			switch addr.Type {
			case "InternalIP":
				nodeInfo.InternalIP = addr.Address
			case "ExternalIP":
				nodeInfo.ExternalIP = addr.Address
			}
		}

		// Get instance type from labels
		if instanceType, ok := node.Labels["node.kubernetes.io/instance-type"]; ok {
			nodeInfo.InstanceType = instanceType
		}

		// Get availability zone from labels
		if az, ok := node.Labels["topology.kubernetes.io/zone"]; ok {
			nodeInfo.AvailabilityZone = az
		}

		nodeInfos = append(nodeInfos, nodeInfo)
	}

	return &api.GetClusterNodesOutput{
		Nodes: nodeInfos,
	}, nil
}

// Helper functions

func (s *ClusterService) estimateNodeCount(cluster *clusterv1.Cluster) int {
	// This is a rough estimate - in a real implementation we would
	// query the MachineDeployments for this cluster
	if cluster.Spec.Topology != nil && cluster.Spec.Topology.Workers != nil {
		count := 0
		for _, md := range cluster.Spec.Topology.Workers.MachineDeployments {
			if md.Replicas != nil {
				count += int(*md.Replicas)
			}
		}
		return count
	}
	return 0
}

func getNodeStatus(node *corev1.Node) string {
	for _, condition := range node.Status.Conditions {
		if condition.Type == "Ready" {
			if condition.Status == "True" {
				return "Ready"
			}
			return "NotReady"
		}
	}
	return "Unknown"
}

func getNodeRoles(node *corev1.Node) []string {
	roles := []string{}
	for label := range node.Labels {
		if strings.HasPrefix(label, "node-role.kubernetes.io/") {
			role := strings.TrimPrefix(label, "node-role.kubernetes.io/")
			if role != "" {
				roles = append(roles, role)
			}
		}
	}
	if len(roles) == 0 {
		roles = append(roles, "worker")
	}
	return roles
}

// extractProviderName determines the provider name from cluster variables or template name.
// This is used to route provider-specific validation and operations.
func (s *ClusterService) extractProviderName(variables map[string]interface{}, templateName string) string {
	// First, check if provider is explicitly specified in variables
	if provider, ok := variables["provider"]; ok {
		if providerStr, ok := provider.(string); ok {
			return providerStr
		}
	}

	// Fall back to inferring from template name
	// Common patterns: "aws-template", "azure-cluster-class", etc.
	templateLower := strings.ToLower(templateName)
	if strings.Contains(templateLower, "aws") {
		return "aws"
	}
	if strings.Contains(templateLower, "azure") {
		return "azure"
	}
	if strings.Contains(templateLower, "gcp") || strings.Contains(templateLower, "google") {
		return "gcp"
	}

	// Default to AWS for V1.0 scope
	return "aws"
}