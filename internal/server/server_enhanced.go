package server

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/capi-mcp/capi-mcp-server/internal/config"
	"github.com/capi-mcp/capi-mcp-server/internal/errors"
	"github.com/capi-mcp/capi-mcp-server/internal/kube"
	"github.com/capi-mcp/capi-mcp-server/internal/logging"
	"github.com/capi-mcp/capi-mcp-server/internal/metrics"
	"github.com/capi-mcp/capi-mcp-server/internal/middleware"
	"github.com/capi-mcp/capi-mcp-server/internal/service"
	"github.com/capi-mcp/capi-mcp-server/pkg/provider"
	"github.com/capi-mcp/capi-mcp-server/pkg/provider/aws"
	"github.com/capi-mcp/capi-mcp-server/pkg/tools"
)

// EnhancedServer represents the CAPI MCP server with enhanced error handling and logging.
type EnhancedServer struct {
	config           *config.Config
	logger           *logging.Logger
	mcpServer        *mcp.Server
	metricsCollector *metrics.Collector
}

// NewEnhanced creates a new server instance with enhanced error handling and logging.
func NewEnhanced(cfg *config.Config) (*EnhancedServer, error) {
	if cfg == nil {
		return nil, errors.New(errors.CodeInvalidInput, "config is required")
	}
	
	// Create metrics collector
	metricsCollector := metrics.NewCollector()
	
	// Set server information metrics
	metricsCollector.SetServerInfo(cfg.Version, cfg.BuildDate, "go1.23")
	
	// Create logger from config with metrics integration
	logLevel := logging.ParseLevel(cfg.LogLevel)
	logger := logging.NewLoggerWithMetrics(logLevel, "json", metricsCollector).WithComponent("server")
	
	logger.Info("Initializing CAPI MCP Server",
		"version", cfg.Version,
		"log_level", cfg.LogLevel,
		"metrics_port", cfg.MetricsPort,
	)

	// Create MCP server instance with metadata
	mcpServer := mcp.NewServer("capi-mcp-server", cfg.Version, nil)

	// Create server instance
	s := &EnhancedServer{
		config:           cfg,
		metricsCollector: metricsCollector,
		logger:    logger,
		mcpServer: mcpServer,
	}

	// Register capabilities
	if err := s.registerCapabilities(); err != nil {
		logger.WithError(err).Error("Failed to register capabilities")
		return nil, errors.Wrap(err, errors.CodeInternal, "failed to register capabilities")
	}
	
	logger.Info("Server initialization completed successfully")
	return s, nil
}

// Run starts the server and blocks until the context is cancelled.
func (s *EnhancedServer) Run(ctx context.Context) error {
	s.logger.Info("Starting CAPI MCP server", 
		"port", s.config.ServerPort,
		"metrics_port", s.config.MetricsPort,
		"shutdown_grace", s.config.ShutdownGrace,
	)
	
	// Create health check handler
	mux := http.NewServeMux()
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/ready", s.handleReady)

	// Create MCP handler with authentication
	mcpHandler := mcp.NewStreamableHTTPHandler(s.authenticateRequest, nil)
	mux.Handle("/", mcpHandler)
	
	// Build middleware chain
	handler := middleware.RequestLogger(s.logger)(
		middleware.ErrorHandler(s.logger)(
			middleware.RequestTimeout(30 * time.Second)(
				middleware.CORS([]string{"*"})(mux),
			),
		),
	)

	// Create HTTP server
	httpServer := &http.Server{
		Addr:           fmt.Sprintf(":%d", s.config.ServerPort),
		Handler:        handler,
		ReadTimeout:    30 * time.Second,
		WriteTimeout:   30 * time.Second,
		IdleTimeout:    120 * time.Second,
		MaxHeaderBytes: 1 << 20, // 1MB
	}

	// Start server in goroutine
	serverErr := make(chan error, 1)
	go func() {
		s.logger.Info("MCP server listening", 
			"addr", httpServer.Addr,
			"tls", false,
		)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErr <- errors.Wrap(err, errors.CodeInternal, "server failed to start")
		}
	}()
	
	// Start metrics server
	metricsErr := make(chan error, 1)
	go func() {
		if err := s.startMetricsServer(ctx); err != nil {
			metricsErr <- err
		}
	}()

	// Wait for shutdown signal or error
	select {
	case err := <-serverErr:
		s.logger.WithError(err).Error("Server error")
		return err
	case err := <-metricsErr:
		s.logger.WithError(err).Error("Metrics server error")
		return err
	case <-ctx.Done():
		s.logger.Info("Shutdown signal received, starting graceful shutdown")
		
		// Create shutdown context with timeout
		shutdownCtx, cancel := context.WithTimeout(context.Background(), s.config.ShutdownGrace)
		defer cancel()
		
		// Shutdown HTTP server
		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			s.logger.WithError(err).Error("Failed to shutdown server gracefully")
			return errors.Wrap(err, errors.CodeInternal, "server shutdown failed")
		}
		
		s.logger.Info("Server shutdown completed")
		return nil
	}
}

// authenticateRequest verifies the API key and returns the MCP server if valid
func (s *EnhancedServer) authenticateRequest(r *http.Request) *mcp.Server {
	// Get request logger
	reqLogger := logging.LoggerFromContext(r.Context())
	
	// Extract API key from header
	authHeader := r.Header.Get("Authorization")
	const bearerPrefix = "Bearer "
	
	if authHeader == "" {
		reqLogger.Warn("Missing authorization header")
		return nil
	}
	
	if len(authHeader) < len(bearerPrefix) || authHeader[:len(bearerPrefix)] != bearerPrefix {
		reqLogger.Warn("Invalid authorization header format")
		return nil
	}
	
	apiKey := authHeader[len(bearerPrefix):]
	
	// Validate API key
	if apiKey != s.config.APIKey {
		reqLogger.Warn("Invalid API key", 
			"provided_key_prefix", logging.MaskSensitive(apiKey, 4),
		)
		return nil
	}
	
	reqLogger.Debug("Authentication successful")
	return s.mcpServer
}

// registerCapabilities registers all tools and resources with the MCP server.
func (s *EnhancedServer) registerCapabilities() error {
	s.logger.Info("Registering server capabilities")
	
	// Create provider manager and register providers
	providerManager := provider.NewProviderManager()
	
	// Register AWS provider
	awsRegion := s.config.Providers["aws"]["region"]
	if awsRegion == "" {
		awsRegion = "us-west-2" // Default region
	}
	awsProvider := aws.NewAWSProvider(awsRegion)
	providerManager.RegisterProvider(awsProvider)
	s.logger.Info("Registered provider", "provider", "aws", "region", awsRegion)
	
	// Create CAPI client
	var kubeClient *kube.Client
	var err error
	
	if s.config.KubeConfigPath != "" {
		s.logger.Info("Creating Kubernetes client", "kubeconfig", s.config.KubeConfigPath)
		kubeClient, err = kube.NewClient(s.config.KubeConfigPath, s.config.KubeNamespace)
		if err != nil {
			return errors.Wrap(err, errors.CodeInternal, "failed to create Kubernetes client")
		}
		s.logger.Info("Kubernetes client created successfully")
	} else {
		s.logger.Warn("No kubeconfig specified, running in stub mode")
	}
	
	// Create enhanced cluster service
	clusterService := service.NewEnhancedClusterService(kubeClient, s.logger, providerManager)
	
	// Create enhanced tool provider with comprehensive error handling
	toolProvider := tools.NewEnhancedProvider(s.mcpServer, s.logger, clusterService)

	// Register tools with error handling wrapper
	s.logger.Info("Registering MCP tools")
	if err := toolProvider.RegisterTools(); err != nil {
		return errors.Wrap(err, errors.CodeInternal, "failed to register tools")
	}
	
	// Log registered tools
	s.logger.Info("MCP tools registered successfully", 
		"tools", []string{
			"list_clusters",
			"get_cluster", 
			"create_cluster",
			"delete_cluster",
			"scale_cluster",
			"get_cluster_kubeconfig",
			"get_cluster_nodes",
		},
	)
	
	// TODO: Register resources
	s.logger.Debug("Resource registration not yet implemented")
	
	return nil
}

// handleHealth handles health check requests
func (s *EnhancedServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	reqLogger := logging.LoggerFromContext(r.Context())
	reqLogger.Debug("Health check requested")
	
	// Basic health check - server is running
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `{"status":"healthy","version":"%s"}`, s.config.Version)
}

// handleReady handles readiness check requests
func (s *EnhancedServer) handleReady(w http.ResponseWriter, r *http.Request) {
	reqLogger := logging.LoggerFromContext(r.Context())
	reqLogger.Debug("Readiness check requested")
	
	// TODO: Check if all dependencies are ready
	// - Kubernetes API connectivity
	// - Provider availability
	// - MCP server readiness
	
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `{"status":"ready","version":"%s"}`, s.config.Version)
}

// startMetricsServer starts the Prometheus metrics server
func (s *EnhancedServer) startMetricsServer(ctx context.Context) error {
	if s.config.MetricsPort == 0 {
		s.logger.Info("Metrics server disabled")
		return nil
	}
	
	metricsAddr := fmt.Sprintf(":%d", s.config.MetricsPort)
	
	s.logger.Info("Starting metrics server", 
		"port", s.config.MetricsPort,
		"addr", metricsAddr,
	)
	
	// Start metrics server - this will block until context is cancelled
	return metrics.StartMetricsServer(ctx, metricsAddr, s.logger.Logger)
}