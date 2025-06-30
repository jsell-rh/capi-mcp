package kube

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestListClusters(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, clusterv1.AddToScheme(scheme))

	cluster1 := &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cluster-1",
			Namespace: "test-namespace",
		},
		Spec: clusterv1.ClusterSpec{
			Topology: &clusterv1.Topology{
				Version: "v1.31.0",
			},
		},
		Status: clusterv1.ClusterStatus{
			Phase: string(clusterv1.ClusterPhaseProvisioned),
		},
	}

	cluster2 := &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cluster-2",
			Namespace: "test-namespace",
		},
		Spec: clusterv1.ClusterSpec{
			Topology: &clusterv1.Topology{
				Version: "v1.30.0",
			},
		},
		Status: clusterv1.ClusterStatus{
			Phase: string(clusterv1.ClusterPhaseProvisioning),
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(cluster1, cluster2).
		Build()

	c := &Client{
		client:    fakeClient,
		namespace: "test-namespace",
	}

	ctx := context.Background()
	clusters, err := c.ListClusters(ctx)

	require.NoError(t, err)
	require.NotNil(t, clusters)
	assert.Len(t, clusters.Items, 2)
	
	// Check cluster names
	names := []string{clusters.Items[0].Name, clusters.Items[1].Name}
	assert.Contains(t, names, "cluster-1")
	assert.Contains(t, names, "cluster-2")
}

func TestGetClusterByName(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, clusterv1.AddToScheme(scheme))

	cluster := &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster",
			Namespace: "test-namespace",
		},
		Spec: clusterv1.ClusterSpec{
			Topology: &clusterv1.Topology{
				Version: "v1.31.0",
			},
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(cluster).
		Build()

	c := &Client{
		client:    fakeClient,
		namespace: "test-namespace",
	}

	ctx := context.Background()

	t.Run("existing cluster", func(t *testing.T) {
		result, err := c.GetClusterByName(ctx, "test-cluster")
		require.NoError(t, err)
		assert.Equal(t, "test-cluster", result.Name)
		assert.Equal(t, "v1.31.0", result.Spec.Topology.Version)
	})

	t.Run("non-existent cluster", func(t *testing.T) {
		_, err := c.GetClusterByName(ctx, "non-existent")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

func TestCreateCluster(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, clusterv1.AddToScheme(scheme))

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		Build()

	c := &Client{
		client:    fakeClient,
		namespace: "test-namespace",
	}

	cluster := &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: "new-cluster",
		},
		Spec: clusterv1.ClusterSpec{
			Topology: &clusterv1.Topology{
				Version: "v1.31.0",
			},
		},
	}

	ctx := context.Background()
	err := c.CreateCluster(ctx, cluster)
	require.NoError(t, err)

	// Verify cluster was created
	created := &clusterv1.Cluster{}
	key := types.NamespacedName{
		Namespace: "test-namespace",
		Name:      "new-cluster",
	}
	err = fakeClient.Get(ctx, key, created)
	require.NoError(t, err)
	assert.Equal(t, "new-cluster", created.Name)
	assert.Equal(t, "test-namespace", created.Namespace)
}

func TestDeleteCluster(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, clusterv1.AddToScheme(scheme))

	cluster := &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster",
			Namespace: "test-namespace",
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(cluster).
		Build()

	c := &Client{
		client:    fakeClient,
		namespace: "test-namespace",
	}

	ctx := context.Background()

	t.Run("existing cluster", func(t *testing.T) {
		err := c.DeleteCluster(ctx, "test-cluster")
		require.NoError(t, err)

		// Verify cluster was deleted
		key := types.NamespacedName{
			Namespace: "test-namespace",
			Name:      "test-cluster",
		}
		deleted := &clusterv1.Cluster{}
		err = fakeClient.Get(ctx, key, deleted)
		assert.Error(t, err) // Should not be found
	})

	t.Run("non-existent cluster", func(t *testing.T) {
		err := c.DeleteCluster(ctx, "non-existent")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

func TestGetMachineDeployment(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, clusterv1.AddToScheme(scheme))

	md := &clusterv1.MachineDeployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "worker-md",
			Namespace: "test-namespace",
			Labels: map[string]string{
				clusterv1.ClusterNameLabel: "test-cluster",
			},
		},
		Spec: clusterv1.MachineDeploymentSpec{
			Replicas: int32Ptr(3),
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(md).
		Build()

	c := &Client{
		client:    fakeClient,
		namespace: "test-namespace",
	}

	ctx := context.Background()

	t.Run("existing machine deployment", func(t *testing.T) {
		result, err := c.GetMachineDeployment(ctx, "test-cluster", "worker-md")
		require.NoError(t, err)
		assert.Equal(t, "worker-md", result.Name)
		assert.Equal(t, int32(3), *result.Spec.Replicas)
	})

	t.Run("non-existent machine deployment", func(t *testing.T) {
		_, err := c.GetMachineDeployment(ctx, "test-cluster", "non-existent")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

func TestUpdateMachineDeployment(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, clusterv1.AddToScheme(scheme))

	md := &clusterv1.MachineDeployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "worker-md",
			Namespace: "test-namespace",
		},
		Spec: clusterv1.MachineDeploymentSpec{
			Replicas: int32Ptr(3),
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(md).
		Build()

	c := &Client{
		client:    fakeClient,
		namespace: "test-namespace",
	}

	ctx := context.Background()

	// Update replicas
	md.Spec.Replicas = int32Ptr(5)
	err := c.UpdateMachineDeployment(ctx, md)
	require.NoError(t, err)

	// Verify update
	key := types.NamespacedName{
		Namespace: "test-namespace",
		Name:      "worker-md",
	}
	updated := &clusterv1.MachineDeployment{}
	err = fakeClient.Get(ctx, key, updated)
	require.NoError(t, err)
	assert.Equal(t, int32(5), *updated.Spec.Replicas)
}

func TestGetKubeconfigSecret(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster-kubeconfig",
			Namespace: "test-namespace",
		},
		Data: map[string][]byte{
			"value": []byte("fake-kubeconfig-data"),
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(secret).
		Build()

	c := &Client{
		client:    fakeClient,
		namespace: "test-namespace",
	}

	ctx := context.Background()

	t.Run("existing secret", func(t *testing.T) {
		result, err := c.GetKubeconfigSecret(ctx, "test-cluster")
		require.NoError(t, err)
		assert.Equal(t, "test-cluster-kubeconfig", result.Name)
		assert.Equal(t, "fake-kubeconfig-data", string(result.Data["value"]))
	})

	t.Run("non-existent secret", func(t *testing.T) {
		_, err := c.GetKubeconfigSecret(ctx, "non-existent")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

func TestHelperFunctions(t *testing.T) {
	t.Run("IsClusterReady", func(t *testing.T) {
		readyCluster := &clusterv1.Cluster{
			Status: clusterv1.ClusterStatus{
				Phase:                string(clusterv1.ClusterPhaseProvisioned),
				ControlPlaneReady:    true,
				InfrastructureReady:  true,
			},
		}
		assert.True(t, IsClusterReady(readyCluster))

		notReadyCluster := &clusterv1.Cluster{
			Status: clusterv1.ClusterStatus{
				Phase:                string(clusterv1.ClusterPhaseProvisioning),
				ControlPlaneReady:    false,
				InfrastructureReady:  true,
			},
		}
		assert.False(t, IsClusterReady(notReadyCluster))
	})

	t.Run("IsClusterFailed", func(t *testing.T) {
		failedCluster := &clusterv1.Cluster{
			Status: clusterv1.ClusterStatus{
				Phase: string(clusterv1.ClusterPhaseFailed),
			},
		}
		assert.True(t, IsClusterFailed(failedCluster))

		provisionedCluster := &clusterv1.Cluster{
			Status: clusterv1.ClusterStatus{
				Phase: string(clusterv1.ClusterPhaseProvisioned),
			},
		}
		assert.False(t, IsClusterFailed(provisionedCluster))
	})

	t.Run("GetClusterFailureMessage", func(t *testing.T) {
		cluster := &clusterv1.Cluster{
			Status: clusterv1.ClusterStatus{
				Conditions: []clusterv1.Condition{
					{
						Type:     clusterv1.ReadyCondition,
						Status:   corev1.ConditionFalse,
						Severity: clusterv1.ConditionSeverityError,
						Reason:   "TestFailure",
						Message:  "Test failure message",
					},
				},
			},
		}
		
		message := GetClusterFailureMessage(cluster)
		assert.Contains(t, message, "TestFailure")
		assert.Contains(t, message, "Test failure message")
	})
}

func TestWaitForClusterReady(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, clusterv1.AddToScheme(scheme))

	t.Run("cluster becomes ready", func(t *testing.T) {
		cluster := &clusterv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-cluster",
				Namespace: "test-namespace",
			},
			Status: clusterv1.ClusterStatus{
				Phase:               string(clusterv1.ClusterPhaseProvisioned),
				ControlPlaneReady:   true,
				InfrastructureReady: true,
			},
		}

		fakeClient := fake.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(cluster).
			Build()

		c := &Client{
			client:    fakeClient,
			namespace: "test-namespace",
		}

		ctx := context.Background()
		err := c.WaitForClusterReady(ctx, "test-cluster", 1*time.Second)
		assert.NoError(t, err)
	})

	t.Run("timeout waiting for cluster", func(t *testing.T) {
		cluster := &clusterv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-cluster",
				Namespace: "test-namespace",
			},
			Status: clusterv1.ClusterStatus{
				Phase:               string(clusterv1.ClusterPhaseProvisioning),
				ControlPlaneReady:   false,
				InfrastructureReady: false,
			},
		}

		fakeClient := fake.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(cluster).
			Build()

		c := &Client{
			client:    fakeClient,
			namespace: "test-namespace",
		}

		ctx := context.Background()
		err := c.WaitForClusterReady(ctx, "test-cluster", 100*time.Millisecond)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "deadline")
	})
}

func int32Ptr(i int32) *int32 {
	return &i
}