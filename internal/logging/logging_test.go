package logging

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"
)

func TestNewLogger(t *testing.T) {
	tests := []struct {
		name   string
		level  slog.Level
		format string
	}{
		{
			name:   "debug json",
			level:  slog.LevelDebug,
			format: "json",
		},
		{
			name:   "info text",
			level:  slog.LevelInfo,
			format: "text",
		},
		{
			name:   "warn json",
			level:  slog.LevelWarn,
			format: "json",
		},
		{
			name:   "error text",
			level:  slog.LevelError,
			format: "text",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := NewLogger(tt.level, tt.format)
			if logger == nil {
				t.Error("NewLogger() returned nil")
			}
		})
	}
}

func TestParseLevel(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected slog.Level
	}{
		{
			name:     "debug",
			input:    "debug",
			expected: slog.LevelDebug,
		},
		{
			name:     "info",
			input:    "info",
			expected: slog.LevelInfo,
		},
		{
			name:     "warn",
			input:    "warn",
			expected: slog.LevelWarn,
		},
		{
			name:     "error",
			input:    "error",
			expected: slog.LevelError,
		},
		{
			name:     "uppercase",
			input:    "INFO",
			expected: slog.LevelInfo,
		},
		{
			name:     "mixed case",
			input:    "WaRn",
			expected: slog.LevelWarn,
		},
		{
			name:     "invalid",
			input:    "invalid",
			expected: slog.LevelInfo,
		},
		{
			name:     "empty",
			input:    "",
			expected: slog.LevelInfo,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ParseLevel(tt.input); got != tt.expected {
				t.Errorf("ParseLevel() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestLogger_WithComponent(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(slog.LevelDebug, "json")
	logger.Logger = slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	componentLogger := logger.WithComponent("test-component")
	componentLogger.Info("test message")

	// Parse the JSON output
	var logEntry map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &logEntry); err != nil {
		t.Fatalf("Failed to parse log output as JSON: %v", err)
	}

	if logEntry["component"] != "test-component" {
		t.Errorf("Expected component 'test-component', got %v", logEntry["component"])
	}
}

func TestLogger_WithOperation(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(slog.LevelDebug, "json")
	logger.Logger = slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	opLogger := logger.WithOperation("test-operation")
	opLogger.Info("test message")

	// Parse the JSON output
	var logEntry map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &logEntry); err != nil {
		t.Fatalf("Failed to parse log output as JSON: %v", err)
	}

	if logEntry["operation"] != "test-operation" {
		t.Errorf("Expected operation 'test-operation', got %v", logEntry["operation"])
	}
}

func TestLogger_WithCluster(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(slog.LevelDebug, "json")
	logger.Logger = slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	clusterLogger := logger.WithCluster("test-cluster", "test-namespace")
	clusterLogger.Info("test message")

	// Parse the JSON output
	var logEntry map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &logEntry); err != nil {
		t.Fatalf("Failed to parse log output as JSON: %v", err)
	}

	if logEntry["cluster_name"] != "test-cluster" {
		t.Errorf("Expected cluster_name 'test-cluster', got %v", logEntry["cluster_name"])
	}

	if logEntry["cluster_namespace"] != "test-namespace" {
		t.Errorf("Expected cluster_namespace 'test-namespace', got %v", logEntry["cluster_namespace"])
	}
}

func TestLogger_WithError(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(slog.LevelDebug, "json")
	logger.Logger = slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	testErr := errors.New("test error")
	errLogger := logger.WithError(testErr)
	errLogger.Error("test message")

	// Parse the JSON output
	var logEntry map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &logEntry); err != nil {
		t.Fatalf("Failed to parse log output as JSON: %v", err)
	}

	if logEntry["error"] != "test error" {
		t.Errorf("Expected error 'test error', got %v", logEntry["error"])
	}
}

func TestLogger_LogOperation(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(slog.LevelDebug, "json")
	logger.Logger = slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	ctx := context.Background()

	// Test successful operation
	err := logger.LogOperation(ctx, "test-op", func() error {
		time.Sleep(10 * time.Millisecond) // Simulate some work
		return nil
	})

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// Check that log entries were written
	logOutput := buf.String()
	if !strings.Contains(logOutput, "test-op") {
		t.Error("Expected operation name in log output")
	}
	if !strings.Contains(logOutput, "Starting operation") {
		t.Error("Expected 'Starting operation' in log output")
	}
	if !strings.Contains(logOutput, "Operation completed") {
		t.Error("Expected 'Operation completed' in log output")
	}
}

func TestLogger_LogOperation_WithError(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(slog.LevelDebug, "json")
	logger.Logger = slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	ctx := context.Background()
	testErr := errors.New("operation failed")

	// Test operation that returns an error
	err := logger.LogOperation(ctx, "failing-op", func() error {
		return testErr
	})

	if err != testErr {
		t.Errorf("Expected error %v, got %v", testErr, err)
	}

	// Check that error was logged
	logOutput := buf.String()
	if !strings.Contains(logOutput, "failing-op") {
		t.Error("Expected operation name in log output")
	}
	if !strings.Contains(logOutput, "Operation failed") {
		t.Error("Expected 'Operation failed' in log output")
	}
	if !strings.Contains(logOutput, "operation failed") {
		t.Error("Expected error message in log output")
	}
}

func TestLogger_LogToolCall(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(slog.LevelDebug, "json")
	logger.Logger = slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	ctx := context.Background()
	input := map[string]interface{}{
		"cluster_name": "test-cluster",
		"replicas":     3,
	}

	// Test successful tool call
	result, err := logger.LogToolCall(ctx, "create_cluster", input, func() (interface{}, error) {
		return map[string]interface{}{
			"success": true,
			"cluster": "test-cluster",
		}, nil
	})

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	expectedResult := map[string]interface{}{
		"success": true,
		"cluster": "test-cluster",
	}

	resultMap, ok := result.(map[string]interface{})
	if !ok {
		t.Errorf("Expected result to be map[string]interface{}, got %T", result)
	}

	if resultMap["success"] != expectedResult["success"] {
		t.Errorf("Expected success %v, got %v", expectedResult["success"], resultMap["success"])
	}

	// Check log output
	logOutput := buf.String()
	if !strings.Contains(logOutput, "create_cluster") {
		t.Error("Expected tool name in log output")
	}
	if !strings.Contains(logOutput, "Tool invocation started") {
		t.Error("Expected 'Tool invocation started' in log output")
	}
	if !strings.Contains(logOutput, "Tool invocation completed") {
		t.Error("Expected 'Tool invocation completed' in log output")
	}
}

func TestMaskSensitive(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		keep     int
		expected string
	}{
		{
			name:     "normal string",
			input:    "abcdefghijk",
			keep:     3,
			expected: "abc***",
		},
		{
			name:     "short string",
			input:    "ab",
			keep:     3,
			expected: "***",
		},
		{
			name:     "empty string",
			input:    "",
			keep:     3,
			expected: "***",
		},
		{
			name:     "zero keep",
			input:    "abcdefgh",
			keep:     0,
			expected: "***",
		},
		{
			name:     "keep longer than string",
			input:    "abc",
			keep:     10,
			expected: "abc",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := MaskSensitive(tt.input, tt.keep); got != tt.expected {
				t.Errorf("MaskSensitive() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestLoggerFromContext(t *testing.T) {
	// Test with logger in context
	logger := NewLogger(slog.LevelDebug, "json")
	ctx := LoggerToContext(context.Background(), logger)

	retrievedLogger := LoggerFromContext(ctx)
	if retrievedLogger != logger {
		t.Error("LoggerFromContext() did not return the logger that was stored in context")
	}

	// Test with no logger in context
	emptyCtx := context.Background()
	defaultLogger := LoggerFromContext(emptyCtx)
	if defaultLogger == nil {
		t.Error("LoggerFromContext() returned nil for context without logger")
	}
}

func TestLoggerToContext(t *testing.T) {
	logger := NewLogger(slog.LevelDebug, "json")
	ctx := LoggerToContext(context.Background(), logger)

	// Verify logger was stored in context
	retrievedLogger := LoggerFromContext(ctx)
	if retrievedLogger != logger {
		t.Error("LoggerToContext() did not properly store logger in context")
	}
}

func TestFieldConstants(t *testing.T) {
	// Test that field constants are defined and non-empty
	fields := map[string]string{
		"FieldTool":             FieldTool,
		"FieldOperation":        FieldOperation,
		"FieldDuration":         FieldDuration,
		"FieldClusterName":      FieldClusterName,
		"FieldClusterNamespace": FieldClusterNamespace,
		"FieldRequestID":        FieldRequestID,
		"FieldUserAgent":        FieldUserAgent,
		"FieldRemoteAddr":       FieldRemoteAddr,
		"FieldStatusCode":       FieldStatusCode,
		"FieldContentLength":    FieldContentLength,
	}

	for name, value := range fields {
		t.Run(name, func(t *testing.T) {
			if value == "" {
				t.Errorf("Field constant %s is empty", name)
			}
		})
	}
}
