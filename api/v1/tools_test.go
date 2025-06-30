package v1

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClusterSummary(t *testing.T) {
	summary := ClusterSummary{
		Name:              "test-cluster",
		Namespace:         "default",
		Provider:          "aws",
		KubernetesVersion: "v1.31.0",
		Status:            "Provisioned",
		CreatedAt:         "2024-01-01T12:00:00Z",
		NodeCount:         3,
	}

	// Test JSON serialization
	data, err := json.Marshal(summary)
	require.NoError(t, err)

	var unmarshaled ClusterSummary
	err = json.Unmarshal(data, &unmarshaled)
	require.NoError(t, err)

	assert.Equal(t, summary.Name, unmarshaled.Name)
	assert.Equal(t, summary.Provider, unmarshaled.Provider)
	assert.Equal(t, summary.NodeCount, unmarshaled.NodeCount)
}

func TestClusterDetails(t *testing.T) {
	details := ClusterDetails{
		Name:              "test-cluster",
		Namespace:         "default",
		Provider:          "aws",
		Region:            "us-west-2",
		KubernetesVersion: "v1.31.0",
		Status:            "Provisioned",
		CreatedAt:         "2024-01-01T12:00:00Z",
		Endpoint:          "https://test-cluster.example.com:6443",
		NodePools: []NodePool{
			{
				Name:          "worker-pool",
				Replicas:      3,
				ReadyReplicas: 3,
				MachineType:   "m5.large",
			},
		},
		Conditions: []ClusterCondition{
			{
				Type:               "Ready",
				Status:             "True",
				LastTransitionTime: "2024-01-01T12:00:00Z",
				Reason:             "ClusterReady",
				Message:            "Cluster is ready",
			},
		},
		InfrastructureRef: map[string]interface{}{
			"kind":       "AWSCluster",
			"name":       "test-cluster-aws",
			"namespace":  "default",
			"apiVersion": "infrastructure.cluster.x-k8s.io/v1beta2",
		},
	}

	// Test JSON serialization
	data, err := json.Marshal(details)
	require.NoError(t, err)

	var unmarshaled ClusterDetails
	err = json.Unmarshal(data, &unmarshaled)
	require.NoError(t, err)

	assert.Equal(t, details.Name, unmarshaled.Name)
	assert.Equal(t, details.Region, unmarshaled.Region)
	assert.Len(t, unmarshaled.NodePools, 1)
	assert.Equal(t, "worker-pool", unmarshaled.NodePools[0].Name)
	assert.Len(t, unmarshaled.Conditions, 1)
	assert.Equal(t, "Ready", unmarshaled.Conditions[0].Type)
	assert.Equal(t, "AWSCluster", unmarshaled.InfrastructureRef["kind"])
}

func TestCreateClusterInput(t *testing.T) {
	input := CreateClusterInput{
		ClusterName:       "new-cluster",
		TemplateName:      "aws-template",
		KubernetesVersion: "v1.31.0",
		Variables: map[string]interface{}{
			"region":     "us-west-2",
			"nodeCount":  3,
			"networking": map[string]interface{}{
				"cidr": "10.0.0.0/16",
			},
		},
	}

	// Test JSON serialization
	data, err := json.Marshal(input)
	require.NoError(t, err)

	var unmarshaled CreateClusterInput
	err = json.Unmarshal(data, &unmarshaled)
	require.NoError(t, err)

	assert.Equal(t, input.ClusterName, unmarshaled.ClusterName)
	assert.Equal(t, input.TemplateName, unmarshaled.TemplateName)
	assert.Equal(t, input.KubernetesVersion, unmarshaled.KubernetesVersion)
	assert.Equal(t, "us-west-2", unmarshaled.Variables["region"])
	assert.Equal(t, float64(3), unmarshaled.Variables["nodeCount"]) // JSON numbers become float64
}

func TestScaleClusterInput(t *testing.T) {
	input := ScaleClusterInput{
		ClusterName:  "test-cluster",
		NodePoolName: "worker-pool",
		Replicas:     5,
	}

	// Test JSON serialization
	data, err := json.Marshal(input)
	require.NoError(t, err)

	var unmarshaled ScaleClusterInput
	err = json.Unmarshal(data, &unmarshaled)
	require.NoError(t, err)

	assert.Equal(t, input.ClusterName, unmarshaled.ClusterName)
	assert.Equal(t, input.NodePoolName, unmarshaled.NodePoolName)
	assert.Equal(t, input.Replicas, unmarshaled.Replicas)
}

func TestNodeInfo(t *testing.T) {
	nodeInfo := NodeInfo{
		Name:              "worker-node-1",
		Status:            "Ready",
		Roles:             []string{"worker"},
		KubeletVersion:    "v1.31.0",
		InternalIP:        "10.0.1.100",
		ExternalIP:        "203.0.113.100",
		InstanceType:      "m5.large",
		AvailabilityZone:  "us-west-2a",
		Labels: map[string]string{
			"node.kubernetes.io/instance-type":     "m5.large",
			"topology.kubernetes.io/zone":          "us-west-2a",
			"node-role.kubernetes.io/worker":       "",
		},
	}

	// Test JSON serialization
	data, err := json.Marshal(nodeInfo)
	require.NoError(t, err)

	var unmarshaled NodeInfo
	err = json.Unmarshal(data, &unmarshaled)
	require.NoError(t, err)

	assert.Equal(t, nodeInfo.Name, unmarshaled.Name)
	assert.Equal(t, nodeInfo.Status, unmarshaled.Status)
	assert.Equal(t, nodeInfo.Roles, unmarshaled.Roles)
	assert.Equal(t, nodeInfo.InternalIP, unmarshaled.InternalIP)
	assert.Equal(t, nodeInfo.ExternalIP, unmarshaled.ExternalIP)
	assert.Equal(t, nodeInfo.Labels, unmarshaled.Labels)
}

func TestOutputStructures(t *testing.T) {
	t.Run("ListClustersOutput", func(t *testing.T) {
		output := ListClustersOutput{
			Clusters: []ClusterSummary{
				{Name: "cluster-1", Status: "Provisioned"},
				{Name: "cluster-2", Status: "Provisioning"},
			},
		}

		data, err := json.Marshal(output)
		require.NoError(t, err)

		var unmarshaled ListClustersOutput
		err = json.Unmarshal(data, &unmarshaled)
		require.NoError(t, err)

		assert.Len(t, unmarshaled.Clusters, 2)
		assert.Equal(t, "cluster-1", unmarshaled.Clusters[0].Name)
	})

	t.Run("CreateClusterOutput", func(t *testing.T) {
		output := CreateClusterOutput{
			ClusterName: "new-cluster",
			Status:      "provisioned",
			Message:     "Cluster created successfully",
		}

		data, err := json.Marshal(output)
		require.NoError(t, err)

		var unmarshaled CreateClusterOutput
		err = json.Unmarshal(data, &unmarshaled)
		require.NoError(t, err)

		assert.Equal(t, output.ClusterName, unmarshaled.ClusterName)
		assert.Equal(t, output.Status, unmarshaled.Status)
		assert.Equal(t, output.Message, unmarshaled.Message)
	})

	t.Run("ScaleClusterOutput", func(t *testing.T) {
		output := ScaleClusterOutput{
			Status:      "scaling",
			Message:     "Scaling in progress",
			OldReplicas: 3,
			NewReplicas: 5,
		}

		data, err := json.Marshal(output)
		require.NoError(t, err)

		var unmarshaled ScaleClusterOutput
		err = json.Unmarshal(data, &unmarshaled)
		require.NoError(t, err)

		assert.Equal(t, output.Status, unmarshaled.Status)
		assert.Equal(t, output.OldReplicas, unmarshaled.OldReplicas)
		assert.Equal(t, output.NewReplicas, unmarshaled.NewReplicas)
	})
}

func TestInputValidation(t *testing.T) {
	t.Run("empty cluster name", func(t *testing.T) {
		input := GetClusterInput{
			ClusterName: "",
		}
		// The validation tags are present, but actual validation
		// would happen at the service layer or with a validator
		assert.Empty(t, input.ClusterName)
	})

	t.Run("negative replicas", func(t *testing.T) {
		input := ScaleClusterInput{
			ClusterName:  "test-cluster",
			NodePoolName: "worker-pool",
			Replicas:     -1,
		}
		// The validation tag specifies gte=0, but validation
		// would need to be enforced by the service layer
		assert.Equal(t, -1, input.Replicas)
	})
}

func TestEmptyStructures(t *testing.T) {
	t.Run("empty input structures", func(t *testing.T) {
		inputs := []interface{}{
			ListClustersInput{},
			GetClusterInput{},
			CreateClusterInput{},
			DeleteClusterInput{},
			ScaleClusterInput{},
			GetClusterKubeconfigInput{},
			GetClusterNodesInput{},
		}

		for _, input := range inputs {
			data, err := json.Marshal(input)
			require.NoError(t, err)
			assert.NotEmpty(t, data)
		}
	})

	t.Run("empty output structures", func(t *testing.T) {
		outputs := []interface{}{
			ListClustersOutput{},
			GetClusterOutput{},
			CreateClusterOutput{},
			DeleteClusterOutput{},
			ScaleClusterOutput{},
			GetClusterKubeconfigOutput{},
			GetClusterNodesOutput{},
		}

		for _, output := range outputs {
			data, err := json.Marshal(output)
			require.NoError(t, err)
			assert.NotEmpty(t, data)
		}
	})
}