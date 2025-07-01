package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"

	api "github.com/capi-mcp/capi-mcp-server/api/v1"
	"github.com/capi-mcp/capi-mcp-server/internal/errors"
	"github.com/capi-mcp/capi-mcp-server/internal/kube"
	"github.com/capi-mcp/capi-mcp-server/internal/logging"
	"github.com/capi-mcp/capi-mcp-server/pkg/provider"
)

// EnhancedClusterService handles CAPI cluster operations with enhanced error handling and logging.
type EnhancedClusterService struct {
	kubeClient      *kube.Client
	logger          *logging.Logger
	providerManager *provider.ProviderManager
}

// NewEnhancedClusterService creates a new cluster service with enhanced features.
func NewEnhancedClusterService(kubeClient *kube.Client, logger *logging.Logger, providerManager *provider.ProviderManager) *EnhancedClusterService {
	return &EnhancedClusterService{
		kubeClient:      kubeClient,
		logger:          logger.WithComponent("cluster-service"),
		providerManager: providerManager,
	}
}

// ListClusters returns a summary of all clusters with enhanced error handling.
func (s *EnhancedClusterService) ListClusters(ctx context.Context) (*api.ListClustersOutput, error) {
	logger := s.logger.WithContext(ctx).WithOperation("ListClusters")
	logger.Debug("Listing all clusters")
	
	// Check if kube client is available
	if s.kubeClient == nil {
		logger.Warn("Kubernetes client not initialized")
		return &api.ListClustersOutput{Clusters: []api.ClusterSummary{}}, nil
	}
	
	// List clusters with timeout
	listCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	
	clusters, err := s.kubeClient.ListClusters(listCtx)
	if err != nil {
		logger.WithError(err).Error("Failed to list clusters from Kubernetes API")
		
		// Check if it's a timeout
		if errors.IsTimeout(err) {
			return nil, errors.Wrap(err, errors.CodeTimeout, "timeout listing clusters")
		}
		
		// Check if it's an auth error
		if apierrors.IsUnauthorized(err) || apierrors.IsForbidden(err) {
			return nil, errors.Wrap(err, errors.CodeUnauthorized, "unauthorized to list clusters")
		}
		
		return nil, errors.Wrap(err, errors.CodeKubernetesAPI, "failed to list clusters")
	}
	
	summaries := make([]api.ClusterSummary, 0, len(clusters.Items))
	for _, cluster := range clusters.Items {
		summary := api.ClusterSummary{
			Name:              cluster.Name,
			Namespace:         cluster.Namespace,
			Status:            s.normalizeClusterStatus(cluster.Status.Phase),
			CreatedAt:         cluster.CreationTimestamp.Format(time.RFC3339),
			KubernetesVersion: "",
			NodeCount:         0,
		}
		
		// Extract Kubernetes version safely
		if cluster.Spec.Topology != nil {
			summary.KubernetesVersion = cluster.Spec.Topology.Version
		}
		
		// Count nodes by listing MachineDeployments
		nodeCount, err := s.getClusterNodeCount(listCtx, cluster.Name, cluster.Namespace)
		if err != nil {
			logger.WithError(err).Warn("Failed to get node count for cluster",
				logging.FieldClusterName, cluster.Name,
			)
			// Continue without node count
		} else {
			summary.NodeCount = nodeCount
		}
		
		summaries = append(summaries, summary)
	}
	
	logger.Info("Listed clusters successfully", "count", len(summaries))
	return &api.ListClustersOutput{Clusters: summaries}, nil
}

// GetCluster returns detailed information about a specific cluster.
func (s *EnhancedClusterService) GetCluster(ctx context.Context, input api.GetClusterInput) (*api.GetClusterOutput, error) {
	logger := s.logger.WithContext(ctx).WithOperation("GetCluster").WithCluster(input.ClusterName, "")
	logger.Debug("Getting cluster details")
	
	// Validate input
	if input.ClusterName == "" {
		err := errors.New(errors.CodeInvalidInput, "cluster name is required")
		logger.WithError(err).Error("Invalid input")
		return nil, err
	}
	
	// Check if kube client is available
	if s.kubeClient == nil {
		err := errors.New(errors.CodeUnavailable, "Kubernetes client not initialized")
		logger.WithError(err).Error("Service unavailable")
		return nil, err
	}
	
	// Get cluster with timeout
	getCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	
	cluster, err := s.kubeClient.GetCluster(getCtx, input.ClusterName)
	if err != nil {
		logger.WithError(err).Error("Failed to get cluster")
		
		if apierrors.IsNotFound(err) {
			return nil, errors.New(errors.CodeNotFound, fmt.Sprintf("cluster '%s' not found", input.ClusterName))
		}
		
		if errors.IsTimeout(err) {
			return nil, errors.Wrap(err, errors.CodeTimeout, "timeout getting cluster")
		}
		
		return nil, errors.Wrap(err, errors.CodeKubernetesAPI, "failed to get cluster")
	}
	
	// Build response
	output := &api.GetClusterOutput{
		Cluster: api.ClusterDetails{
			Name:      cluster.Name,
			Namespace: cluster.Namespace,
			Status: api.ClusterStatus{
				Phase:               s.normalizeClusterStatus(cluster.Status.Phase),
				ControlPlaneReady:   cluster.Status.ControlPlaneReady,
				InfrastructureReady: cluster.Status.InfrastructureReady,
			},
			Spec: api.ClusterSpec{
				ClusterClass:      s.getClusterClass(cluster),
				KubernetesVersion: s.getKubernetesVersion(cluster),
				ControlPlane: api.ControlPlaneSpec{
					Replicas: s.getControlPlaneReplicas(cluster),
				},
			},
			CreatedAt: cluster.CreationTimestamp.Format(time.RFC3339),
		},
	}
	
	// Add provider-specific status if available
	providerStatus, err := s.getProviderStatus(getCtx, cluster)
	if err != nil {
		logger.WithError(err).Warn("Failed to get provider status")
		// Continue without provider status
	} else if providerStatus != nil {
		output.ProviderStatus = providerStatus
	}
	
	logger.Info("Retrieved cluster successfully")
	return output, nil
}

// CreateCluster creates a new cluster from a template.
func (s *EnhancedClusterService) CreateCluster(ctx context.Context, input api.CreateClusterInput) (*api.CreateClusterOutput, error) {
	logger := s.logger.WithContext(ctx).WithOperation("CreateCluster").WithCluster(input.ClusterName, "")
	logger.Info("Creating new cluster",
		"template", input.TemplateName,
		"kubernetes_version", input.KubernetesVersion,
	)
	
	// Validate input
	if err := s.validateCreateClusterInput(input); err != nil {
		logger.WithError(err).Error("Invalid input")
		return nil, err
	}
	
	// Check if kube client is available
	if s.kubeClient == nil {
		err := errors.New(errors.CodeUnavailable, "Kubernetes client not initialized")
		logger.WithError(err).Error("Service unavailable")
		return nil, err
	}
	
	// Extract provider name and validate with provider
	providerName := s.extractProviderName(input.Variables, input.TemplateName)
	if s.providerManager != nil {
		if prov, exists := s.providerManager.GetProvider(providerName); exists {
			logger.Debug("Validating cluster configuration with provider", "provider", providerName)
			if err := prov.ValidateClusterConfig(ctx, input.Variables); err != nil {
				logger.WithError(err).Error("Provider validation failed")
				return nil, errors.Wrap(err, errors.CodeProviderValidation, "provider validation failed")
			}
		}
	}
	
	// Get ClusterClass
	clusterClass, err := s.kubeClient.GetClusterClass(ctx, input.TemplateName)
	if err != nil {
		logger.WithError(err).Error("Failed to get ClusterClass")
		if apierrors.IsNotFound(err) {
			return nil, errors.New(errors.CodeNotFound, fmt.Sprintf("cluster template '%s' not found", input.TemplateName))
		}
		return nil, errors.Wrap(err, errors.CodeKubernetesAPI, "failed to get cluster template")
	}
	
	// Check if cluster already exists
	existingCluster, err := s.kubeClient.GetCluster(ctx, input.ClusterName)
	if err == nil && existingCluster != nil {
		err := errors.New(errors.CodeAlreadyExists, fmt.Sprintf("cluster '%s' already exists", input.ClusterName))
		logger.WithError(err).Error("Cluster already exists")
		return nil, err
	}
	
	// Create cluster resource
	cluster := s.buildClusterResource(input, clusterClass)
	
	logger.Info("Creating cluster resource in Kubernetes")
	createdCluster, err := s.kubeClient.CreateCluster(ctx, cluster)
	if err != nil {
		logger.WithError(err).Error("Failed to create cluster resource")
		
		if apierrors.IsAlreadyExists(err) {
			return nil, errors.New(errors.CodeAlreadyExists, fmt.Sprintf("cluster '%s' already exists", input.ClusterName))
		}
		
		return nil, errors.Wrap(err, errors.CodeKubernetesAPI, "failed to create cluster")
	}
	
	// Wait for initial status
	logger.Debug("Waiting for cluster initial status")
	finalCluster, err := s.waitForClusterPhase(ctx, createdCluster.Name, createdCluster.Namespace, 2*time.Minute)
	if err != nil {
		logger.WithError(err).Warn("Failed to wait for cluster phase")
		// Return created cluster anyway
		finalCluster = createdCluster
	}
	
	output := &api.CreateClusterOutput{
		Success: true,
		Cluster: api.ClusterSummary{
			Name:              finalCluster.Name,
			Namespace:         finalCluster.Namespace,
			Status:            s.normalizeClusterStatus(finalCluster.Status.Phase),
			CreatedAt:         finalCluster.CreationTimestamp.Format(time.RFC3339),
			KubernetesVersion: input.KubernetesVersion,
		},
		Message: fmt.Sprintf("Cluster '%s' creation initiated successfully", input.ClusterName),
	}
	
	logger.Info("Cluster created successfully",
		"phase", finalCluster.Status.Phase,
		logging.FieldDuration, time.Since(finalCluster.CreationTimestamp.Time).Milliseconds(),
	)
	
	return output, nil
}

// Helper methods

// normalizeClusterStatus converts CAPI phase to a consistent status string
func (s *EnhancedClusterService) normalizeClusterStatus(phase string) string {
	if phase == "" {
		return "Unknown"
	}
	
	// Normalize common phases
	switch strings.ToLower(phase) {
	case "provisioning":
		return "Provisioning"
	case "provisioned":
		return "Ready"
	case "failed":
		return "Failed"
	case "deleting":
		return "Deleting"
	default:
		return phase
	}
}

// validateCreateClusterInput validates the create cluster input
func (s *EnhancedClusterService) validateCreateClusterInput(input api.CreateClusterInput) error {
	if input.ClusterName == "" {
		return errors.New(errors.CodeInvalidInput, "cluster name is required")
	}
	
	if input.TemplateName == "" {
		return errors.New(errors.CodeInvalidInput, "template name is required")
	}
	
	if input.KubernetesVersion == "" {
		return errors.New(errors.CodeInvalidInput, "kubernetes version is required")
	}
	
	// Validate cluster name format
	if !isValidClusterName(input.ClusterName) {
		return errors.New(errors.CodeInvalidInput, "cluster name must be a valid DNS subdomain")
	}
	
	return nil
}

// isValidClusterName checks if the cluster name is a valid DNS subdomain
func isValidClusterName(name string) bool {
	if len(name) == 0 || len(name) > 63 {
		return false
	}
	
	// Must start and end with alphanumeric
	if !isAlphaNumeric(name[0]) || !isAlphaNumeric(name[len(name)-1]) {
		return false
	}
	
	// Check all characters
	for _, ch := range name {
		if !isAlphaNumeric(byte(ch)) && ch != '-' {
			return false
		}
	}
	
	return true
}

func isAlphaNumeric(ch byte) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9')
}

// getClusterNodeCount counts the total nodes in a cluster
func (s *EnhancedClusterService) getClusterNodeCount(ctx context.Context, clusterName, namespace string) (int32, error) {
	machineDeployments, err := s.kubeClient.ListMachineDeployments(ctx, clusterName)
	if err != nil {
		return 0, err
	}
	
	var totalNodes int32
	for _, md := range machineDeployments.Items {
		if md.Spec.Replicas != nil {
			totalNodes += *md.Spec.Replicas
		}
	}
	
	// Add control plane nodes (assuming single control plane for now)
	totalNodes += 1
	
	return totalNodes, nil
}

// DeleteCluster deletes a cluster with enhanced error handling.
func (s *EnhancedClusterService) DeleteCluster(ctx context.Context, input api.DeleteClusterInput) (*api.DeleteClusterOutput, error) {
	logger := s.logger.WithContext(ctx).WithOperation("DeleteCluster").WithCluster(input.ClusterName, "")
	logger.Info("Deleting cluster")
	
	// Validate input
	if input.ClusterName == "" {
		err := errors.New(errors.CodeInvalidInput, "cluster name is required")
		logger.WithError(err).Error("Invalid input")
		return nil, err
	}
	
	// Check if kube client is available
	if s.kubeClient == nil {
		err := errors.New(errors.CodeUnavailable, "Kubernetes client not initialized")
		logger.WithError(err).Error("Service unavailable")
		return nil, err
	}
	
	// Check if cluster exists first
	deleteCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	
	cluster, err := s.kubeClient.GetCluster(deleteCtx, input.ClusterName)
	if err != nil {
		logger.WithError(err).Error("Failed to get cluster before deletion")
		if apierrors.IsNotFound(err) {
			return nil, errors.New(errors.CodeNotFound, fmt.Sprintf("cluster '%s' not found", input.ClusterName))
		}
		return nil, errors.Wrap(err, errors.CodeKubernetesAPI, "failed to verify cluster exists")
	}
	
	// Delete the cluster
	logger.Info("Deleting cluster resource from Kubernetes")
	if err := s.kubeClient.DeleteCluster(deleteCtx, input.ClusterName); err != nil {
		logger.WithError(err).Error("Failed to delete cluster resource")
		return nil, errors.Wrap(err, errors.CodeKubernetesAPI, "failed to delete cluster")
	}
	
	// Wait for deletion to complete (with timeout)
	logger.Debug("Waiting for cluster deletion to complete")
	waitCtx, waitCancel := context.WithTimeout(ctx, 10*time.Minute)
	defer waitCancel()
	
	err = s.waitForClusterDeleted(waitCtx, input.ClusterName, cluster.Namespace)
	if err != nil {
		logger.WithError(err).Warn("Failed to wait for cluster deletion completion")
		// Return success anyway since deletion was initiated
		return &api.DeleteClusterOutput{
			Status:  "deleting",
			Message: fmt.Sprintf("Cluster '%s' deletion initiated (may still be in progress)", input.ClusterName),
		}, nil
	}
	
	logger.Info("Cluster deleted successfully")
	return &api.DeleteClusterOutput{
		Status:  "deleted",
		Message: fmt.Sprintf("Cluster '%s' deleted successfully", input.ClusterName),
	}, nil
}

// ScaleCluster scales a cluster's worker nodes with enhanced error handling.
func (s *EnhancedClusterService) ScaleCluster(ctx context.Context, input api.ScaleClusterInput) (*api.ScaleClusterOutput, error) {
	logger := s.logger.WithContext(ctx).WithOperation("ScaleCluster").WithCluster(input.ClusterName, "")
	logger.Info("Scaling cluster",
		"node_pool", input.NodePoolName,
		"target_replicas", input.Replicas,
	)
	
	// Validate input
	if input.ClusterName == "" {
		err := errors.New(errors.CodeInvalidInput, "cluster name is required")
		logger.WithError(err).Error("Invalid input")
		return nil, err
	}
	
	if input.NodePoolName == "" {
		err := errors.New(errors.CodeInvalidInput, "node pool name is required")
		logger.WithError(err).Error("Invalid input")
		return nil, err
	}
	
	if input.Replicas < 0 {
		err := errors.New(errors.CodeInvalidInput, "replica count cannot be negative")
		logger.WithError(err).Error("Invalid input")
		return nil, err
	}
	
	// Check if kube client is available
	if s.kubeClient == nil {
		err := errors.New(errors.CodeUnavailable, "Kubernetes client not initialized")
		logger.WithError(err).Error("Service unavailable")
		return nil, err
	}
	
	// Get MachineDeployment with timeout
	scaleCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	
	md, err := s.kubeClient.GetMachineDeployment(scaleCtx, input.ClusterName, input.NodePoolName)
	if err != nil {
		logger.WithError(err).Error("Failed to get MachineDeployment")
		if apierrors.IsNotFound(err) {
			return nil, errors.New(errors.CodeNotFound, fmt.Sprintf("node pool '%s' not found in cluster '%s'", input.NodePoolName, input.ClusterName))
		}
		return nil, errors.Wrap(err, errors.CodeKubernetesAPI, "failed to get node pool")
	}
	
	// Get current replica count
	oldReplicas := int32(0)
	if md.Spec.Replicas != nil {
		oldReplicas = *md.Spec.Replicas
	}
	
	newReplicas := int32(input.Replicas)
	
	// Check if scaling is needed
	if oldReplicas == newReplicas {
		logger.Info("No scaling needed - already at target replica count")
		return &api.ScaleClusterOutput{
			Status:      "ready",
			Message:     fmt.Sprintf("Node pool '%s' already has %d replicas", input.NodePoolName, input.Replicas),
			OldReplicas: int(oldReplicas),
			NewReplicas: input.Replicas,
		}, nil
	}
	
	// Update replica count
	md.Spec.Replicas = &newReplicas
	
	logger.Info("Updating MachineDeployment replica count",
		"old_replicas", oldReplicas,
		"new_replicas", newReplicas,
	)
	
	if err := s.kubeClient.UpdateMachineDeployment(scaleCtx, md); err != nil {
		logger.WithError(err).Error("Failed to update MachineDeployment")
		return nil, errors.Wrap(err, errors.CodeKubernetesAPI, "failed to scale node pool")
	}
	
	logger.Info("Cluster scaling initiated successfully")
	return &api.ScaleClusterOutput{
		Status:      "scaling",
		Message:     fmt.Sprintf("Scaling node pool '%s' from %d to %d replicas", input.NodePoolName, oldReplicas, newReplicas),
		OldReplicas: int(oldReplicas),
		NewReplicas: input.Replicas,
	}, nil
}

// GetClusterKubeconfig retrieves the kubeconfig for a cluster with enhanced error handling.
func (s *EnhancedClusterService) GetClusterKubeconfig(ctx context.Context, input api.GetClusterKubeconfigInput) (*api.GetClusterKubeconfigOutput, error) {
	logger := s.logger.WithContext(ctx).WithOperation("GetClusterKubeconfig").WithCluster(input.ClusterName, "")
	logger.Debug("Getting cluster kubeconfig")
	
	// Validate input
	if input.ClusterName == "" {
		err := errors.New(errors.CodeInvalidInput, "cluster name is required")
		logger.WithError(err).Error("Invalid input")
		return nil, err
	}
	
	// Check if kube client is available
	if s.kubeClient == nil {
		err := errors.New(errors.CodeUnavailable, "Kubernetes client not initialized")
		logger.WithError(err).Error("Service unavailable")
		return nil, err
	}
	
	// Get kubeconfig secret with timeout
	kubeconfigCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	
	secret, err := s.kubeClient.GetKubeconfigSecret(kubeconfigCtx, input.ClusterName)
	if err != nil {
		logger.WithError(err).Error("Failed to get kubeconfig secret")
		if apierrors.IsNotFound(err) {
			return nil, errors.New(errors.CodeNotFound, fmt.Sprintf("kubeconfig for cluster '%s' not found", input.ClusterName))
		}
		return nil, errors.Wrap(err, errors.CodeKubernetesAPI, "failed to get kubeconfig")
	}
	
	// Extract kubeconfig data
	kubeconfigData, ok := secret.Data["value"]
	if !ok {
		err := errors.New(errors.CodeInternal, "kubeconfig data not found in secret")
		logger.WithError(err).Error("Invalid kubeconfig secret format")
		return nil, err
	}
	
	// Validate kubeconfig is not empty
	if len(kubeconfigData) == 0 {
		err := errors.New(errors.CodeInternal, "kubeconfig data is empty")
		logger.WithError(err).Error("Empty kubeconfig")
		return nil, err
	}
	
	logger.Info("Retrieved kubeconfig successfully", "size_bytes", len(kubeconfigData))
	return &api.GetClusterKubeconfigOutput{
		Kubeconfig: string(kubeconfigData),
	}, nil
}

// GetClusterNodes retrieves nodes from a workload cluster with enhanced error handling.
func (s *EnhancedClusterService) GetClusterNodes(ctx context.Context, input api.GetClusterNodesInput) (*api.GetClusterNodesOutput, error) {
	logger := s.logger.WithContext(ctx).WithOperation("GetClusterNodes").WithCluster(input.ClusterName, "")
	logger.Debug("Getting cluster nodes")
	
	// Validate input
	if input.ClusterName == "" {
		err := errors.New(errors.CodeInvalidInput, "cluster name is required")
		logger.WithError(err).Error("Invalid input")
		return nil, err
	}
	
	// Check if kube client is available
	if s.kubeClient == nil {
		err := errors.New(errors.CodeUnavailable, "Kubernetes client not initialized")
		logger.WithError(err).Error("Service unavailable")
		return nil, err
	}
	
	// Get kubeconfig first
	nodesCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	
	kubeconfigOutput, err := s.GetClusterKubeconfig(nodesCtx, api.GetClusterKubeconfigInput{
		ClusterName: input.ClusterName,
	})
	if err != nil {
		logger.WithError(err).Error("Failed to get kubeconfig for workload cluster")
		return nil, errors.Wrap(err, errors.CodeDependencyFailure, "failed to get kubeconfig")
	}
	
	// Create workload client
	workloadClient, err := kube.NewWorkloadClientFromKubeconfig([]byte(kubeconfigOutput.Kubeconfig))
	if err != nil {
		logger.WithError(err).Error("Failed to create workload client")
		return nil, errors.Wrap(err, errors.CodeInternal, "failed to create workload cluster client")
	}
	
	// List nodes from workload cluster
	logger.Debug("Listing nodes from workload cluster")
	nodes, err := workloadClient.ListNodes(nodesCtx)
	if err != nil {
		logger.WithError(err).Error("Failed to list nodes from workload cluster")
		
		// Check for common errors
		if errors.IsTimeout(err) {
			return nil, errors.Wrap(err, errors.CodeTimeout, "timeout listing nodes from workload cluster")
		}
		
		return nil, errors.Wrap(err, errors.CodeWorkloadCluster, "failed to list nodes from workload cluster")
	}
	
	// Convert to API format
	nodeInfos := make([]api.NodeInfo, 0, len(nodes.Items))
	for _, node := range nodes.Items {
		nodeInfo := api.NodeInfo{
			Name:           node.Name,
			Status:         s.getNodeStatus(&node),
			Roles:          s.getNodeRoles(&node),
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
	
	logger.Info("Retrieved cluster nodes successfully", "node_count", len(nodeInfos))
	return &api.GetClusterNodesOutput{
		Nodes: nodeInfos,
	}, nil
}

// Helper methods

// getNodeStatus determines the status of a node
func (s *EnhancedClusterService) getNodeStatus(node *corev1.Node) string {
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

// getNodeRoles extracts the roles from node labels
func (s *EnhancedClusterService) getNodeRoles(node *corev1.Node) []string {
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

// extractProviderName determines the provider name from cluster variables or template name
func (s *EnhancedClusterService) extractProviderName(variables map[string]interface{}, templateName string) string {
	// First, check if provider is explicitly specified in variables
	if provider, ok := variables["provider"]; ok {
		if providerStr, ok := provider.(string); ok {
			return providerStr
		}
	}
	
	// Fall back to inferring from template name
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

// waitForClusterPhase waits for a cluster to reach a specific phase
func (s *EnhancedClusterService) waitForClusterPhase(ctx context.Context, clusterName, namespace string, timeout time.Duration) (*clusterv1.Cluster, error) {
	waitCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-waitCtx.Done():
			return nil, waitCtx.Err()
		case <-ticker.C:
			cluster, err := s.kubeClient.GetCluster(waitCtx, clusterName)
			if err != nil {
				continue // Keep trying
			}
			
			// Return cluster regardless of phase after initial creation
			if cluster.Status.Phase != "" {
				return cluster, nil
			}
		}
	}
}

// waitForClusterDeleted waits for a cluster to be fully deleted
func (s *EnhancedClusterService) waitForClusterDeleted(ctx context.Context, clusterName, namespace string) error {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			_, err := s.kubeClient.GetCluster(ctx, clusterName)
			if apierrors.IsNotFound(err) {
				return nil // Successfully deleted
			}
			if err != nil {
				continue // Keep trying on other errors
			}
			// Cluster still exists, continue waiting
		}
	}
}

// buildClusterResource builds a CAPI Cluster resource from the input
func (s *EnhancedClusterService) buildClusterResource(input api.CreateClusterInput, clusterClass *clusterv1.ClusterClass) *clusterv1.Cluster {
	cluster := &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: input.ClusterName,
			Labels: map[string]string{
				"cluster.x-k8s.io/cluster-name": input.ClusterName,
			},
		},
		Spec: clusterv1.ClusterSpec{
			Topology: &clusterv1.Topology{
				Class:   clusterClass.Name,
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
				// Skip invalid variables
				continue
			}
			variables = append(variables, clusterv1.ClusterVariable{
				Name:  name,
				Value: apiextensionsv1.JSON{Raw: rawValue},
			})
		}
		cluster.Spec.Topology.Variables = variables
	}
	
	return cluster
}

// getClusterClass safely extracts cluster class name
func (s *EnhancedClusterService) getClusterClass(cluster *clusterv1.Cluster) string {
	if cluster.Spec.Topology != nil {
		return cluster.Spec.Topology.Class
	}
	return ""
}

// getKubernetesVersion safely extracts Kubernetes version
func (s *EnhancedClusterService) getKubernetesVersion(cluster *clusterv1.Cluster) string {
	if cluster.Spec.Topology != nil {
		return cluster.Spec.Topology.Version
	}
	return ""
}

// getControlPlaneReplicas safely extracts control plane replica count
func (s *EnhancedClusterService) getControlPlaneReplicas(cluster *clusterv1.Cluster) int32 {
	// Default to 1 for single control plane
	// In a real implementation, we would check the control plane spec
	return 1
}

// getProviderStatus gets provider-specific status information
func (s *EnhancedClusterService) getProviderStatus(ctx context.Context, cluster *clusterv1.Cluster) (map[string]interface{}, error) {
	if s.providerManager == nil {
		return nil, nil
	}
	
	// Determine provider from cluster
	providerName := "aws" // Default for now
	if provider, ok := cluster.Labels["cluster.x-k8s.io/provider"]; ok {
		providerName = provider
	}
	
	// Get provider-specific status
	if prov, exists := s.providerManager.GetProvider(providerName); exists {
		return prov.GetProviderSpecificStatus(ctx, cluster)
	}
	
	return nil, nil
}