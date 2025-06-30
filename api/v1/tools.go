package v1

// ListClustersInput defines the parameters for the list_clusters tool.
type ListClustersInput struct{}

// ListClustersOutput defines the response for the list_clusters tool.
type ListClustersOutput struct {
	Clusters []ClusterSummary `json:"clusters"`
}

// ClusterSummary provides basic information about a cluster.
type ClusterSummary struct {
	Name            string `json:"name"`
	Namespace       string `json:"namespace"`
	Provider        string `json:"provider"`
	KubernetesVersion string `json:"kubernetes_version"`
	Status          string `json:"status"`
	CreatedAt       string `json:"created_at"`
	NodeCount       int    `json:"node_count"`
}

// GetClusterInput defines the parameters for the get_cluster tool.
type GetClusterInput struct {
	ClusterName string `json:"cluster_name" validate:"required"`
}

// GetClusterOutput defines the response for the get_cluster tool.
type GetClusterOutput struct {
	Cluster ClusterDetails `json:"cluster"`
}

// ClusterDetails provides detailed information about a cluster.
type ClusterDetails struct {
	Name              string                 `json:"name"`
	Namespace         string                 `json:"namespace"`
	Provider          string                 `json:"provider"`
	Region            string                 `json:"region"`
	KubernetesVersion string                 `json:"kubernetes_version"`
	Status            string                 `json:"status"`
	CreatedAt         string                 `json:"created_at"`
	Endpoint          string                 `json:"endpoint"`
	NodePools         []NodePool             `json:"node_pools"`
	Conditions        []ClusterCondition     `json:"conditions"`
	InfrastructureRef map[string]interface{} `json:"infrastructure_ref"`
}

// NodePool represents a group of nodes in a cluster.
type NodePool struct {
	Name          string `json:"name"`
	Replicas      int    `json:"replicas"`
	ReadyReplicas int    `json:"ready_replicas"`
	MachineType   string `json:"machine_type"`
}

// ClusterCondition represents a condition of a cluster.
type ClusterCondition struct {
	Type               string `json:"type"`
	Status             string `json:"status"`
	LastTransitionTime string `json:"last_transition_time"`
	Reason             string `json:"reason"`
	Message            string `json:"message"`
}

// CreateClusterInput defines the parameters for the create_cluster tool.
type CreateClusterInput struct {
	ClusterName       string                 `json:"cluster_name" validate:"required"`
	TemplateName      string                 `json:"template_name" validate:"required"`
	KubernetesVersion string                 `json:"kubernetes_version" validate:"required"`
	Variables         map[string]interface{} `json:"variables,omitempty"`
}

// CreateClusterOutput defines the response for the create_cluster tool.
type CreateClusterOutput struct {
	ClusterName string `json:"cluster_name"`
	Status      string `json:"status"`
	Message     string `json:"message"`
}

// DeleteClusterInput defines the parameters for the delete_cluster tool.
type DeleteClusterInput struct {
	ClusterName string `json:"cluster_name" validate:"required"`
}

// DeleteClusterOutput defines the response for the delete_cluster tool.
type DeleteClusterOutput struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

// ScaleClusterInput defines the parameters for the scale_cluster tool.
type ScaleClusterInput struct {
	ClusterName  string `json:"cluster_name" validate:"required"`
	NodePoolName string `json:"node_pool_name" validate:"required"`
	Replicas     int    `json:"replicas" validate:"gte=0"`
}

// ScaleClusterOutput defines the response for the scale_cluster tool.
type ScaleClusterOutput struct {
	Status      string `json:"status"`
	Message     string `json:"message"`
	OldReplicas int    `json:"old_replicas"`
	NewReplicas int    `json:"new_replicas"`
}

// GetClusterKubeconfigInput defines the parameters for the get_cluster_kubeconfig tool.
type GetClusterKubeconfigInput struct {
	ClusterName string `json:"cluster_name" validate:"required"`
}

// GetClusterKubeconfigOutput defines the response for the get_cluster_kubeconfig tool.
type GetClusterKubeconfigOutput struct {
	Kubeconfig string `json:"kubeconfig"`
}

// GetClusterNodesInput defines the parameters for the get_cluster_nodes tool.
type GetClusterNodesInput struct {
	ClusterName string `json:"cluster_name" validate:"required"`
}

// GetClusterNodesOutput defines the response for the get_cluster_nodes tool.
type GetClusterNodesOutput struct {
	Nodes []NodeInfo `json:"nodes"`
}

// NodeInfo provides information about a node.
type NodeInfo struct {
	Name              string            `json:"name"`
	Status            string            `json:"status"`
	Roles             []string          `json:"roles"`
	KubeletVersion    string            `json:"kubelet_version"`
	InternalIP        string            `json:"internal_ip"`
	ExternalIP        string            `json:"external_ip,omitempty"`
	InstanceType      string            `json:"instance_type"`
	AvailabilityZone  string            `json:"availability_zone"`
	Labels            map[string]string `json:"labels"`
}