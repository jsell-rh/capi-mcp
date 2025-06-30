package server

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/capi-mcp/capi-mcp-server/internal/config"
	"github.com/capi-mcp/capi-mcp-server/pkg/tools"
)

// Server represents the CAPI MCP server.
type Server struct {
	config    *config.Config
	logger    *slog.Logger
	mcpServer *mcp.Server
}

// New creates a new server instance.
func New(cfg *config.Config, logger *slog.Logger) (*Server, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is required")
	}
	if logger == nil {
		return nil, fmt.Errorf("logger is required")
	}

	// Create MCP server instance with metadata
	mcpServer := mcp.NewServer("capi-mcp-server", cfg.Version, nil)

	// Create server instance
	s := &Server{
		config:    cfg,
		logger:    logger,
		mcpServer: mcpServer,
	}

	// Register tools and resources
	if err := s.registerCapabilities(); err != nil {
		return nil, fmt.Errorf("failed to register capabilities: %w", err)
	}

	return s, nil
}

// Run starts the server and blocks until the context is cancelled.
func (s *Server) Run(ctx context.Context) error {
	s.logger.Info("server starting", 
		"port", s.config.ServerPort,
		"metrics_port", s.config.MetricsPort,
	)

	// Create HTTP handler with authentication middleware
	handler := mcp.NewStreamableHTTPHandler(func(r *http.Request) *mcp.Server {
		// Verify authentication before returning server
		authHeader := r.Header.Get("Authorization")
		const bearerPrefix = "Bearer "
		
		if authHeader == "" || len(authHeader) < len(bearerPrefix) || 
			authHeader[:len(bearerPrefix)] != bearerPrefix ||
			authHeader[len(bearerPrefix):] != s.config.APIKey {
			return nil // This will cause the handler to return 401
		}
		
		return s.mcpServer
	}, nil)

	// Wrap with logging middleware
	loggedHandler := s.loggingMiddleware(handler)

	// Create HTTP server
	httpServer := &http.Server{
		Addr:    fmt.Sprintf(":%d", s.config.ServerPort),
		Handler: loggedHandler,
	}

	// Start server in goroutine
	errCh := make(chan error, 1)
	go func() {
		s.logger.Info("MCP server listening", "addr", httpServer.Addr)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- fmt.Errorf("server error: %w", err)
		}
	}()

	// TODO: Start metrics server
	// TODO: Initialize CAPI client

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		s.logger.Info("server shutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), s.config.ShutdownGrace)
		defer cancel()
		return httpServer.Shutdown(shutdownCtx)
	}
}

// registerCapabilities registers all tools and resources with the MCP server.
func (s *Server) registerCapabilities() error {
	// TODO: Create tool provider with CAPI service
	toolProvider := tools.NewProvider(s.mcpServer, s.logger)

	// Register tools
	if err := toolProvider.RegisterTools(); err != nil {
		return fmt.Errorf("failed to register tools: %w", err)
	}

	// TODO: Register resources

	return nil
}

// loggingMiddleware provides request logging for the HTTP server.
func (s *Server) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Log request
		s.logger.Debug("handling request", 
			"remote_addr", r.RemoteAddr,
			"method", r.Method,
			"path", r.URL.Path,
			"user_agent", r.UserAgent(),
		)

		// Create response writer wrapper to capture status code
		wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		
		// Handle request
		next.ServeHTTP(wrapped, r)
		
		// Log response
		s.logger.Info("request completed",
			"remote_addr", r.RemoteAddr,
			"method", r.Method,
			"path", r.URL.Path,
			"status", wrapped.statusCode,
		)
	})
}

// responseWriter wraps http.ResponseWriter to capture the status code.
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}