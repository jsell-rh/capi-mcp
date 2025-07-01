package kube

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewWorkloadClientFromKubeconfig(t *testing.T) {
	validKubeconfig := `
apiVersion: v1
kind: Config
clusters:
- cluster:
    server: https://test-cluster-api.example.com:6443
    insecure-skip-tls-verify: true
  name: test-cluster
contexts:
- context:
    cluster: test-cluster
    user: test-user
  name: test-context
current-context: test-context
users:
- name: test-user
  user:
    token: test-token
`

	invalidKubeconfig := `invalid yaml content`

	t.Run("valid kubeconfig", func(t *testing.T) {
		client, err := NewWorkloadClientFromKubeconfig([]byte(validKubeconfig))
		require.NoError(t, err)
		assert.NotNil(t, client)
		assert.NotNil(t, client.clientset)
	})

	t.Run("invalid kubeconfig", func(t *testing.T) {
		_, err := NewWorkloadClientFromKubeconfig([]byte(invalidKubeconfig))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to parse kubeconfig")
	})

	t.Run("empty kubeconfig", func(t *testing.T) {
		_, err := NewWorkloadClientFromKubeconfig([]byte(""))
		assert.Error(t, err)
	})
}

func TestClusterInfo(t *testing.T) {
	clusterInfo := &ClusterInfo{
		KubernetesVersion: "v1.31.0",
		NodeCount:         3,
	}

	assert.Equal(t, "v1.31.0", clusterInfo.KubernetesVersion)
	assert.Equal(t, 3, clusterInfo.NodeCount)
}

// Note: Testing ListNodes and GetClusterInfo would require a real or mocked Kubernetes API server
// These would be better tested in integration tests
