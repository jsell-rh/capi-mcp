package server

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/capi-mcp/capi-mcp-server/internal/config"
)

// Server represents the CAPI MCP server.
type Server struct {
	config *config.Config
	logger *slog.Logger
}

// New creates a new server instance.
func New(cfg *config.Config, logger *slog.Logger) (*Server, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is required")
	}
	if logger == nil {
		return nil, fmt.Errorf("logger is required")
	}

	return &Server{
		config: cfg,
		logger: logger,
	}, nil
}

// Run starts the server and blocks until the context is cancelled.
func (s *Server) Run(ctx context.Context) error {
	s.logger.Info("server starting", 
		"port", s.config.ServerPort,
		"metrics_port", s.config.MetricsPort,
	)

	// TODO: Implement MCP server using modelcontextprotocol/go-sdk
	// TODO: Set up metrics server
	// TODO: Set up health checks
	// TODO: Initialize CAPI client
	// TODO: Register tools and resources

	<-ctx.Done()
	
	s.logger.Info("server shutting down")
	return nil
}