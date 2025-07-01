package kube

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	bootstrapv1 "sigs.k8s.io/cluster-api/bootstrap/kubeadm/api/v1beta1"
	controlplanev1 "sigs.k8s.io/cluster-api/controlplane/kubeadm/api/v1beta1"
	expv1 "sigs.k8s.io/cluster-api/exp/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Client wraps controller-runtime client for CAPI operations.
type Client struct {
	client    client.Client
	namespace string
}

// NewClient creates a new CAPI client wrapper.
func NewClient(kubeconfig string, namespace string) (*Client, error) {
	// Create the client configuration
	var config *rest.Config
	var err error
	
	if kubeconfig == "" {
		// Use in-cluster config when no kubeconfig is provided
		config, err = rest.InClusterConfig()
		if err != nil {
			return nil, fmt.Errorf("failed to get in-cluster config: %w", err)
		}
	} else {
		// Use kubeconfig file
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			return nil, fmt.Errorf("failed to build config from kubeconfig: %w", err)
		}
	}

	// Create a new scheme and add CAPI types
	sch := runtime.NewScheme()
	if err := scheme.AddToScheme(sch); err != nil {
		return nil, fmt.Errorf("failed to add Kubernetes types to scheme: %w", err)
	}
	if err := clusterv1.AddToScheme(sch); err != nil {
		return nil, fmt.Errorf("failed to add CAPI types to scheme: %w", err)
	}
	if err := bootstrapv1.AddToScheme(sch); err != nil {
		return nil, fmt.Errorf("failed to add bootstrap types to scheme: %w", err)
	}
	if err := controlplanev1.AddToScheme(sch); err != nil {
		return nil, fmt.Errorf("failed to add control plane types to scheme: %w", err)
	}
	if err := expv1.AddToScheme(sch); err != nil {
		return nil, fmt.Errorf("failed to add experimental types to scheme: %w", err)
	}

	// Create the client
	c, err := client.New(config, client.Options{Scheme: sch})
	if err != nil {
		return nil, fmt.Errorf("failed to create client: %w", err)
	}

	return &Client{
		client:    c,
		namespace: namespace,
	}, nil
}

// ListClusters returns all clusters in the namespace.
func (c *Client) ListClusters(ctx context.Context) (*clusterv1.ClusterList, error) {
	clusters := &clusterv1.ClusterList{}
	if err := c.client.List(ctx, clusters, client.InNamespace(c.namespace)); err != nil {
		return nil, fmt.Errorf("failed to list clusters: %w", err)
	}
	return clusters, nil
}

// GetClusterByName retrieves a cluster by name.
func (c *Client) GetClusterByName(ctx context.Context, name string) (*clusterv1.Cluster, error) {
	cluster := &clusterv1.Cluster{}
	key := types.NamespacedName{
		Namespace: c.namespace,
		Name:      name,
	}
	if err := c.client.Get(ctx, key, cluster); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, fmt.Errorf("cluster %s not found", name)
		}
		return nil, fmt.Errorf("failed to get cluster: %w", err)
	}
	return cluster, nil
}

// CreateCluster creates a new cluster.
func (c *Client) CreateCluster(ctx context.Context, cluster *clusterv1.Cluster) error {
	cluster.Namespace = c.namespace
	if err := c.client.Create(ctx, cluster); err != nil {
		return fmt.Errorf("failed to create cluster: %w", err)
	}
	return nil
}

// DeleteCluster deletes a cluster.
func (c *Client) DeleteCluster(ctx context.Context, name string) error {
	cluster := &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: c.namespace,
		},
	}
	if err := c.client.Delete(ctx, cluster); err != nil {
		if apierrors.IsNotFound(err) {
			return fmt.Errorf("cluster %s not found", name)
		}
		return fmt.Errorf("failed to delete cluster: %w", err)
	}
	return nil
}

// GetMachineDeployment retrieves a MachineDeployment by cluster and name.
func (c *Client) GetMachineDeployment(ctx context.Context, clusterName, mdName string) (*clusterv1.MachineDeployment, error) {
	// List all MachineDeployments for the cluster
	mdList := &clusterv1.MachineDeploymentList{}
	if err := c.client.List(ctx, mdList, 
		client.InNamespace(c.namespace),
		client.MatchingLabels{clusterv1.ClusterNameLabel: clusterName},
	); err != nil {
		return nil, fmt.Errorf("failed to list machine deployments: %w", err)
	}

	// Find the specific MachineDeployment
	for _, md := range mdList.Items {
		if md.Name == mdName {
			return &md, nil
		}
	}

	return nil, fmt.Errorf("machine deployment %s not found in cluster %s", mdName, clusterName)
}

// UpdateMachineDeployment updates a MachineDeployment.
func (c *Client) UpdateMachineDeployment(ctx context.Context, md *clusterv1.MachineDeployment) error {
	if err := c.client.Update(ctx, md); err != nil {
		return fmt.Errorf("failed to update machine deployment: %w", err)
	}
	return nil
}

// ListMachineDeployments lists all MachineDeployments for a cluster.
func (c *Client) ListMachineDeployments(ctx context.Context, clusterName string) (*clusterv1.MachineDeploymentList, error) {
	mdList := &clusterv1.MachineDeploymentList{}
	if err := c.client.List(ctx, mdList, client.InNamespace(c.namespace), client.MatchingLabels{
		clusterv1.ClusterNameLabel: clusterName,
	}); err != nil {
		return nil, fmt.Errorf("failed to list machine deployments: %w", err)
	}
	return mdList, nil
}

// GetKubeconfigSecret retrieves the kubeconfig secret for a cluster.
func (c *Client) GetKubeconfigSecret(ctx context.Context, clusterName string) (*corev1.Secret, error) {
	// The kubeconfig secret name follows the pattern: <cluster-name>-kubeconfig
	secretName := fmt.Sprintf("%s-kubeconfig", clusterName)
	
	secret := &corev1.Secret{}
	key := types.NamespacedName{
		Namespace: c.namespace,
		Name:      secretName,
	}
	
	if err := c.client.Get(ctx, key, secret); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, fmt.Errorf("kubeconfig secret for cluster %s not found", clusterName)
		}
		return nil, fmt.Errorf("failed to get kubeconfig secret: %w", err)
	}
	
	return secret, nil
}

// ListClusterClasses returns all ClusterClass resources in the namespace.
func (c *Client) ListClusterClasses(ctx context.Context) (*clusterv1.ClusterClassList, error) {
	clusterClasses := &clusterv1.ClusterClassList{}
	if err := c.client.List(ctx, clusterClasses, client.InNamespace(c.namespace)); err != nil {
		return nil, fmt.Errorf("failed to list cluster classes: %w", err)
	}
	return clusterClasses, nil
}

// GetClusterClass retrieves a ClusterClass by name.
func (c *Client) GetClusterClass(ctx context.Context, name string) (*clusterv1.ClusterClass, error) {
	// Handle nil client for testing
	if c == nil || c.client == nil {
		return nil, fmt.Errorf("kubernetes client not available (running in test mode)")
	}
	
	clusterClass := &clusterv1.ClusterClass{}
	key := types.NamespacedName{
		Namespace: c.namespace,
		Name:      name,
	}
	if err := c.client.Get(ctx, key, clusterClass); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, fmt.Errorf("cluster class %s not found", name)
		}
		return nil, fmt.Errorf("failed to get cluster class: %w", err)
	}
	return clusterClass, nil
}

// WaitForClusterReady waits for a cluster to reach ready state.
func (c *Client) WaitForClusterReady(ctx context.Context, clusterName string, timeout time.Duration) error {
	deadline, ok := ctx.Deadline()
	if !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
		deadline, _ = ctx.Deadline()
	}

	for {
		cluster, err := c.GetClusterByName(ctx, clusterName)
		if err != nil {
			return fmt.Errorf("failed to get cluster: %w", err)
		}

		// Check if cluster is ready
		if IsClusterReady(cluster) {
			return nil
		}

		// Check if cluster has failed
		if IsClusterFailed(cluster) {
			return fmt.Errorf("cluster %s has failed: %s", clusterName, GetClusterFailureMessage(cluster))
		}

		// Check timeout
		if time.Now().After(deadline) {
			return fmt.Errorf("timeout waiting for cluster %s to be ready", clusterName)
		}

		// Wait before next check
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(10 * time.Second):
			// Continue to next iteration
		}
	}
}

// WaitForClusterDeleted waits for a cluster to be fully deleted.
func (c *Client) WaitForClusterDeleted(ctx context.Context, clusterName string, timeout time.Duration) error {
	deadline, ok := ctx.Deadline()
	if !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
		deadline, _ = ctx.Deadline()
	}

	for {
		_, err := c.GetClusterByName(ctx, clusterName)
		if err != nil {
			if apierrors.IsNotFound(err) || fmt.Sprintf("cluster %s not found", clusterName) == err.Error() {
				// Cluster is gone
				return nil
			}
			return fmt.Errorf("failed to check cluster: %w", err)
		}

		// Check timeout
		if time.Now().After(deadline) {
			return fmt.Errorf("timeout waiting for cluster %s to be deleted", clusterName)
		}

		// Wait before next check
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(10 * time.Second):
			// Continue to next iteration
		}
	}
}

// Helper functions

// IsClusterReady checks if a cluster is in ready state.
func IsClusterReady(cluster *clusterv1.Cluster) bool {
	return cluster.Status.Phase == string(clusterv1.ClusterPhaseProvisioned) &&
		cluster.Status.ControlPlaneReady &&
		cluster.Status.InfrastructureReady
}

// IsClusterFailed checks if a cluster is in failed state.
func IsClusterFailed(cluster *clusterv1.Cluster) bool {
	return cluster.Status.Phase == string(clusterv1.ClusterPhaseFailed) ||
		cluster.Status.Phase == string(clusterv1.ClusterPhaseDeleting)
}

// GetClusterFailureMessage extracts failure message from cluster conditions.
func GetClusterFailureMessage(cluster *clusterv1.Cluster) string {
	for _, condition := range cluster.Status.Conditions {
		if condition.Status == corev1.ConditionFalse && condition.Severity == clusterv1.ConditionSeverityError {
			return fmt.Sprintf("%s: %s", condition.Reason, condition.Message)
		}
	}
	return "unknown failure"
}