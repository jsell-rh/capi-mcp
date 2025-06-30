package kube

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

// WorkloadClient represents a client for a workload cluster.
type WorkloadClient struct {
	clientset *kubernetes.Clientset
}

// NewWorkloadClientFromKubeconfig creates a new workload cluster client from kubeconfig data.
func NewWorkloadClientFromKubeconfig(kubeconfigData []byte) (*WorkloadClient, error) {
	// Parse the kubeconfig
	config, err := clientcmd.RESTConfigFromKubeConfig(kubeconfigData)
	if err != nil {
		return nil, fmt.Errorf("failed to parse kubeconfig: %w", err)
	}

	// Create clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create clientset: %w", err)
	}

	return &WorkloadClient{
		clientset: clientset,
	}, nil
}

// ListNodes returns all nodes in the workload cluster.
func (w *WorkloadClient) ListNodes(ctx context.Context) (*corev1.NodeList, error) {
	nodes, err := w.clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list nodes: %w", err)
	}
	return nodes, nil
}

// GetClusterInfo returns basic information about the workload cluster.
func (w *WorkloadClient) GetClusterInfo(ctx context.Context) (*ClusterInfo, error) {
	// Get server version
	version, err := w.clientset.Discovery().ServerVersion()
	if err != nil {
		return nil, fmt.Errorf("failed to get server version: %w", err)
	}

	// Get node count
	nodes, err := w.ListNodes(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get nodes: %w", err)
	}

	return &ClusterInfo{
		KubernetesVersion: version.GitVersion,
		NodeCount:         len(nodes.Items),
	}, nil
}

// ClusterInfo contains basic information about a workload cluster.
type ClusterInfo struct {
	KubernetesVersion string `json:"kubernetes_version"`
	NodeCount         int    `json:"node_count"`
}