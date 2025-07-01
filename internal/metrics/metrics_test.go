package metrics

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestNewCollector(t *testing.T) {
	collector := NewCollector()

	if collector == nil {
		t.Fatal("Expected collector to be created")
	}

	// Test that we can use the collector methods without errors
	collector.IncRequestsTotal("test", "success")
	collector.ObserveRequestDuration("test", "success", 100*time.Millisecond)
	collector.IncToolInvocations("test", "success")
	collector.ObserveToolExecutionDuration("test", 100*time.Millisecond)
	collector.SetClustersTotal("test", "default", 1)

	// If we reach here without panics, the collector is working
}

func TestCollector_RequestMetrics(t *testing.T) {
	// Create isolated registry
	reg := prometheus.NewRegistry()

	collector := NewCollectorWithRegisterer(reg)

	// Test request metrics
	collector.IncRequestsTotal("list_clusters", "success")
	collector.IncRequestsTotal("create_cluster", "error")
	collector.ObserveRequestDuration("list_clusters", "success", 100*time.Millisecond)

	// Verify counter values
	if value := testutil.ToFloat64(collector.requestsTotal.WithLabelValues("list_clusters", "success")); value != 1 {
		t.Errorf("Expected requests_total to be 1, got %f", value)
	}

	if value := testutil.ToFloat64(collector.requestsTotal.WithLabelValues("create_cluster", "error")); value != 1 {
		t.Errorf("Expected requests_total to be 1, got %f", value)
	}

	// Test active requests
	collector.IncActiveRequests("list_clusters")
	if value := testutil.ToFloat64(collector.activeRequests.WithLabelValues("list_clusters")); value != 1 {
		t.Errorf("Expected active_requests to be 1, got %f", value)
	}

	collector.DecActiveRequests("list_clusters")
	if value := testutil.ToFloat64(collector.activeRequests.WithLabelValues("list_clusters")); value != 0 {
		t.Errorf("Expected active_requests to be 0, got %f", value)
	}
}

func TestCollector_ToolMetrics(t *testing.T) {
	// Create isolated registry
	reg := prometheus.NewRegistry()

	collector := NewCollectorWithRegisterer(reg)

	// Test tool metrics
	collector.IncToolInvocations("create_cluster", "success")
	collector.ObserveToolExecutionDuration("create_cluster", 2*time.Second)
	collector.IncToolErrors("create_cluster", "INVALID_INPUT")

	// Verify values
	if value := testutil.ToFloat64(collector.toolInvocationsTotal.WithLabelValues("create_cluster", "success")); value != 1 {
		t.Errorf("Expected tool_invocations_total to be 1, got %f", value)
	}

	if value := testutil.ToFloat64(collector.toolErrors.WithLabelValues("create_cluster", "INVALID_INPUT")); value != 1 {
		t.Errorf("Expected tool_errors_total to be 1, got %f", value)
	}
}

func TestCollector_KubernetesMetrics(t *testing.T) {
	// Create isolated registry
	reg := prometheus.NewRegistry()

	collector := NewCollectorWithRegisterer(reg)

	// Test Kubernetes API metrics
	collector.IncKubernetesAPICalls("list", "success")
	collector.ObserveKubernetesAPICallDuration("list", 50*time.Millisecond)
	collector.IncKubernetesAPIErrors("create", "TIMEOUT")

	// Verify values
	if value := testutil.ToFloat64(collector.kubernetesAPICallsTotal.WithLabelValues("list", "success")); value != 1 {
		t.Errorf("Expected kubernetes_api_calls_total to be 1, got %f", value)
	}

	if value := testutil.ToFloat64(collector.kubernetesAPIErrors.WithLabelValues("create", "TIMEOUT")); value != 1 {
		t.Errorf("Expected kubernetes_api_errors_total to be 1, got %f", value)
	}
}

func TestCollector_ProviderMetrics(t *testing.T) {
	// Create isolated registry
	reg := prometheus.NewRegistry()

	collector := NewCollectorWithRegisterer(reg)

	// Test provider metrics
	collector.IncProviderOperations("aws", "create_instance", "success")
	collector.ObserveProviderOperationDuration("aws", "create_instance", 30*time.Second)
	collector.IncProviderErrors("aws", "delete_instance", "NOT_FOUND")

	// Verify values
	if value := testutil.ToFloat64(collector.providerOperationsTotal.WithLabelValues("aws", "create_instance", "success")); value != 1 {
		t.Errorf("Expected provider_operations_total to be 1, got %f", value)
	}

	if value := testutil.ToFloat64(collector.providerErrors.WithLabelValues("aws", "delete_instance", "NOT_FOUND")); value != 1 {
		t.Errorf("Expected provider_errors_total to be 1, got %f", value)
	}
}

func TestCollector_ClusterMetrics(t *testing.T) {
	// Create isolated registry
	reg := prometheus.NewRegistry()

	collector := NewCollectorWithRegisterer(reg)

	// Test cluster metrics
	collector.SetClustersTotal("aws", "default", 5)
	collector.IncClusterOperations("create", "aws", "success")

	// Verify values
	if value := testutil.ToFloat64(collector.clustersTotal.WithLabelValues("aws", "default")); value != 5 {
		t.Errorf("Expected clusters_total to be 5, got %f", value)
	}

	if value := testutil.ToFloat64(collector.clusterOperations.WithLabelValues("create", "aws", "success")); value != 1 {
		t.Errorf("Expected cluster_operations_total to be 1, got %f", value)
	}
}

func TestTimer(t *testing.T) {
	timer := NewTimer()

	// Wait a bit
	time.Sleep(10 * time.Millisecond)

	duration := timer.Duration()
	if duration < 10*time.Millisecond {
		t.Errorf("Expected duration to be at least 10ms, got %v", duration)
	}

	if duration > 100*time.Millisecond {
		t.Errorf("Expected duration to be less than 100ms, got %v", duration)
	}
}

func TestMetricsMiddleware_WrapToolExecution(t *testing.T) {
	// Create isolated registry
	reg := prometheus.NewRegistry()

	collector := NewCollectorWithRegisterer(reg)
	logger := slog.Default()
	middleware := NewMetricsMiddleware(collector, logger)

	// Test successful execution
	err := middleware.WrapToolExecution("test_tool", func() error {
		time.Sleep(1 * time.Millisecond) // Simulate work
		return nil
	})

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// Verify metrics were recorded
	if value := testutil.ToFloat64(collector.toolInvocationsTotal.WithLabelValues("test_tool", "success")); value != 1 {
		t.Errorf("Expected tool_invocations_total to be 1, got %f", value)
	}

	if value := testutil.ToFloat64(collector.requestsTotal.WithLabelValues("test_tool", "success")); value != 1 {
		t.Errorf("Expected requests_total to be 1, got %f", value)
	}
}

func TestMetricsMiddleware_WrapKubernetesOperation(t *testing.T) {
	// Create isolated registry
	reg := prometheus.NewRegistry()

	collector := NewCollectorWithRegisterer(reg)
	logger := slog.Default()
	middleware := NewMetricsMiddleware(collector, logger)

	// Test successful execution
	err := middleware.WrapKubernetesOperation("list_pods", func() error {
		return nil
	})

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// Verify metrics were recorded
	if value := testutil.ToFloat64(collector.kubernetesAPICallsTotal.WithLabelValues("list_pods", "success")); value != 1 {
		t.Errorf("Expected kubernetes_api_calls_total to be 1, got %f", value)
	}
}

func TestGetMetricsPort(t *testing.T) {
	tests := []struct {
		addr string
		want int
	}{
		{":9090", 9090},
		{"localhost:8080", 8080},
		{"0.0.0.0:3000", 3000},
		{":8888", 8888},
		{"invalid", 9090}, // default
	}

	for _, tt := range tests {
		t.Run(tt.addr, func(t *testing.T) {
			if got := GetMetricsPort(tt.addr); got != tt.want {
				t.Errorf("GetMetricsPort(%s) = %d, want %d", tt.addr, got, tt.want)
			}
		})
	}
}

func TestStartMetricsServer(t *testing.T) {
	logger := slog.Default()

	// Create a context that we can cancel
	ctx, cancel := context.WithCancel(context.Background())

	// Start server in background
	errCh := make(chan error, 1)
	go func() {
		errCh <- StartMetricsServer(ctx, ":0", logger) // Use :0 for random port
	}()

	// Give the server a moment to start
	time.Sleep(10 * time.Millisecond)

	// Cancel context to stop server
	cancel()

	// Wait for server to stop
	select {
	case err := <-errCh:
		if err != nil {
			t.Errorf("Expected no error from metrics server, got %v", err)
		}
	case <-time.After(1 * time.Second):
		t.Error("Metrics server did not shut down within timeout")
	}
}
