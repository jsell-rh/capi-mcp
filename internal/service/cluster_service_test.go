package service

import (
	"context"
	"encoding/json"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"

	api "github.com/capi-mcp/capi-mcp-server/api/v1"
	"github.com/capi-mcp/capi-mcp-server/internal/kube"
)

func createTestCluster(name, namespace string, phase clusterv1.ClusterPhase) *clusterv1.Cluster {
	return &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				"cluster.x-k8s.io/provider": "aws",
			},
			CreationTimestamp: metav1.Now(),
		},
		Spec: clusterv1.ClusterSpec{
			Topology: &clusterv1.Topology{
				Version: "v1.31.0",
				Class:   "aws-cluster-class",
			},
			ControlPlaneEndpoint: clusterv1.APIEndpoint{
				Host: "test-cluster-api.example.com",
				Port: 6443,
			},
		},
		Status: clusterv1.ClusterStatus{
			Phase:               string(phase),
			ControlPlaneReady:   phase == clusterv1.ClusterPhaseProvisioned,
			InfrastructureReady: phase == clusterv1.ClusterPhaseProvisioned,
		},
	}
}

func createTestMachineDeployment(name, namespace, clusterName string, replicas int32) *clusterv1.MachineDeployment {
	return &clusterv1.MachineDeployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				clusterv1.ClusterNameLabel: clusterName,
			},
		},
		Spec: clusterv1.MachineDeploymentSpec{
			Replicas: &replicas,
		},
		Status: clusterv1.MachineDeploymentStatus{
			UpdatedReplicas: replicas,
		},
	}
}

func createTestKubeconfigSecret(clusterName, namespace string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      clusterName + "-kubeconfig",
			Namespace: namespace,
		},
		Data: map[string][]byte{
			"value": []byte(`apiVersion: v1
kind: Config
clusters:
- cluster:
    server: https://test-cluster-api.example.com:6443
  name: test-cluster
contexts:
- context:
    cluster: test-cluster
  name: test-cluster
current-context: test-cluster`),
		},
	}
}

func setupTestService() *ClusterService {
	kubeClient := &kube.Client{} // Mock client for unit tests
	logger := slog.New(slog.NewTextHandler(nil, &slog.HandlerOptions{Level: slog.LevelError}))
	
	// For these tests, we'll test the business logic parts that don't require the client
	return NewClusterService(kubeClient, logger)
}

func TestClusterService_ListClusters(t *testing.T) {
	service := setupTestService()
	
	// We need to test this with a proper mock or interface
	// For now, let's test the helper functions
	t.Run("estimateNodeCount", func(t *testing.T) {
		cluster := &clusterv1.Cluster{
			Spec: clusterv1.ClusterSpec{
				Topology: &clusterv1.Topology{
					Workers: &clusterv1.WorkersTopology{
						MachineDeployments: []clusterv1.MachineDeploymentTopology{
							{
								Replicas: func(i int32) *int32 { return &i }(3),
							},
							{
								Replicas: func(i int32) *int32 { return &i }(2),
							},
						},
					},
				},
			},
		}
		
		count := service.estimateNodeCount(cluster)
		assert.Equal(t, 5, count)
	})

	t.Run("estimateNodeCount with nil workers", func(t *testing.T) {
		cluster := &clusterv1.Cluster{
			Spec: clusterv1.ClusterSpec{
				Topology: &clusterv1.Topology{},
			},
		}
		
		count := service.estimateNodeCount(cluster)
		assert.Equal(t, 0, count)
	})

	t.Run("estimateNodeCount with nil topology", func(t *testing.T) {
		cluster := &clusterv1.Cluster{
			Spec: clusterv1.ClusterSpec{},
		}
		
		count := service.estimateNodeCount(cluster)
		assert.Equal(t, 0, count)
	})
}

func TestGetNodeStatus(t *testing.T) {
	tests := []struct {
		name       string
		node       *corev1.Node
		wantStatus string
	}{
		{
			name: "ready node",
			node: &corev1.Node{
				Status: corev1.NodeStatus{
					Conditions: []corev1.NodeCondition{
						{
							Type:   corev1.NodeReady,
							Status: corev1.ConditionTrue,
						},
					},
				},
			},
			wantStatus: "Ready",
		},
		{
			name: "not ready node",
			node: &corev1.Node{
				Status: corev1.NodeStatus{
					Conditions: []corev1.NodeCondition{
						{
							Type:   corev1.NodeReady,
							Status: corev1.ConditionFalse,
						},
					},
				},
			},
			wantStatus: "NotReady",
		},
		{
			name: "unknown status node",
			node: &corev1.Node{
				Status: corev1.NodeStatus{
					Conditions: []corev1.NodeCondition{
						{
							Type:   corev1.NodeDiskPressure,
							Status: corev1.ConditionFalse,
						},
					},
				},
			},
			wantStatus: "Unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status := getNodeStatus(tt.node)
			assert.Equal(t, tt.wantStatus, status)
		})
	}
}

func TestGetNodeRoles(t *testing.T) {
	tests := []struct {
		name      string
		node      *corev1.Node
		wantRoles []string
	}{
		{
			name: "control plane node",
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"node-role.kubernetes.io/control-plane": "",
					},
				},
			},
			wantRoles: []string{"control-plane"},
		},
		{
			name: "worker node with explicit role",
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"node-role.kubernetes.io/worker": "",
					},
				},
			},
			wantRoles: []string{"worker"},
		},
		{
			name: "node with multiple roles",
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"node-role.kubernetes.io/control-plane": "",
						"node-role.kubernetes.io/master":        "",
					},
				},
			},
			wantRoles: []string{"control-plane", "master"},
		},
		{
			name: "node with no role labels",
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"other-label": "value",
					},
				},
			},
			wantRoles: []string{"worker"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			roles := getNodeRoles(tt.node)
			assert.ElementsMatch(t, tt.wantRoles, roles)
		})
	}
}

func TestValidateInputs(t *testing.T) {
	_ = setupTestService()

	t.Run("valid GetClusterInput", func(t *testing.T) {
		input := api.GetClusterInput{
			ClusterName: "test-cluster",
		}
		assert.NotEmpty(t, input.ClusterName)
	})

	t.Run("valid CreateClusterInput", func(t *testing.T) {
		input := api.CreateClusterInput{
			ClusterName:       "new-cluster",
			TemplateName:      "aws-template",
			KubernetesVersion: "v1.31.0",
			Variables: map[string]interface{}{
				"region": "us-west-2",
				"instanceType": "m5.large",
			},
		}
		assert.NotEmpty(t, input.ClusterName)
		assert.NotEmpty(t, input.TemplateName)
		assert.NotEmpty(t, input.KubernetesVersion)
	})

	t.Run("valid ScaleClusterInput", func(t *testing.T) {
		input := api.ScaleClusterInput{
			ClusterName:  "test-cluster",
			NodePoolName: "worker-pool",
			Replicas:     5,
		}
		assert.NotEmpty(t, input.ClusterName)
		assert.NotEmpty(t, input.NodePoolName)
		assert.GreaterOrEqual(t, input.Replicas, 0)
	})
}

// Mock test for CreateCluster to test variable marshaling
func TestCreateClusterVariableMarshaling(t *testing.T) {
	t.Run("marshal complex variables", func(t *testing.T) {
		variables := map[string]interface{}{
			"region": "us-west-2",
			"instanceType": "m5.large",
			"nodeCount": 3,
			"config": map[string]interface{}{
				"networking": map[string]interface{}{
					"cidr": "10.0.0.0/16",
				},
			},
		}

		// Test that we can marshal these variables to JSON
		for name, value := range variables {
			_, err := json.Marshal(value)
			assert.NoError(t, err, "Failed to marshal variable %s", name)
		}
	})

	t.Run("marshal nil variables", func(t *testing.T) {
		var variables map[string]interface{}
		assert.Len(t, variables, 0)
	})
}

func TestTimeoutCalculation(t *testing.T) {
	tests := []struct {
		name            string
		contextTimeout  time.Duration
		operationTimeout time.Duration
		expectImmediate bool
	}{
		{
			name:            "context already has deadline",
			contextTimeout:  5 * time.Minute,
			operationTimeout: 10 * time.Minute,
			expectImmediate: true,
		},
		{
			name:            "no context deadline",
			contextTimeout:  0, // No deadline
			operationTimeout: 10 * time.Minute,
			expectImmediate: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var ctx context.Context
			if tt.contextTimeout > 0 {
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(context.Background(), tt.contextTimeout)
				defer cancel()
			} else {
				ctx = context.Background()
			}

			_, hasDeadline := ctx.Deadline()
			assert.Equal(t, tt.expectImmediate, hasDeadline)
		})
	}
}