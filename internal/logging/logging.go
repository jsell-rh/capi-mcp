package logging

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"runtime"
	"strings"
	"time"
)

// Common log field keys
const (
	// Context fields
	FieldRequestID = "request_id"
	FieldTraceID   = "trace_id"
	FieldUserID    = "user_id"
	FieldOperation = "operation"
	FieldComponent = "component"

	// Resource fields
	FieldClusterName      = "cluster_name"
	FieldClusterNamespace = "cluster_namespace"
	FieldNamespace        = "namespace"
	FieldResourceKind     = "resource_kind"
	FieldResourceName     = "resource_name"
	FieldProvider         = "provider"

	// Error fields
	FieldError      = "error"
	FieldErrorCode  = "error_code"
	FieldStackTrace = "stack_trace"

	// Performance fields
	FieldDuration  = "duration_ms"
	FieldStartTime = "start_time"
	FieldEndTime   = "end_time"

	// Tool fields
	FieldTool       = "tool"
	FieldToolInput  = "tool_input"
	FieldToolOutput = "tool_output"

	// HTTP request fields
	FieldUserAgent     = "user_agent"
	FieldRemoteAddr    = "remote_addr"
	FieldStatusCode    = "status_code"
	FieldContentLength = "content_length"
)

// Logger wraps slog.Logger with additional functionality
type Logger struct {
	*slog.Logger
	metricsCollector MetricsCollector
}

// MetricsCollector interface for recording metrics alongside logs
type MetricsCollector interface {
	IncRequestsTotal(tool, status string)
	ObserveRequestDuration(tool, status string, duration time.Duration)
	IncActiveRequests(tool string)
	DecActiveRequests(tool string)
	IncToolInvocations(tool, status string)
	ObserveToolExecutionDuration(tool string, duration time.Duration)
	IncToolErrors(tool, errorCode string)
	IncKubernetesAPICalls(operation, status string)
	ObserveKubernetesAPICallDuration(operation string, duration time.Duration)
	IncKubernetesAPIErrors(operation, errorCode string)
}

// contextKey is a custom type for context keys
type contextKey string

const (
	loggerKey    contextKey = "logger"
	requestIDKey contextKey = "request_id"
	traceIDKey   contextKey = "trace_id"
)

// NewLogger creates a new logger with the specified configuration
func NewLogger(level slog.Level, format string) *Logger {
	return NewLoggerWithMetrics(level, format, nil)
}

// NewLoggerWithMetrics creates a new logger with metrics collection
func NewLoggerWithMetrics(level slog.Level, format string, metricsCollector MetricsCollector) *Logger {
	var handler slog.Handler

	opts := &slog.HandlerOptions{
		Level: level,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			// Customize time format
			if a.Key == slog.TimeKey {
				if t, ok := a.Value.Any().(time.Time); ok {
					a.Value = slog.StringValue(t.Format(time.RFC3339))
				}
			}
			// Add source location for errors
			if a.Key == slog.SourceKey {
				if src, ok := a.Value.Any().(*slog.Source); ok {
					a.Value = slog.StringValue(fmt.Sprintf("%s:%d", src.File, src.Line))
				}
			}
			return a
		},
	}

	switch strings.ToLower(format) {
	case "json":
		handler = slog.NewJSONHandler(os.Stdout, opts)
	default:
		handler = slog.NewTextHandler(os.Stdout, opts)
	}

	return &Logger{
		Logger:           slog.New(handler),
		metricsCollector: metricsCollector,
	}
}

// WithContext returns a logger with context fields
func (l *Logger) WithContext(ctx context.Context) *Logger {
	attrs := []slog.Attr{}

	// Add request ID if present
	if requestID := GetRequestID(ctx); requestID != "" {
		attrs = append(attrs, slog.String(FieldRequestID, requestID))
	}

	// Add trace ID if present
	if traceID := GetTraceID(ctx); traceID != "" {
		attrs = append(attrs, slog.String(FieldTraceID, traceID))
	}

	if len(attrs) > 0 {
		// Convert []slog.Attr to []any for With method
		args := make([]any, len(attrs))
		for i, attr := range attrs {
			args[i] = attr
		}
		return &Logger{
			Logger:           l.Logger.With(args...),
			metricsCollector: l.metricsCollector,
		}
	}

	return l
}

// WithComponent returns a logger for a specific component
func (l *Logger) WithComponent(component string) *Logger {
	return &Logger{
		Logger:           l.Logger.With(slog.String(FieldComponent, component)),
		metricsCollector: l.metricsCollector,
	}
}

// WithOperation returns a logger for a specific operation
func (l *Logger) WithOperation(operation string) *Logger {
	return &Logger{
		Logger:           l.Logger.With(slog.String(FieldOperation, operation)),
		metricsCollector: l.metricsCollector,
	}
}

// WithCluster returns a logger with cluster context
func (l *Logger) WithCluster(clusterName, namespace string) *Logger {
	return &Logger{
		Logger: l.Logger.With(
			slog.String(FieldClusterName, clusterName),
			slog.String(FieldClusterNamespace, namespace),
		),
		metricsCollector: l.metricsCollector,
	}
}

// WithResource returns a logger with resource context
func (l *Logger) WithResource(kind, name, namespace string) *Logger {
	return &Logger{
		Logger: l.Logger.With(
			slog.String(FieldResourceKind, kind),
			slog.String(FieldResourceName, name),
			slog.String(FieldNamespace, namespace),
		),
		metricsCollector: l.metricsCollector,
	}
}

// WithError returns a logger with error context
func (l *Logger) WithError(err error) *Logger {
	if err == nil {
		return l
	}

	attrs := []slog.Attr{
		slog.String(FieldError, err.Error()),
	}

	// Add stack trace for debugging
	if slog.Default().Enabled(context.Background(), slog.LevelDebug) {
		attrs = append(attrs, slog.String(FieldStackTrace, getStackTrace()))
	}

	// Convert []slog.Attr to []any for With method
	args := make([]any, len(attrs))
	for i, attr := range attrs {
		args[i] = attr
	}

	return &Logger{
		Logger:           l.Logger.With(args...),
		metricsCollector: l.metricsCollector,
	}
}

// LogOperation logs the start and end of an operation with duration
func (l *Logger) LogOperation(ctx context.Context, operation string, fn func() error) error {
	startTime := time.Now()

	opLogger := l.WithContext(ctx).WithOperation(operation)
	opLogger.Info("Starting operation",
		slog.Time(FieldStartTime, startTime),
	)

	err := fn()

	duration := time.Since(startTime)
	endTime := time.Now()

	fields := []slog.Attr{
		slog.Time(FieldEndTime, endTime),
		slog.Int64(FieldDuration, duration.Milliseconds()),
	}

	// Convert fields to []any
	fieldArgs := make([]any, len(fields))
	for i, field := range fields {
		fieldArgs[i] = field
	}

	if err != nil {
		opLogger.WithError(err).Error("Operation failed", fieldArgs...)
	} else {
		opLogger.Info("Operation completed", fieldArgs...)
	}

	return err
}

// LogToolCall logs MCP tool invocations with metrics collection
func (l *Logger) LogToolCall(ctx context.Context, toolName string, input interface{}, fn func() (interface{}, error)) (interface{}, error) {
	startTime := time.Now()

	toolLogger := &Logger{
		Logger: l.WithContext(ctx).Logger.With(
			slog.String(FieldTool, toolName),
			slog.Any(FieldToolInput, input),
		),
		metricsCollector: l.metricsCollector,
	}

	// Track active request if metrics available
	if l.metricsCollector != nil {
		l.metricsCollector.IncActiveRequests(toolName)
		defer l.metricsCollector.DecActiveRequests(toolName)
	}

	toolLogger.Info("Tool invocation started")

	output, err := fn()

	duration := time.Since(startTime)

	fields := []slog.Attr{
		slog.Int64(FieldDuration, duration.Milliseconds()),
	}

	// Convert fields to []any
	fieldArgs := make([]any, len(fields))
	for i, field := range fields {
		fieldArgs[i] = field
	}

	status := "success"
	if err != nil {
		status = "error"
		errorLogger := &Logger{
			Logger:           toolLogger.Logger.With(slog.String(FieldError, err.Error())),
			metricsCollector: l.metricsCollector,
		}
		errorLogger.Error("Tool invocation failed", fieldArgs...)

		// Record error metrics
		if l.metricsCollector != nil {
			l.metricsCollector.IncToolErrors(toolName, "unknown") // TODO: Extract error code
		}
	} else {
		successFields := append(fields, slog.Any(FieldToolOutput, output))
		successArgs := make([]any, len(successFields))
		for i, field := range successFields {
			successArgs[i] = field
		}
		toolLogger.Info("Tool invocation completed", successArgs...)
	}

	// Record metrics
	if l.metricsCollector != nil {
		l.metricsCollector.IncToolInvocations(toolName, status)
		l.metricsCollector.ObserveToolExecutionDuration(toolName, duration)
		l.metricsCollector.IncRequestsTotal(toolName, status)
		l.metricsCollector.ObserveRequestDuration(toolName, status, duration)
	}

	return output, err
}

// LogKubernetesOperation logs Kubernetes API operations with metrics collection
func (l *Logger) LogKubernetesOperation(ctx context.Context, operation string, fn func() error) error {
	startTime := time.Now()

	opLogger := l.WithContext(ctx).WithOperation(operation)
	opLogger.Debug("Kubernetes operation started")

	err := fn()

	duration := time.Since(startTime)

	status := "success"
	if err != nil {
		status = "error"
		opLogger.WithError(err).Error("Kubernetes operation failed",
			slog.Int64(FieldDuration, duration.Milliseconds()))

		// Record error metrics
		if l.metricsCollector != nil {
			l.metricsCollector.IncKubernetesAPIErrors(operation, "unknown") // TODO: Extract error code
		}
	} else {
		opLogger.Debug("Kubernetes operation completed",
			slog.Int64(FieldDuration, duration.Milliseconds()))
	}

	// Record metrics
	if l.metricsCollector != nil {
		l.metricsCollector.IncKubernetesAPICalls(operation, status)
		l.metricsCollector.ObserveKubernetesAPICallDuration(operation, duration)
	}

	return err
}

// Context management functions

// ContextWithLogger adds a logger to the context
func ContextWithLogger(ctx context.Context, logger *Logger) context.Context {
	return context.WithValue(ctx, loggerKey, logger)
}

// LoggerFromContext retrieves the logger from context
func LoggerFromContext(ctx context.Context) *Logger {
	if logger, ok := ctx.Value(loggerKey).(*Logger); ok {
		return logger
	}
	// Return default logger if not found
	return &Logger{
		Logger:           slog.Default(),
		metricsCollector: nil,
	}
}

// LoggerToContext adds a logger to the context (alias for ContextWithLogger)
func LoggerToContext(ctx context.Context, logger *Logger) context.Context {
	return ContextWithLogger(ctx, logger)
}

// ContextWithRequestID adds a request ID to the context
func ContextWithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, requestIDKey, requestID)
}

// GetRequestID retrieves the request ID from context
func GetRequestID(ctx context.Context) string {
	if id, ok := ctx.Value(requestIDKey).(string); ok {
		return id
	}
	return ""
}

// ContextWithTraceID adds a trace ID to the context
func ContextWithTraceID(ctx context.Context, traceID string) context.Context {
	return context.WithValue(ctx, traceIDKey, traceID)
}

// GetTraceID retrieves the trace ID from context
func GetTraceID(ctx context.Context) string {
	if id, ok := ctx.Value(traceIDKey).(string); ok {
		return id
	}
	return ""
}

// Helper functions

// getStackTrace returns the current stack trace
func getStackTrace() string {
	buf := make([]byte, 1024)
	for {
		n := runtime.Stack(buf, false)
		if n < len(buf) {
			return string(buf[:n])
		}
		buf = make([]byte, len(buf)*2)
	}
}

// MaskSensitive masks sensitive data in logs
func MaskSensitive(value string, showChars int) string {
	if showChars <= 0 {
		return "***"
	}

	if len(value) == 0 {
		return "***"
	}

	// If string is shorter than or equal to showChars, mask it for security
	// unless showChars is much larger than the string (indicating we want to see it all)
	if len(value) <= showChars {
		if showChars > len(value)*2 { // Much larger than string, show all
			return value
		}
		return "***" // Close to string length, mask for security
	}

	return value[:showChars] + "***"
}

// LoggerConfig represents logger configuration
type LoggerConfig struct {
	Level  string `json:"level" env:"LOG_LEVEL" default:"info"`
	Format string `json:"format" env:"LOG_FORMAT" default:"json"`
}

// ParseLevel converts a string log level to slog.Level
func ParseLevel(level string) slog.Level {
	switch strings.ToLower(level) {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
