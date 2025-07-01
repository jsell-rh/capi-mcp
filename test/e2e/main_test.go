package e2e

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"

	"github.com/capi-mcp/capi-mcp-server/test/e2e/utils"
)

const (
	// Test environment configuration
	KindClusterName     = "capi-e2e"
	TestNamespace       = "default"
	MCPServerNamespace  = "capi-mcp-system"
	MCPServerName       = "capi-mcp-server"
	
	// Test timeouts
	ClusterCreateTimeout = 15 * time.Minute
	ClusterDeleteTimeout = 10 * time.Minute
	NodeScaleTimeout     = 5 * time.Minute
	DefaultTimeout       = 30 * time.Second
	
	// AWS configuration
	DefaultAWSRegion     = "us-west-2"
	DefaultInstanceType  = "t3.small"
	DefaultControlPlaneInstanceType = "t3.medium"
)

var (
	// Global test environment state
	kubeClient    client.Client
	clusterUtil   *utils.ClusterUtil
	mcpClient     *utils.MCPClient
	logger        *slog.Logger
	testWorkDir   string
	cleanupAWS    bool
)

// TestMain sets up and tears down the complete E2E test environment
func TestMain(m *testing.M) {
	var err error
	
	// Setup logging
	logger = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	
	logger.Info("Starting E2E test suite setup")
	
	// Get test working directory
	testWorkDir, err = os.Getwd()
	if err != nil {
		logger.Error("Failed to get working directory", "error", err)
		os.Exit(1)
	}
	
	// Load environment configuration
	loadEnvironmentConfig()
	
	// Setup test environment
	if err := setupTestEnvironment(); err != nil {
		logger.Error("Failed to setup test environment", "error", err)
		os.Exit(1)
	}
	
	// Run tests
	logger.Info("Running E2E tests")
	code := m.Run()
	
	// Cleanup test environment
	if err := cleanupTestEnvironment(); err != nil {
		logger.Error("Failed to cleanup test environment", "error", err)
		// Don't exit with error code as tests may have passed
	}
	
	logger.Info("E2E test suite completed", "exit_code", code)
	os.Exit(code)
}

// loadEnvironmentConfig loads configuration from environment variables
func loadEnvironmentConfig() {
	// AWS cleanup configuration
	cleanupEnv := os.Getenv("E2E_CLEANUP_AWS_RESOURCES")
	cleanupAWS = cleanupEnv == "" || cleanupEnv == "true" // Default to true
	
	// AWS region configuration
	if region := os.Getenv("AWS_REGION"); region == "" {
		os.Setenv("AWS_REGION", DefaultAWSRegion)
		logger.Info("Using default AWS region", "region", DefaultAWSRegion)
	}
	
	// Validate required AWS credentials
	if os.Getenv("AWS_ACCESS_KEY_ID") == "" || os.Getenv("AWS_SECRET_ACCESS_KEY") == "" {
		logger.Warn("AWS credentials not found in environment - some tests may fail")
	}
	
	// SSH key name for AWS instances
	if keyName := os.Getenv("AWS_SSH_KEY_NAME"); keyName == "" {
		logger.Warn("AWS_SSH_KEY_NAME not set - cluster SSH access will not be available")
	}
	
	logger.Info("Environment configuration loaded", 
		"cleanup_aws", cleanupAWS,
		"aws_region", os.Getenv("AWS_REGION"),
		"has_aws_credentials", os.Getenv("AWS_ACCESS_KEY_ID") != "",
	)
}

// setupTestEnvironment creates the complete test environment
func setupTestEnvironment() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	
	logger.Info("Setting up E2E test environment")
	
	// Step 1: Create kind cluster
	if err := setupKindCluster(ctx); err != nil {
		return fmt.Errorf("failed to setup kind cluster: %w", err)
	}
	
	// Step 2: Install CAPI components
	if err := installCAPIComponents(ctx); err != nil {
		return fmt.Errorf("failed to install CAPI components: %w", err)
	}
	
	// Step 3: Create Kubernetes client
	if err := setupKubernetesClient(); err != nil {
		return fmt.Errorf("failed to setup Kubernetes client: %w", err)
	}
	
	// Step 4: Setup test utilities
	if err := setupTestUtilities(); err != nil {
		return fmt.Errorf("failed to setup test utilities: %w", err)
	}
	
	// Step 5: Deploy MCP server
	if err := deployMCPServer(ctx); err != nil {
		return fmt.Errorf("failed to deploy MCP server: %w", err)
	}
	
	// Step 6: Wait for environment to be ready
	if err := waitForEnvironmentReady(ctx); err != nil {
		return fmt.Errorf("failed to wait for environment ready: %w", err)
	}
	
	logger.Info("E2E test environment setup completed successfully")
	return nil
}

// setupKindCluster creates and configures the kind cluster
func setupKindCluster(ctx context.Context) error {
	logger.Info("Setting up kind cluster", "name", KindClusterName)
	
	scriptsDir := filepath.Join(testWorkDir, "scripts")
	setupScript := filepath.Join(scriptsDir, "setup-cluster.sh")
	
	// Execute setup script
	cmd := utils.NewCommand("bash", setupScript)
	cmd.Env = append(os.Environ(), 
		"KIND_CLUSTER_NAME="+KindClusterName,
		"SCRIPTS_DIR="+scriptsDir,
	)
	
	if err := cmd.RunWithContext(ctx); err != nil {
		return fmt.Errorf("setup-cluster.sh failed: %w", err)
	}
	
	logger.Info("Kind cluster setup completed", "name", KindClusterName)
	return nil
}

// installCAPIComponents installs Cluster API components
func installCAPIComponents(ctx context.Context) error {
	logger.Info("Installing CAPI components")
	
	// Install cluster-api core components
	if err := installCAPICore(ctx); err != nil {
		return fmt.Errorf("failed to install CAPI core: %w", err)
	}
	
	// Install AWS provider (CAPA)
	if err := installCAPAProvider(ctx); err != nil {
		return fmt.Errorf("failed to install CAPA provider: %w", err)
	}
	
	// Apply test ClusterClass
	if err := applyTestClusterClass(ctx); err != nil {
		return fmt.Errorf("failed to apply test ClusterClass: %w", err)
	}
	
	logger.Info("CAPI components installation completed")
	return nil
}

// installCAPICore installs the core Cluster API components
func installCAPICore(ctx context.Context) error {
	logger.Info("Installing CAPI core components")
	
	// Use clusterctl to initialize the management cluster
	cmd := utils.NewCommand("clusterctl", "init")
	if err := cmd.RunWithContext(ctx); err != nil {
		return fmt.Errorf("clusterctl init failed: %w", err)
	}
	
	return nil
}

// installCAPAProvider installs the AWS provider
func installCAPAProvider(ctx context.Context) error {
	logger.Info("Installing CAPA provider")
	
	// Create AWS credentials secret
	if err := createAWSCredentialsSecret(ctx); err != nil {
		return fmt.Errorf("failed to create AWS credentials secret: %w", err)
	}
	
	// Install AWS provider
	cmd := utils.NewCommand("clusterctl", "init", "--infrastructure", "aws")
	if err := cmd.RunWithContext(ctx); err != nil {
		return fmt.Errorf("clusterctl init aws failed: %w", err)
	}
	
	return nil
}

// createAWSCredentialsSecret creates the AWS credentials secret required by CAPA
func createAWSCredentialsSecret(ctx context.Context) error {
	logger.Info("Creating AWS credentials secret")
	
	accessKey := os.Getenv("AWS_ACCESS_KEY_ID")
	secretKey := os.Getenv("AWS_SECRET_ACCESS_KEY")
	
	if accessKey == "" || secretKey == "" {
		return fmt.Errorf("AWS credentials not found in environment")
	}
	
	// Create secret using kubectl
	cmd := utils.NewCommand("kubectl", "create", "secret", "generic", 
		"capa-manager-bootstrap-credentials",
		"--namespace", "capa-system",
		"--from-literal", "AccessKeyID="+accessKey,
		"--from-literal", "SecretAccessKey="+secretKey,
	)
	
	// Ignore error if secret already exists
	if err := cmd.RunWithContext(ctx); err != nil {
		logger.Warn("Failed to create AWS credentials secret (may already exist)", "error", err)
	}
	
	return nil
}

// applyTestClusterClass applies the test ClusterClass definition
func applyTestClusterClass(ctx context.Context) error {
	logger.Info("Applying test ClusterClass")
	
	manifestsDir := filepath.Join(testWorkDir, "manifests")
	clusterClassFile := filepath.Join(manifestsDir, "clusterclass.yaml")
	
	cmd := utils.NewCommand("kubectl", "apply", "-f", clusterClassFile)
	if err := cmd.RunWithContext(ctx); err != nil {
		return fmt.Errorf("failed to apply ClusterClass: %w", err)
	}
	
	return nil
}

// setupKubernetesClient creates a Kubernetes client for the test environment
func setupKubernetesClient() error {
	logger.Info("Setting up Kubernetes client")
	
	cfg, err := config.GetConfig()
	if err != nil {
		return fmt.Errorf("failed to get kubeconfig: %w", err)
	}
	
	kubeClient, err = client.New(cfg, client.Options{})
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes client: %w", err)
	}
	
	return nil
}

// setupTestUtilities initializes test utility objects
func setupTestUtilities() error {
	logger.Info("Setting up test utilities")
	
	var err error
	
	// Create cluster utility
	clusterUtil, err = utils.NewClusterUtil(kubeClient, logger)
	if err != nil {
		return fmt.Errorf("failed to create cluster utility: %w", err)
	}
	
	logger.Info("Test utilities setup completed")
	return nil
}

// deployMCPServer deploys the MCP server to the test environment
func deployMCPServer(ctx context.Context) error {
	logger.Info("Deploying MCP server")
	
	scriptsDir := filepath.Join(testWorkDir, "scripts")
	deployScript := filepath.Join(scriptsDir, "deploy-server.sh")
	
	cmd := utils.NewCommand("bash", deployScript)
	cmd.Env = append(os.Environ(),
		"MCP_SERVER_NAMESPACE="+MCPServerNamespace,
		"MCP_SERVER_NAME="+MCPServerName,
	)
	
	if err := cmd.RunWithContext(ctx); err != nil {
		return fmt.Errorf("deploy-server.sh failed: %w", err)
	}
	
	logger.Info("MCP server deployment completed")
	return nil
}

// waitForEnvironmentReady waits for all components to be ready
func waitForEnvironmentReady(ctx context.Context) error {
	logger.Info("Waiting for environment to be ready")
	
	// Wait for CAPI controllers
	if err := waitForCAPIControllers(ctx); err != nil {
		return fmt.Errorf("CAPI controllers not ready: %w", err)
	}
	
	// Wait for MCP server
	if err := waitForMCPServer(ctx); err != nil {
		return fmt.Errorf("MCP server not ready: %w", err)
	}
	
	// Initialize MCP client
	if err := initializeMCPClient(); err != nil {
		return fmt.Errorf("failed to initialize MCP client: %w", err)
	}
	
	logger.Info("Environment is ready for testing")
	return nil
}

// waitForCAPIControllers waits for CAPI controllers to be ready
func waitForCAPIControllers(ctx context.Context) error {
	logger.Info("Waiting for CAPI controllers to be ready")
	
	// Wait for core controllers
	cmd := utils.NewCommand("kubectl", "wait", "--for=condition=ready", "pod",
		"-l", "cluster.x-k8s.io/provider=cluster-api",
		"-n", "capi-system",
		"--timeout=300s")
	if err := cmd.RunWithContext(ctx); err != nil {
		return fmt.Errorf("CAPI core controllers not ready: %w", err)
	}
	
	// Wait for AWS provider controllers  
	cmd = utils.NewCommand("kubectl", "wait", "--for=condition=ready", "pod",
		"-l", "cluster.x-k8s.io/provider=infrastructure-aws",
		"-n", "capa-system",
		"--timeout=300s")
	if err := cmd.RunWithContext(ctx); err != nil {
		return fmt.Errorf("CAPA controllers not ready: %w", err)
	}
	
	logger.Info("CAPI controllers are ready")
	return nil
}

// waitForMCPServer waits for the MCP server to be ready
func waitForMCPServer(ctx context.Context) error {
	logger.Info("Waiting for MCP server to be ready")
	
	cmd := utils.NewCommand("kubectl", "wait", "--for=condition=ready", "pod",
		"-l", "app="+MCPServerName,
		"-n", MCPServerNamespace,
		"--timeout=300s")
	if err := cmd.RunWithContext(ctx); err != nil {
		return fmt.Errorf("MCP server not ready: %w", err)
	}
	
	logger.Info("MCP server is ready")
	return nil
}

// initializeMCPClient creates and configures the MCP client
func initializeMCPClient() error {
	logger.Info("Initializing MCP client")
	
	// Get MCP server endpoint (will be implemented in utils package)
	serverURL, err := getMCPServerURL()
	if err != nil {
		return fmt.Errorf("failed to get MCP server URL: %w", err)
	}
	
	// Create MCP client
	mcpClient, err = utils.NewMCPClient(serverURL, logger)
	if err != nil {
		return fmt.Errorf("failed to create MCP client: %w", err)
	}
	
	// Test connection
	if err := mcpClient.TestConnection(); err != nil {
		return fmt.Errorf("MCP client connection test failed: %w", err)
	}
	
	logger.Info("MCP client initialized successfully")
	return nil
}

// getMCPServerURL gets the MCP server URL from the Kubernetes service
func getMCPServerURL() (string, error) {
	// For now, use port-forward to access the service
	// In a real deployment, this would be a LoadBalancer or Ingress
	
	// Start port-forward in background
	cmd := utils.NewCommand("kubectl", "port-forward",
		"-n", MCPServerNamespace,
		"service/"+MCPServerName,
		"8080:8080")
	
	if err := cmd.StartBackground(); err != nil {
		return "", fmt.Errorf("failed to start port-forward: %w", err)
	}
	
	// Wait a moment for port-forward to establish
	time.Sleep(2 * time.Second)
	
	return "http://localhost:8080", nil
}

// cleanupTestEnvironment tears down the test environment
func cleanupTestEnvironment() error {
	logger.Info("Cleaning up E2E test environment")
	
	// Cleanup AWS resources if enabled
	if cleanupAWS {
		if err := cleanupAWSResources(); err != nil {
			logger.Error("Failed to cleanup AWS resources", "error", err)
		}
	}
	
	// Cleanup kind cluster
	if err := cleanupKindCluster(); err != nil {
		logger.Error("Failed to cleanup kind cluster", "error", err)
		return err
	}
	
	logger.Info("E2E test environment cleanup completed")
	return nil
}

// cleanupAWSResources removes any AWS resources created during testing
func cleanupAWSResources() error {
	logger.Info("Cleaning up AWS resources")
	
	scriptsDir := filepath.Join(testWorkDir, "scripts")
	cleanupScript := filepath.Join(scriptsDir, "cleanup.sh")
	
	cmd := utils.NewCommand("bash", cleanupScript)
	cmd.Env = append(os.Environ(),
		"AWS_REGION="+os.Getenv("AWS_REGION"),
		"CLEANUP_AWS=true",
	)
	
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("cleanup.sh failed: %w", err)
	}
	
	return nil
}

// cleanupKindCluster removes the kind cluster
func cleanupKindCluster() error {
	logger.Info("Cleaning up kind cluster", "name", KindClusterName)
	
	cmd := utils.NewCommand("kind", "delete", "cluster", "--name", KindClusterName)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to delete kind cluster: %w", err)
	}
	
	return nil
}

// Helper functions for tests

// RequireTestEnvironment ensures the test environment is properly set up
func RequireTestEnvironment(t *testing.T) {
	t.Helper()
	
	require.NotNil(t, kubeClient, "Kubernetes client not initialized")
	require.NotNil(t, clusterUtil, "Cluster utility not initialized")
	require.NotNil(t, mcpClient, "MCP client not initialized")
	require.NotNil(t, logger, "Logger not initialized")
}

// GetKubeClient returns the Kubernetes client for tests
func GetKubeClient() client.Client {
	return kubeClient
}

// GetClusterUtil returns the cluster utility for tests
func GetClusterUtil() *utils.ClusterUtil {
	return clusterUtil
}

// GetMCPClient returns the MCP client for tests
func GetMCPClient() *utils.MCPClient {
	return mcpClient
}

// GetLogger returns the logger for tests
func GetLogger() *slog.Logger {
	return logger
}