package config

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad(t *testing.T) {
	tests := []struct {
		name    string
		envVars map[string]string
		wantErr bool
		checks  func(t *testing.T, cfg *Config)
	}{
		{
			name: "valid configuration with API key",
			envVars: map[string]string{
				"API_KEY": "test-api-key",
			},
			wantErr: false,
			checks: func(t *testing.T, cfg *Config) {
				assert.Equal(t, "test-api-key", cfg.APIKey)
				assert.Equal(t, 8080, cfg.ServerPort)
				assert.Equal(t, 30*time.Second, cfg.ServerTimeout)
				assert.Equal(t, "default", cfg.KubeNamespace)
				assert.Equal(t, "info", cfg.LogLevel)
				assert.Equal(t, "dev", cfg.Version)
			},
		},
		{
			name: "custom configuration values",
			envVars: map[string]string{
				"API_KEY":         "custom-key",
				"SERVER_PORT":     "9000",
				"LOG_LEVEL":       "debug",
				"KUBE_NAMESPACE":  "test-namespace",
				"CLUSTER_TIMEOUT": "15m",
				"METRICS_PORT":    "9091",
				"ENABLE_PPROF":    "true",
				"VERSION":         "v1.0.0",
			},
			wantErr: false,
			checks: func(t *testing.T, cfg *Config) {
				assert.Equal(t, "custom-key", cfg.APIKey)
				assert.Equal(t, 9000, cfg.ServerPort)
				assert.Equal(t, "debug", cfg.LogLevel)
				assert.Equal(t, "test-namespace", cfg.KubeNamespace)
				assert.Equal(t, 15*time.Minute, cfg.ClusterTimeout)
				assert.Equal(t, 9091, cfg.MetricsPort)
				assert.True(t, cfg.EnablePprof)
				assert.Equal(t, "v1.0.0", cfg.Version)
			},
		},
		{
			name:    "missing API key",
			envVars: map[string]string{},
			wantErr: true,
		},
		{
			name: "invalid port number",
			envVars: map[string]string{
				"API_KEY":     "test-key",
				"SERVER_PORT": "invalid",
			},
			wantErr: false, // Should use default port
			checks: func(t *testing.T, cfg *Config) {
				assert.Equal(t, 8080, cfg.ServerPort) // Default value
			},
		},
		{
			name: "invalid duration",
			envVars: map[string]string{
				"API_KEY":         "test-key",
				"CLUSTER_TIMEOUT": "invalid",
			},
			wantErr: false, // Should use default duration
			checks: func(t *testing.T, cfg *Config) {
				assert.Equal(t, 10*time.Minute, cfg.ClusterTimeout) // Default value
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear environment
			clearEnv()
			
			// Set test environment variables
			for key, value := range tt.envVars {
				t.Setenv(key, value)
			}

			cfg, err := Load()

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, cfg)

			if tt.checks != nil {
				tt.checks(t, cfg)
			}
		})
	}
}

func TestGetEnvFunctions(t *testing.T) {
	t.Run("getEnv", func(t *testing.T) {
		t.Setenv("TEST_STRING", "test-value")
		
		assert.Equal(t, "test-value", getEnv("TEST_STRING", "default"))
		assert.Equal(t, "default", getEnv("NON_EXISTENT", "default"))
	})

	t.Run("getEnvInt", func(t *testing.T) {
		t.Setenv("TEST_INT", "123")
		t.Setenv("TEST_INVALID_INT", "invalid")
		
		assert.Equal(t, 123, getEnvInt("TEST_INT", 999))
		assert.Equal(t, 999, getEnvInt("TEST_INVALID_INT", 999))
		assert.Equal(t, 999, getEnvInt("NON_EXISTENT", 999))
	})

	t.Run("getEnvBool", func(t *testing.T) {
		t.Setenv("TEST_BOOL_TRUE", "true")
		t.Setenv("TEST_BOOL_FALSE", "false")
		t.Setenv("TEST_INVALID_BOOL", "invalid")
		
		assert.True(t, getEnvBool("TEST_BOOL_TRUE", false))
		assert.False(t, getEnvBool("TEST_BOOL_FALSE", true))
		assert.True(t, getEnvBool("TEST_INVALID_BOOL", true))
		assert.False(t, getEnvBool("NON_EXISTENT", false))
	})

	t.Run("getEnvDuration", func(t *testing.T) {
		t.Setenv("TEST_DURATION", "5m")
		t.Setenv("TEST_INVALID_DURATION", "invalid")
		
		assert.Equal(t, 5*time.Minute, getEnvDuration("TEST_DURATION", time.Hour))
		assert.Equal(t, time.Hour, getEnvDuration("TEST_INVALID_DURATION", time.Hour))
		assert.Equal(t, time.Hour, getEnvDuration("NON_EXISTENT", time.Hour))
	})
}

func clearEnv() {
	envVars := []string{
		"API_KEY", "SERVER_PORT", "SERVER_TIMEOUT", "SHUTDOWN_GRACE",
		"KUBE_NAMESPACE", "KUBECONFIG", "CLUSTER_TIMEOUT", "LOG_LEVEL",
		"METRICS_PORT", "ENABLE_PPROF", "VERSION", "BUILD_DATE",
	}
	
	for _, key := range envVars {
		os.Unsetenv(key)
	}
}