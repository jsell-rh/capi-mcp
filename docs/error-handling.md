# Error Handling & Logging

This document describes the comprehensive error handling and logging system implemented for the CAPI MCP Server.

## Error Codes

The server uses structured error codes to provide consistent, actionable feedback to clients. All errors include a standardized code, user-friendly message, and optional context details.

### Client Error Codes (4xx equivalent)

| Code | Description | Example Use Case |
|------|-------------|------------------|
| `INVALID_INPUT` | Invalid input parameters provided | Missing required fields, invalid formats |
| `NOT_FOUND` | Requested resource does not exist | Cluster, template, or node pool not found |
| `ALREADY_EXISTS` | Resource with same identifier exists | Creating cluster with existing name |
| `UNAUTHORIZED` | Authentication failed or missing | Invalid API key, expired token |
| `FORBIDDEN` | Operation not allowed for user | Insufficient permissions |
| `VALIDATION_FAILED` | Input validation failed | Invalid cluster name format, unsupported version |
| `PRECONDITION_FAILED` | Required conditions not met | Cluster not in correct state for operation |

### Server Error Codes (5xx equivalent)

| Code | Description | Example Use Case |
|------|-------------|------------------|
| `INTERNAL_ERROR` | Unexpected internal server error | Programming errors, system failures |
| `TIMEOUT` | Operation timed out | Kubernetes API calls, cluster operations |
| `PROVIDER_ERROR` | Cloud provider API error | AWS API failures, quota exceeded |
| `KUBERNETES_API_ERROR` | Kubernetes API operation failed | CAPI resource creation/update failures |
| `RESOURCE_EXHAUSTED` | System resources unavailable | Memory, CPU, or storage limits |
| `SERVICE_UNAVAILABLE` | Service temporarily unavailable | Maintenance mode, overload |
| `PROVIDER_VALIDATION` | Provider-specific validation failed | Invalid AWS region, instance type |
| `DEPENDENCY_FAILURE` | External dependency failed | Database connection, cache service |
| `WORKLOAD_CLUSTER` | Workload cluster operation failed | Node listing, kubectl operations |

## Error Structure

All errors follow a consistent structure:

```go
type Error struct {
    Code    ErrorCode                  // Standardized error code
    Message string                     // User-friendly message
    Details map[string]interface{}     // Optional context details
    Cause   error                      // Underlying error (if any)
}
```

### Example Error Response

```json
{
  "error": {
    "code": "INVALID_INPUT",
    "message": "cluster name must be a valid DNS subdomain",
    "details": {
      "field": "cluster_name",
      "provided_value": "My-Cluster!"
    }
  }
}
```

## Error Safety & Security

### Sensitive Data Protection

The error handling system automatically sanitizes error messages to prevent sensitive information leakage:

- **API Keys**: `AKIA123456789` → `[REDACTED]`
- **Tokens**: `Bearer eyJhbGc...` → `Bearer [REDACTED]`
- **Passwords**: `password secret123` → `password [REDACTED]`
- **Secrets**: `secret abc123def` → `secret [REDACTED]`

### User-Friendly Messages

Internal errors are converted to generic, user-friendly messages:

```go
// Internal error with stack trace
err := fmt.Errorf("database connection failed: %w", sqlErr)

// User sees:
{
  "code": "SERVICE_UNAVAILABLE", 
  "message": "The service is temporarily unavailable"
}
```

## Logging System

### Structured Logging

All logging uses structured JSON format with consistent field names:

```json
{
  "timestamp": "2024-01-15T10:30:00Z",
  "level": "info",
  "component": "cluster-service",
  "operation": "CreateCluster",
  "cluster_name": "my-cluster",
  "duration_ms": 1234,
  "message": "Cluster created successfully"
}
```

### Standard Log Fields

| Field | Purpose | Example |
|-------|---------|---------|
| `component` | Service/module name | `cluster-service`, `tools`, `auth` |
| `operation` | Current operation | `CreateCluster`, `ListNodes` |
| `cluster_name` | Target cluster | `production-cluster` |
| `cluster_namespace` | Cluster namespace | `default`, `team-a` |
| `tool` | MCP tool name | `create_cluster`, `scale_cluster` |
| `duration_ms` | Operation duration | `1234` |
| `error` | Error message | `cluster not found` |
| `request_id` | Request identifier | `req_abc123` |
| `user_id` | User identifier | `user_456` |

### Log Levels

- **DEBUG**: Detailed tracing, sensitive operations (disabled in production)
- **INFO**: Normal operations, cluster lifecycle events
- **WARN**: Recoverable errors, degraded performance
- **ERROR**: Failed operations, system errors

### Context-Aware Logging

Loggers automatically include context from HTTP requests:

```go
// Request context is propagated through all operations
logger := logging.LoggerFromContext(ctx)
logger.WithCluster("my-cluster", "default").Info("Starting operation")
```

### Sensitive Data Masking

Log values are automatically masked:

```go
logger.Info("API key provided", 
    "key", logging.MaskSensitive(apiKey, 4)) // Shows: "AKIA***"
```

## Operation Tracking

### Tool Call Logging

All MCP tool invocations are automatically logged:

```json
{
  "level": "info",
  "tool": "create_cluster",
  "tool_input": {"cluster_name": "test", "replicas": 3},
  "duration_ms": 2500,
  "message": "Tool invocation completed"
}
```

### Operation Lifecycle

Long-running operations are tracked from start to completion:

```json
// Start
{"level": "info", "operation": "CreateCluster", "message": "Starting operation"}

// Progress
{"level": "info", "operation": "CreateCluster", "message": "Validating cluster configuration"}

// Completion
{"level": "info", "operation": "CreateCluster", "duration_ms": 30000, "message": "Operation completed"}
```

## Error Handling Best Practices

### For Developers

1. **Use Structured Errors**: Always create errors with appropriate codes
   ```go
   return errors.New(errors.CodeNotFound, "cluster not found")
   ```

2. **Wrap Errors**: Preserve context when wrapping errors
   ```go
   return errors.Wrap(err, errors.CodeKubernetesAPI, "failed to create cluster")
   ```

3. **Add Context**: Include relevant details for debugging
   ```go
   err := errors.New(errors.CodeInvalidInput, "invalid cluster name").
           WithDetails("field", "cluster_name").
           WithDetails("value", clusterName)
   ```

4. **Validate Early**: Use the validation package for input checking
   ```go
   if err := validator.ValidateClusterName(name); err != nil {
       return nil, err
   }
   ```

### For Operations

1. **Monitor Error Rates**: Track error codes in metrics/alerting
2. **Log Aggregation**: Use structured logs for searching/filtering  
3. **Alert on Patterns**: Set up alerts for repeated error codes
4. **Capacity Planning**: Monitor `RESOURCE_EXHAUSTED` errors

## Testing Error Handling

The error handling system includes comprehensive tests:

- **Unit Tests**: All error codes, wrapping, unwrapping
- **Integration Tests**: End-to-end error propagation
- **Security Tests**: Sensitive data sanitization
- **Logging Tests**: Structured output, context propagation

Example test:
```go
func TestErrorSanitization(t *testing.T) {
    err := errors.New(errors.CodeInternal, "secret abc123 leaked")
    sanitized := errors.SanitizeErrorMessage(err.Error())
    assert.Contains(t, sanitized, "[REDACTED]")
    assert.NotContains(t, sanitized, "abc123")
}
```

## Configuration

Error handling behavior can be configured:

```yaml
# config.yaml
logging:
  level: "info"           # debug, info, warn, error
  format: "json"          # json, text
  
error_handling:
  sanitize_errors: true   # Remove sensitive data
  include_stack_trace: false  # Include stack traces (debug only)
```

This comprehensive error handling and logging system ensures that the CAPI MCP Server provides excellent observability while maintaining security and user experience.