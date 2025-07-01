package metrics

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"log/slog"
)

const (
	// Metric name prefixes
	metricPrefix = "capi_mcp_"

	// Common label names
	LabelTool      = "tool"
	LabelStatus    = "status"
	LabelOperation = "operation"
	LabelComponent = "component"
	LabelProvider  = "provider"
	LabelCluster   = "cluster"
	LabelNamespace = "namespace"
	LabelErrorCode = "error_code"
)

// Collector holds all Prometheus metrics
type Collector struct {
	// Request metrics
	requestsTotal   *prometheus.CounterVec
	requestDuration *prometheus.HistogramVec
	activeRequests  *prometheus.GaugeVec

	// Tool metrics
	toolInvocationsTotal  *prometheus.CounterVec
	toolExecutionDuration *prometheus.HistogramVec
	toolErrors            *prometheus.CounterVec

	// Kubernetes API metrics
	kubernetesAPICallsTotal   *prometheus.CounterVec
	kubernetesAPICallDuration *prometheus.HistogramVec
	kubernetesAPIErrors       *prometheus.CounterVec

	// Provider metrics
	providerOperationsTotal   *prometheus.CounterVec
	providerOperationDuration *prometheus.HistogramVec
	providerErrors            *prometheus.CounterVec

	// Cluster metrics
	clustersTotal     *prometheus.GaugeVec
	clusterOperations *prometheus.CounterVec

	// System metrics
	serverInfo *prometheus.GaugeVec
	buildInfo  *prometheus.GaugeVec
}

// NewCollector creates a new metrics collector with all metrics registered
func NewCollector() *Collector {
	return NewCollectorWithRegisterer(prometheus.DefaultRegisterer)
}

// NewCollectorWithRegisterer creates a new metrics collector with a custom registerer
func NewCollectorWithRegisterer(registerer prometheus.Registerer) *Collector {
	c := &Collector{
		// Request metrics
		requestsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: metricPrefix + "requests_total",
				Help: "Total number of MCP requests handled",
			},
			[]string{LabelTool, LabelStatus},
		),

		requestDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    metricPrefix + "request_duration_seconds",
				Help:    "Duration of MCP requests in seconds",
				Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
			},
			[]string{LabelTool, LabelStatus},
		),

		activeRequests: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: metricPrefix + "active_requests",
				Help: "Number of currently active MCP requests",
			},
			[]string{LabelTool},
		),

		// Tool metrics
		toolInvocationsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: metricPrefix + "tool_invocations_total",
				Help: "Total number of tool invocations",
			},
			[]string{LabelTool, LabelStatus},
		),

		toolExecutionDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    metricPrefix + "tool_execution_duration_seconds",
				Help:    "Duration of tool execution in seconds",
				Buckets: []float64{0.01, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30, 60},
			},
			[]string{LabelTool},
		),

		toolErrors: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: metricPrefix + "tool_errors_total",
				Help: "Total number of tool execution errors",
			},
			[]string{LabelTool, LabelErrorCode},
		),

		// Kubernetes API metrics
		kubernetesAPICallsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: metricPrefix + "kubernetes_api_calls_total",
				Help: "Total number of Kubernetes API calls",
			},
			[]string{LabelOperation, LabelStatus},
		),

		kubernetesAPICallDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    metricPrefix + "kubernetes_api_call_duration_seconds",
				Help:    "Duration of Kubernetes API calls in seconds",
				Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5},
			},
			[]string{LabelOperation},
		),

		kubernetesAPIErrors: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: metricPrefix + "kubernetes_api_errors_total",
				Help: "Total number of Kubernetes API errors",
			},
			[]string{LabelOperation, LabelErrorCode},
		),

		// Provider metrics
		providerOperationsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: metricPrefix + "provider_operations_total",
				Help: "Total number of provider operations",
			},
			[]string{LabelProvider, LabelOperation, LabelStatus},
		),

		providerOperationDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    metricPrefix + "provider_operation_duration_seconds",
				Help:    "Duration of provider operations in seconds",
				Buckets: []float64{0.1, 0.5, 1, 5, 10, 30, 60, 120, 300, 600, 900},
			},
			[]string{LabelProvider, LabelOperation},
		),

		providerErrors: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: metricPrefix + "provider_errors_total",
				Help: "Total number of provider operation errors",
			},
			[]string{LabelProvider, LabelOperation, LabelErrorCode},
		),

		// Cluster metrics
		clustersTotal: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: metricPrefix + "clusters_total",
				Help: "Total number of managed clusters",
			},
			[]string{LabelProvider, LabelNamespace},
		),

		clusterOperations: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: metricPrefix + "cluster_operations_total",
				Help: "Total number of cluster operations",
			},
			[]string{LabelOperation, LabelProvider, LabelStatus},
		),

		// System metrics
		serverInfo: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: metricPrefix + "server_info",
				Help: "Server information",
			},
			[]string{"version", "build_time", "go_version"},
		),

		buildInfo: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: metricPrefix + "build_info",
				Help: "Build information",
			},
			[]string{"version", "revision", "branch", "build_user", "build_date"},
		),
	}

	// Register all metrics
	registerer.MustRegister(
		c.requestsTotal,
		c.requestDuration,
		c.activeRequests,
		c.toolInvocationsTotal,
		c.toolExecutionDuration,
		c.toolErrors,
		c.kubernetesAPICallsTotal,
		c.kubernetesAPICallDuration,
		c.kubernetesAPIErrors,
		c.providerOperationsTotal,
		c.providerOperationDuration,
		c.providerErrors,
		c.clustersTotal,
		c.clusterOperations,
		c.serverInfo,
		c.buildInfo,
	)

	return c
}

// Request metrics methods

// IncRequestsTotal increments the total request counter
func (c *Collector) IncRequestsTotal(tool, status string) {
	c.requestsTotal.WithLabelValues(tool, status).Inc()
}

// ObserveRequestDuration records request duration
func (c *Collector) ObserveRequestDuration(tool, status string, duration time.Duration) {
	c.requestDuration.WithLabelValues(tool, status).Observe(duration.Seconds())
}

// IncActiveRequests increments active requests gauge
func (c *Collector) IncActiveRequests(tool string) {
	c.activeRequests.WithLabelValues(tool).Inc()
}

// DecActiveRequests decrements active requests gauge
func (c *Collector) DecActiveRequests(tool string) {
	c.activeRequests.WithLabelValues(tool).Dec()
}

// Tool metrics methods

// IncToolInvocations increments tool invocation counter
func (c *Collector) IncToolInvocations(tool, status string) {
	c.toolInvocationsTotal.WithLabelValues(tool, status).Inc()
}

// ObserveToolExecutionDuration records tool execution duration
func (c *Collector) ObserveToolExecutionDuration(tool string, duration time.Duration) {
	c.toolExecutionDuration.WithLabelValues(tool).Observe(duration.Seconds())
}

// IncToolErrors increments tool error counter
func (c *Collector) IncToolErrors(tool, errorCode string) {
	c.toolErrors.WithLabelValues(tool, errorCode).Inc()
}

// Kubernetes API metrics methods

// IncKubernetesAPICalls increments Kubernetes API call counter
func (c *Collector) IncKubernetesAPICalls(operation, status string) {
	c.kubernetesAPICallsTotal.WithLabelValues(operation, status).Inc()
}

// ObserveKubernetesAPICallDuration records Kubernetes API call duration
func (c *Collector) ObserveKubernetesAPICallDuration(operation string, duration time.Duration) {
	c.kubernetesAPICallDuration.WithLabelValues(operation).Observe(duration.Seconds())
}

// IncKubernetesAPIErrors increments Kubernetes API error counter
func (c *Collector) IncKubernetesAPIErrors(operation, errorCode string) {
	c.kubernetesAPIErrors.WithLabelValues(operation, errorCode).Inc()
}

// Provider metrics methods

// IncProviderOperations increments provider operation counter
func (c *Collector) IncProviderOperations(provider, operation, status string) {
	c.providerOperationsTotal.WithLabelValues(provider, operation, status).Inc()
}

// ObserveProviderOperationDuration records provider operation duration
func (c *Collector) ObserveProviderOperationDuration(provider, operation string, duration time.Duration) {
	c.providerOperationDuration.WithLabelValues(provider, operation).Observe(duration.Seconds())
}

// IncProviderErrors increments provider error counter
func (c *Collector) IncProviderErrors(provider, operation, errorCode string) {
	c.providerErrors.WithLabelValues(provider, operation, errorCode).Inc()
}

// Cluster metrics methods

// SetClustersTotal sets the total number of clusters
func (c *Collector) SetClustersTotal(provider, namespace string, count float64) {
	c.clustersTotal.WithLabelValues(provider, namespace).Set(count)
}

// IncClusterOperations increments cluster operation counter
func (c *Collector) IncClusterOperations(operation, provider, status string) {
	c.clusterOperations.WithLabelValues(operation, provider, status).Inc()
}

// System metrics methods

// SetServerInfo sets server information
func (c *Collector) SetServerInfo(version, buildTime, goVersion string) {
	c.serverInfo.WithLabelValues(version, buildTime, goVersion).Set(1)
}

// SetBuildInfo sets build information
func (c *Collector) SetBuildInfo(version, revision, branch, buildUser, buildDate string) {
	c.buildInfo.WithLabelValues(version, revision, branch, buildUser, buildDate).Set(1)
}

// Timer helps measure operation duration
type Timer struct {
	startTime time.Time
}

// NewTimer creates a new timer
func NewTimer() *Timer {
	return &Timer{
		startTime: time.Now(),
	}
}

// Duration returns the elapsed time since timer creation
func (t *Timer) Duration() time.Duration {
	return time.Since(t.startTime)
}

// MetricsMiddleware provides middleware for automatic metrics collection
type MetricsMiddleware struct {
	collector *Collector
	logger    *slog.Logger
}

// NewMetricsMiddleware creates a new metrics middleware
func NewMetricsMiddleware(collector *Collector, logger *slog.Logger) *MetricsMiddleware {
	return &MetricsMiddleware{
		collector: collector,
		logger:    logger,
	}
}

// WrapToolExecution wraps a tool execution with metrics collection
func (m *MetricsMiddleware) WrapToolExecution(tool string, fn func() error) error {
	timer := NewTimer()

	// Track active request
	m.collector.IncActiveRequests(tool)
	defer m.collector.DecActiveRequests(tool)

	// Execute function
	err := fn()

	// Record metrics
	duration := timer.Duration()
	status := "success"
	if err != nil {
		status = "error"
		m.collector.IncToolErrors(tool, "unknown") // TODO: Extract error code from error
	}

	m.collector.IncToolInvocations(tool, status)
	m.collector.ObserveToolExecutionDuration(tool, duration)
	m.collector.IncRequestsTotal(tool, status)
	m.collector.ObserveRequestDuration(tool, status, duration)

	return err
}

// WrapKubernetesOperation wraps a Kubernetes operation with metrics collection
func (m *MetricsMiddleware) WrapKubernetesOperation(operation string, fn func() error) error {
	timer := NewTimer()

	// Execute function
	err := fn()

	// Record metrics
	duration := timer.Duration()
	status := "success"
	if err != nil {
		status = "error"
		m.collector.IncKubernetesAPIErrors(operation, "unknown") // TODO: Extract error code
	}

	m.collector.IncKubernetesAPICalls(operation, status)
	m.collector.ObserveKubernetesAPICallDuration(operation, duration)

	return err
}

// WrapProviderOperation wraps a provider operation with metrics collection
func (m *MetricsMiddleware) WrapProviderOperation(provider, operation string, fn func() error) error {
	timer := NewTimer()

	// Execute function
	err := fn()

	// Record metrics
	duration := timer.Duration()
	status := "success"
	if err != nil {
		status = "error"
		m.collector.IncProviderErrors(provider, operation, "unknown") // TODO: Extract error code
	}

	m.collector.IncProviderOperations(provider, operation, status)
	m.collector.ObserveProviderOperationDuration(provider, operation, duration)

	return err
}

// StartMetricsServer starts the Prometheus metrics HTTP server
func StartMetricsServer(ctx context.Context, addr string, logger *slog.Logger) error {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())

	// Add health check endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte("OK")); err != nil {
			// Log error but don't fail the handler
			slog.Default().Error("Failed to write health check response", "error", err)
		}
	})

	server := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
		ReadHeaderTimeout: 10 * time.Second, // Prevents Slowloris attacks
	}

	// Start server in goroutine
	go func() {
		logger.Info("Starting metrics server", "addr", addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("Metrics server error", "error", err)
		}
	}()

	// Wait for context cancellation
	<-ctx.Done()

	// Graceful shutdown
	logger.Info("Shutting down metrics server")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return server.Shutdown(shutdownCtx)
}

// GetMetricsPort extracts port number from address string
func GetMetricsPort(addr string) int {
	// Parse port from address (e.g., ":9090" -> 9090, "localhost:9090" -> 9090)
	if addr[0] == ':' {
		if port, err := strconv.Atoi(addr[1:]); err == nil {
			return port
		}
	}

	// Try to extract port after last colon
	lastColon := -1
	for i := len(addr) - 1; i >= 0; i-- {
		if addr[i] == ':' {
			lastColon = i
			break
		}
	}

	if lastColon != -1 && lastColon < len(addr)-1 {
		if port, err := strconv.Atoi(addr[lastColon+1:]); err == nil {
			return port
		}
	}

	return 9090 // Default port
}
