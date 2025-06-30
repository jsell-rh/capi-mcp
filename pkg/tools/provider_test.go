package tools

import (
	"context"
	"encoding/json"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/capi-mcp/capi-mcp-server/internal/service"
)

func createTestProvider(clusterService *service.ClusterService) *Provider {
	server := mcp.NewServer("test-server", "v1.0.0", nil)
	logger := slog.New(slog.NewTextHandler(nil, &slog.HandlerOptions{Level: slog.LevelError}))
	return NewProvider(server, logger, clusterService)
}

func TestNewProvider(t *testing.T) {
	server := mcp.NewServer("test-server", "v1.0.0", nil)
	logger := slog.New(slog.NewTextHandler(nil, &slog.HandlerOptions{Level: slog.LevelError}))
	
	t.Run("with cluster service", func(t *testing.T) {
		clusterService := &service.ClusterService{}
		provider := NewProvider(server, logger, clusterService)
		
		assert.NotNil(t, provider)
		assert.Equal(t, server, provider.server)
		assert.Equal(t, logger, provider.logger)
		assert.Equal(t, clusterService, provider.clusterService)
	})

	t.Run("with nil cluster service", func(t *testing.T) {
		provider := NewProvider(server, logger, nil)
		
		assert.NotNil(t, provider)
		assert.Nil(t, provider.clusterService)
	})
}

func TestRegisterTools(t *testing.T) {
	provider := createTestProvider(nil)
	
	err := provider.RegisterTools()
	assert.NoError(t, err)
	
	// The actual tool registration is handled by the MCP server
	// We can't easily test it without more complex mocking
}

func TestHandleListClusters(t *testing.T) {
	t.Run("with nil cluster service", func(t *testing.T) {
		provider := createTestProvider(nil)
		
		ctx := context.Background()
		session := &mcp.ServerSession{}
		params := &mcp.CallToolParamsFor[ListClustersArgs]{}
		
		result, err := provider.handleListClusters(ctx, session, params)
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Len(t, result.Content, 1)
		
		textContent, ok := result.Content[0].(*mcp.TextContent)
		require.True(t, ok)
		assert.Contains(t, textContent.Text, "service not initialized")
	})
}

func TestHandleGetCluster(t *testing.T) {
	t.Run("with nil cluster service", func(t *testing.T) {
		provider := createTestProvider(nil)
		
		ctx := context.Background()
		session := &mcp.ServerSession{}
		params := &mcp.CallToolParamsFor[GetClusterArgs]{
			Arguments: GetClusterArgs{
				ClusterName: "test-cluster",
			},
		}
		
		_, err := provider.handleGetCluster(ctx, session, params)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "service not initialized")
	})
}

func TestArgumentStructs(t *testing.T) {
	t.Run("ListClustersArgs", func(t *testing.T) {
		args := ListClustersArgs{}
		// ListClustersArgs is an alias for EmptyArgs, so it should be empty
		_ = args // Just to verify it compiles
	})

	t.Run("GetClusterArgs", func(t *testing.T) {
		args := GetClusterArgs{
			ClusterName: "test-cluster",
		}
		assert.Equal(t, "test-cluster", args.ClusterName)
	})

	t.Run("CreateClusterArgs", func(t *testing.T) {
		args := CreateClusterArgs{
			ClusterName:       "new-cluster",
			TemplateName:      "aws-template",
			KubernetesVersion: "v1.31.0",
			Variables: map[string]interface{}{
				"region": "us-west-2",
			},
		}
		assert.Equal(t, "new-cluster", args.ClusterName)
		assert.Equal(t, "aws-template", args.TemplateName)
		assert.Equal(t, "v1.31.0", args.KubernetesVersion)
		assert.Equal(t, "us-west-2", args.Variables["region"])
	})

	t.Run("DeleteClusterArgs", func(t *testing.T) {
		args := DeleteClusterArgs{
			ClusterName: "test-cluster",
		}
		assert.Equal(t, "test-cluster", args.ClusterName)
	})

	t.Run("ScaleClusterArgs", func(t *testing.T) {
		args := ScaleClusterArgs{
			ClusterName:  "test-cluster",
			NodePoolName: "worker-pool",
			Replicas:     5,
		}
		assert.Equal(t, "test-cluster", args.ClusterName)
		assert.Equal(t, "worker-pool", args.NodePoolName)
		assert.Equal(t, 5, args.Replicas)
	})

	t.Run("GetClusterKubeconfigArgs", func(t *testing.T) {
		args := GetClusterKubeconfigArgs{
			ClusterName: "test-cluster",
		}
		assert.Equal(t, "test-cluster", args.ClusterName)
	})

	t.Run("GetClusterNodesArgs", func(t *testing.T) {
		args := GetClusterNodesArgs{
			ClusterName: "test-cluster",
		}
		assert.Equal(t, "test-cluster", args.ClusterName)
	})
}

func TestErrorHandling(t *testing.T) {
	provider := createTestProvider(nil)
	ctx := context.Background()
	session := &mcp.ServerSession{}

	testCases := []struct {
		name     string
		handler  func() error
		wantErr  bool
		errCheck func(error) bool
	}{
		{
			name: "get_cluster with nil service",
			handler: func() error {
				params := &mcp.CallToolParamsFor[GetClusterArgs]{
					Arguments: GetClusterArgs{ClusterName: "test"},
				}
				_, err := provider.handleGetCluster(ctx, session, params)
				return err
			},
			wantErr: true,
			errCheck: func(err error) bool {
				return assert.Contains(t, err.Error(), "service not initialized")
			},
		},
		{
			name: "create_cluster with nil service",
			handler: func() error {
				params := &mcp.CallToolParamsFor[CreateClusterArgs]{
					Arguments: CreateClusterArgs{
						ClusterName:       "test",
						TemplateName:      "template",
						KubernetesVersion: "v1.31.0",
					},
				}
				_, err := provider.handleCreateCluster(ctx, session, params)
				return err
			},
			wantErr: true,
			errCheck: func(err error) bool {
				return assert.Contains(t, err.Error(), "service not initialized")
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.handler()
			if tc.wantErr {
				assert.Error(t, err)
				if tc.errCheck != nil {
					tc.errCheck(err)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// Test JSON marshaling for complex arguments
func TestJSONMarshaling(t *testing.T) {
	t.Run("CreateClusterArgs with complex variables", func(t *testing.T) {
		args := CreateClusterArgs{
			ClusterName:       "test-cluster",
			TemplateName:      "aws-template",
			KubernetesVersion: "v1.31.0",
			Variables: map[string]interface{}{
				"region": "us-west-2",
				"config": map[string]interface{}{
					"networking": map[string]interface{}{
						"cidr": "10.0.0.0/16",
						"subnets": []string{"10.0.1.0/24", "10.0.2.0/24"},
					},
				},
				"replicas": 3,
			},
		}

		// Verify we can marshal and unmarshal the args
		data, err := json.Marshal(args)
		require.NoError(t, err)

		var unmarshaled CreateClusterArgs
		err = json.Unmarshal(data, &unmarshaled)
		require.NoError(t, err)

		assert.Equal(t, args.ClusterName, unmarshaled.ClusterName)
		assert.Equal(t, args.TemplateName, unmarshaled.TemplateName)
		assert.Equal(t, args.KubernetesVersion, unmarshaled.KubernetesVersion)
		
		// Check complex nested variables
		assert.Equal(t, "us-west-2", unmarshaled.Variables["region"])
		assert.Equal(t, float64(3), unmarshaled.Variables["replicas"]) // JSON unmarshals numbers as float64
	})
}