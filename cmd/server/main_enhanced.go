package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"runtime/debug"
	"syscall"

	"github.com/capi-mcp/capi-mcp-server/internal/config"
	"github.com/capi-mcp/capi-mcp-server/internal/logging"
	"github.com/capi-mcp/capi-mcp-server/internal/server"
)

// Version is set at build time
var Version = "dev"

func main() {
	// Handle panics at the top level
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(os.Stderr, "PANIC: %v\n%s\n", r, debug.Stack())
			os.Exit(1)
		}
	}()
	
	// Load configuration first to get log level
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load configuration: %v\n", err)
		os.Exit(1)
	}
	
	// Set version if not provided
	if cfg.Version == "" {
		cfg.Version = Version
	}
	
	// Create logger based on configuration
	logLevel := logging.ParseLevel(cfg.LogLevel)
	logger := logging.NewLogger(logLevel, "json").WithComponent("main")
	
	logger.Info("Starting CAPI MCP Server",
		"version", cfg.Version,
		"log_level", cfg.LogLevel,
		"go_version", getGoVersion(),
	)
	
	// Log configuration (without sensitive data)
	logger.Debug("Configuration loaded",
		"server_port", cfg.ServerPort,
		"metrics_port", cfg.MetricsPort,
		"namespace", cfg.KubeNamespace,
		"providers", getProviderNames(cfg.Providers),
	)
	
	// Create server with enhanced error handling
	srv, err := server.NewEnhanced(cfg)
	if err != nil {
		logger.WithError(err).Error("Failed to create server")
		os.Exit(1)
	}
	
	// Setup signal handling
	ctx, cancel := setupSignalHandling(logger)
	defer cancel()
	
	// Run server
	logger.Info("Server starting...")
	if err := srv.Run(ctx); err != nil {
		logger.WithError(err).Error("Server error")
		os.Exit(1)
	}
	
	logger.Info("Server shutdown completed successfully")
}

// setupSignalHandling creates a context that cancels on interrupt signals
func setupSignalHandling(logger *logging.Logger) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())
	
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM, syscall.SIGQUIT)
	
	go func() {
		sig := <-sigChan
		logger.Info("Received shutdown signal", "signal", sig.String())
		cancel()
		
		// If we get another signal, force exit
		sig = <-sigChan
		logger.Warn("Received second signal, forcing shutdown", "signal", sig.String())
		os.Exit(1)
	}()
	
	return ctx, cancel
}

// getGoVersion returns the Go runtime version
func getGoVersion() string {
	if info, ok := debug.ReadBuildInfo(); ok {
		return info.GoVersion
	}
	return "unknown"
}

// getProviderNames extracts provider names from the config
func getProviderNames(providers map[string]map[string]string) []string {
	names := make([]string, 0, len(providers))
	for name := range providers {
		names = append(names, name)
	}
	return names
}