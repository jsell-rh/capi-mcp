package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config holds the server configuration.
type Config struct {
	// Server configuration
	ServerPort    int           `json:"server_port"`
	ServerTimeout time.Duration `json:"server_timeout"`
	ShutdownGrace time.Duration `json:"shutdown_grace"`

	// Authentication
	APIKey string `json:"-"`

	// Kubernetes configuration
	KubeConfigPath string `json:"kubeconfig_path"`
	KubeNamespace  string `json:"kube_namespace"`

	// CAPI configuration
	ClusterTimeout time.Duration `json:"cluster_timeout"`

	// Provider configuration
	Providers map[string]map[string]string `json:"providers"`

	// Observability
	LogLevel    string `json:"log_level"`
	MetricsPort int    `json:"metrics_port"`
	EnablePprof bool   `json:"enable_pprof"`

	// Version information
	Version   string `json:"version"`
	BuildDate string `json:"build_date"`
}

// Load loads configuration from environment variables.
func Load() (*Config, error) {
	cfg := &Config{
		// Default values
		ServerPort:     getEnvInt("SERVER_PORT", 8080),
		ServerTimeout:  getEnvDuration("SERVER_TIMEOUT", 30*time.Second),
		ShutdownGrace:  getEnvDuration("SHUTDOWN_GRACE", 30*time.Second),
		KubeNamespace:  getEnv("KUBE_NAMESPACE", "default"),
		ClusterTimeout: getEnvDuration("CLUSTER_TIMEOUT", 10*time.Minute),
		LogLevel:       getEnv("LOG_LEVEL", "info"),
		MetricsPort:    getEnvInt("METRICS_PORT", 9090),
		EnablePprof:    getEnvBool("ENABLE_PPROF", false),
		Version:        getEnv("VERSION", "dev"),
		BuildDate:      getEnv("BUILD_DATE", "unknown"),
		Providers:      make(map[string]map[string]string),
	}

	// Required configuration
	apiKey := os.Getenv("API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("API_KEY environment variable is required")
	}
	cfg.APIKey = apiKey

	// Kubernetes configuration
	cfg.KubeConfigPath = getEnv("KUBECONFIG", "")

	return cfg, nil
}

// getEnv gets an environment variable with a default value.
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvInt gets an integer environment variable with a default value.
func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

// getEnvBool gets a boolean environment variable with a default value.
func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolValue, err := strconv.ParseBool(value); err == nil {
			return boolValue
		}
	}
	return defaultValue
}

// getEnvDuration gets a duration environment variable with a default value.
func getEnvDuration(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	return defaultValue
}
