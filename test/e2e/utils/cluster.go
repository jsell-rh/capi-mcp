package utils

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ClusterUtil provides utilities for managing CAPI clusters in E2E tests
type ClusterUtil struct {
	client client.Client
	logger *slog.Logger
}

// NewClusterUtil creates a new ClusterUtil instance
func NewClusterUtil(kubeClient client.Client, logger *slog.Logger) (*ClusterUtil, error) {
	if kubeClient == nil {
		return nil, fmt.Errorf("kubernetes client cannot be nil")
	}
	
	if logger == nil {
		return nil, fmt.Errorf("logger cannot be nil")
	}
	
	return &ClusterUtil{
		client: kubeClient,
		logger: logger,
	}, nil
}

// ClusterInfo represents information about a cluster
type ClusterInfo struct {
	Name                string
	Namespace           string
	Phase               string
	ControlPlaneReady   bool
	InfrastructureReady bool
	KubernetesVersion   string
	ControlPlaneNodes   int32
	WorkerNodes         int32
	CreationTimestamp   time.Time
}

// WaitForClusterReady waits for a cluster to reach the Provisioned phase
func (c *ClusterUtil) WaitForClusterReady(ctx context.Context, clusterName, namespace string, timeout time.Duration) error {
	c.logger.Info("Waiting for cluster to be ready",
		"cluster", clusterName,
		"namespace", namespace,
		"timeout", timeout)
	
	return wait.PollUntilContextTimeout(ctx, 30*time.Second, timeout, false, func(ctx context.Context) (bool, error) {
		cluster, err := c.GetCluster(ctx, clusterName, namespace)
		if err != nil {
			c.logger.Warn("Failed to get cluster while waiting", "error", err)
			return false, nil // Continue polling
		}
		
		c.logger.Debug("Cluster status check",
			"cluster", clusterName,
			"phase", cluster.Phase,
			"controlPlaneReady", cluster.ControlPlaneReady,
			"infrastructureReady", cluster.InfrastructureReady)
		
		// Check if cluster is ready
		if cluster.Phase == string(clusterv1.ClusterPhaseProvisioned) &&
			cluster.ControlPlaneReady &&
			cluster.InfrastructureReady {
			c.logger.Info("Cluster is ready", "cluster", clusterName)
			return true, nil
		}
		
		// Check if cluster failed
		if cluster.Phase == string(clusterv1.ClusterPhaseFailed) {
			return false, fmt.Errorf("cluster failed to provision: %s", clusterName)
		}
		
		return false, nil // Continue polling
	})
}

// WaitForClusterDeleted waits for a cluster to be completely deleted
func (c *ClusterUtil) WaitForClusterDeleted(ctx context.Context, clusterName, namespace string, timeout time.Duration) error {
	c.logger.Info("Waiting for cluster to be deleted",
		"cluster", clusterName,
		"namespace", namespace,
		"timeout", timeout)
	
	return wait.PollUntilContextTimeout(ctx, 10*time.Second, timeout, false, func(ctx context.Context) (bool, error) {
		cluster := &clusterv1.Cluster{}
		err := c.client.Get(ctx, types.NamespacedName{
			Name:      clusterName,
			Namespace: namespace,
		}, cluster)
		
		if apierrors.IsNotFound(err) {
			c.logger.Info("Cluster has been deleted", "cluster", clusterName)
			return true, nil
		}
		
		if err != nil {
			c.logger.Warn("Error checking cluster deletion status", "error", err)
			return false, nil // Continue polling
		}
		
		c.logger.Debug("Cluster still exists, continuing to wait", "cluster", clusterName)
		return false, nil
	})
}

// GetCluster retrieves cluster information
func (c *ClusterUtil) GetCluster(ctx context.Context, clusterName, namespace string) (*ClusterInfo, error) {
	cluster := &clusterv1.Cluster{}
	err := c.client.Get(ctx, types.NamespacedName{
		Name:      clusterName,
		Namespace: namespace,
	}, cluster)
	
	if err != nil {
		return nil, fmt.Errorf("failed to get cluster %s/%s: %w", namespace, clusterName, err)
	}
	
	return &ClusterInfo{
		Name:                cluster.Name,
		Namespace:           cluster.Namespace,
		Phase:               cluster.Status.Phase,
		ControlPlaneReady:   cluster.Status.ControlPlaneReady,
		InfrastructureReady: cluster.Status.InfrastructureReady,
		KubernetesVersion:   cluster.Spec.Topology.Version,
		CreationTimestamp:   cluster.CreationTimestamp.Time,
	}, nil
}

// ListClusters lists all clusters in the specified namespace
func (c *ClusterUtil) ListClusters(ctx context.Context, namespace string) ([]*ClusterInfo, error) {
	clusterList := &clusterv1.ClusterList{}
	err := c.client.List(ctx, clusterList, client.InNamespace(namespace))
	if err != nil {
		return nil, fmt.Errorf("failed to list clusters in namespace %s: %w", namespace, err)
	}
	
	var clusters []*ClusterInfo
	for _, cluster := range clusterList.Items {
		clusterInfo := &ClusterInfo{
			Name:                cluster.Name,
			Namespace:           cluster.Namespace,
			Phase:               cluster.Status.Phase,
			ControlPlaneReady:   cluster.Status.ControlPlaneReady,
			InfrastructureReady: cluster.Status.InfrastructureReady,
			KubernetesVersion:   cluster.Spec.Topology.Version,
			CreationTimestamp:   cluster.CreationTimestamp.Time,
		}
		clusters = append(clusters, clusterInfo)
	}
	
	return clusters, nil
}

// GetMachineDeployments gets all MachineDeployments for a cluster
func (c *ClusterUtil) GetMachineDeployments(ctx context.Context, clusterName, namespace string) ([]clusterv1.MachineDeployment, error) {
	mdList := &clusterv1.MachineDeploymentList{}
	
	// List MachineDeployments with cluster label
	err := c.client.List(ctx, mdList, 
		client.InNamespace(namespace),
		client.MatchingLabels{clusterv1.ClusterNameLabel: clusterName})
	
	if err != nil {
		return nil, fmt.Errorf("failed to list MachineDeployments for cluster %s/%s: %w", namespace, clusterName, err)
	}
	
	return mdList.Items, nil
}

// WaitForMachineDeploymentReady waits for a MachineDeployment to have the desired replica count
func (c *ClusterUtil) WaitForMachineDeploymentReady(ctx context.Context, mdName, namespace string, expectedReplicas int32, timeout time.Duration) error {
	c.logger.Info("Waiting for MachineDeployment to be ready",
		"machineDeployment", mdName,
		"namespace", namespace,
		"expectedReplicas", expectedReplicas,
		"timeout", timeout)
	
	return wait.PollUntilContextTimeout(ctx, 15*time.Second, timeout, false, func(ctx context.Context) (bool, error) {
		md := &clusterv1.MachineDeployment{}
		err := c.client.Get(ctx, types.NamespacedName{
			Name:      mdName,
			Namespace: namespace,
		}, md)
		
		if err != nil {
			c.logger.Warn("Failed to get MachineDeployment while waiting", "error", err)
			return false, nil
		}
		
		c.logger.Debug("MachineDeployment status check",
			"machineDeployment", mdName,
			"replicas", md.Spec.Replicas,
			"readyReplicas", md.Status.ReadyReplicas,
			"updatedReplicas", md.Status.UpdatedReplicas)
		
		// Check if the desired number of replicas are ready
		if md.Status.ReadyReplicas == expectedReplicas &&
			md.Status.UpdatedReplicas == expectedReplicas {
			c.logger.Info("MachineDeployment is ready", 
				"machineDeployment", mdName,
				"replicas", expectedReplicas)
			return true, nil
		}
		
		return false, nil
	})
}

// GetClusterKubeconfig retrieves the kubeconfig secret for a cluster
func (c *ClusterUtil) GetClusterKubeconfig(ctx context.Context, clusterName, namespace string) ([]byte, error) {
	secretName := fmt.Sprintf("%s-kubeconfig", clusterName)
	
	secret := &corev1.Secret{}
	err := c.client.Get(ctx, types.NamespacedName{
		Name:      secretName,
		Namespace: namespace,
	}, secret)
	
	if err != nil {
		return nil, fmt.Errorf("failed to get kubeconfig secret %s/%s: %w", namespace, secretName, err)
	}
	
	kubeconfigData, exists := secret.Data["value"]
	if !exists {
		return nil, fmt.Errorf("kubeconfig data not found in secret %s/%s", namespace, secretName)
	}
	
	return kubeconfigData, nil
}

// ValidateClusterDeleted verifies that a cluster and its resources are completely removed
func (c *ClusterUtil) ValidateClusterDeleted(ctx context.Context, clusterName, namespace string) error {
	c.logger.Info("Validating cluster deletion", "cluster", clusterName, "namespace", namespace)
	
	// Check that cluster no longer exists
	cluster := &clusterv1.Cluster{}
	err := c.client.Get(ctx, types.NamespacedName{
		Name:      clusterName,
		Namespace: namespace,
	}, cluster)
	
	if !apierrors.IsNotFound(err) {
		if err != nil {
			return fmt.Errorf("unexpected error checking cluster existence: %w", err)
		}
		return fmt.Errorf("cluster %s still exists", clusterName)
	}
	
	// Check that MachineDeployments are deleted
	mdList := &clusterv1.MachineDeploymentList{}
	err = c.client.List(ctx, mdList,
		client.InNamespace(namespace),
		client.MatchingLabels{clusterv1.ClusterNameLabel: clusterName})
	
	if err != nil {
		return fmt.Errorf("failed to list MachineDeployments: %w", err)
	}
	
	if len(mdList.Items) > 0 {
		return fmt.Errorf("found %d MachineDeployments that should have been deleted", len(mdList.Items))
	}
	
	// Check that kubeconfig secret is deleted
	kubeconfigSecretName := fmt.Sprintf("%s-kubeconfig", clusterName)
	secret := &corev1.Secret{}
	err = c.client.Get(ctx, types.NamespacedName{
		Name:      kubeconfigSecretName,
		Namespace: namespace,
	}, secret)
	
	if !apierrors.IsNotFound(err) {
		if err != nil {
			return fmt.Errorf("unexpected error checking kubeconfig secret: %w", err)
		}
		return fmt.Errorf("kubeconfig secret %s still exists", kubeconfigSecretName)
	}
	
	c.logger.Info("Cluster deletion validated successfully", "cluster", clusterName)
	return nil
}

// GetClusterClasses lists available ClusterClass resources
func (c *ClusterUtil) GetClusterClasses(ctx context.Context, namespace string) ([]clusterv1.ClusterClass, error) {
	ccList := &clusterv1.ClusterClassList{}
	err := c.client.List(ctx, ccList, client.InNamespace(namespace))
	if err != nil {
		return nil, fmt.Errorf("failed to list ClusterClasses in namespace %s: %w", namespace, err)
	}
	
	return ccList.Items, nil
}

// GetDefaultClusterClass returns the first available ClusterClass (for testing)
func (c *ClusterUtil) GetDefaultClusterClass(ctx context.Context, namespace string) (*clusterv1.ClusterClass, error) {
	classes, err := c.GetClusterClasses(ctx, namespace)
	if err != nil {
		return nil, err
	}
	
	if len(classes) == 0 {
		return nil, fmt.Errorf("no ClusterClasses found in namespace %s", namespace)
	}
	
	return &classes[0], nil
}