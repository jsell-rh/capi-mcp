package errors

import (
	"errors"
	"fmt"
	"strings"
)

// Common error types for the CAPI MCP Server
var (
	// ErrNotFound indicates a requested resource was not found
	ErrNotFound = errors.New("resource not found")
	
	// ErrAlreadyExists indicates a resource already exists
	ErrAlreadyExists = errors.New("resource already exists")
	
	// ErrInvalidInput indicates invalid input parameters
	ErrInvalidInput = errors.New("invalid input")
	
	// ErrUnauthorized indicates authentication/authorization failure
	ErrUnauthorized = errors.New("unauthorized")
	
	// ErrForbidden indicates the operation is not allowed
	ErrForbidden = errors.New("forbidden")
	
	// ErrTimeout indicates an operation timed out
	ErrTimeout = errors.New("operation timed out")
	
	// ErrInternal indicates an internal server error
	ErrInternal = errors.New("internal server error")
	
	// ErrProviderNotFound indicates the specified provider doesn't exist
	ErrProviderNotFound = errors.New("provider not found")
	
	// ErrProviderValidation indicates provider validation failed
	ErrProviderValidation = errors.New("provider validation failed")
	
	// ErrClusterNotReady indicates cluster is not in ready state
	ErrClusterNotReady = errors.New("cluster not ready")
	
	// ErrClusterFailed indicates cluster is in failed state
	ErrClusterFailed = errors.New("cluster failed")
	
	// ErrOperationInProgress indicates another operation is in progress
	ErrOperationInProgress = errors.New("operation in progress")
)

// ErrorCode represents standardized error codes for the MCP server
type ErrorCode string

const (
	// Client errors (4xx equivalent)
	CodeInvalidInput       ErrorCode = "INVALID_INPUT"
	CodeNotFound           ErrorCode = "NOT_FOUND"
	CodeAlreadyExists      ErrorCode = "ALREADY_EXISTS"
	CodeUnauthorized       ErrorCode = "UNAUTHORIZED"
	CodeForbidden          ErrorCode = "FORBIDDEN"
	CodeValidationFailed   ErrorCode = "VALIDATION_FAILED"
	CodePreconditionFailed ErrorCode = "PRECONDITION_FAILED"
	
	// Server errors (5xx equivalent)
	CodeInternal           ErrorCode = "INTERNAL_ERROR"
	CodeTimeout            ErrorCode = "TIMEOUT"
	CodeProviderError      ErrorCode = "PROVIDER_ERROR"
	CodeKubernetesAPI      ErrorCode = "KUBERNETES_API_ERROR"
	CodeResourceExhausted  ErrorCode = "RESOURCE_EXHAUSTED"
	CodeUnavailable        ErrorCode = "SERVICE_UNAVAILABLE"
	CodeProviderValidation ErrorCode = "PROVIDER_VALIDATION"
	CodeDependencyFailure  ErrorCode = "DEPENDENCY_FAILURE"
	CodeWorkloadCluster    ErrorCode = "WORKLOAD_CLUSTER"
)

// Error represents a structured error with code and context
type Error struct {
	Code    ErrorCode
	Message string
	Details map[string]interface{}
	Cause   error
}

// Error implements the error interface
func (e *Error) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s (caused by: %v)", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Unwrap returns the underlying cause of the error
func (e *Error) Unwrap() error {
	return e.Cause
}

// Is checks if the error matches the target error
func (e *Error) Is(target error) bool {
	if target == nil {
		return false
	}
	
	// Check if target is an Error with same code
	if targetErr, ok := target.(*Error); ok {
		return e.Code == targetErr.Code
	}
	
	// Check if cause matches
	return errors.Is(e.Cause, target)
}

// New creates a new Error with the given code and message
func New(code ErrorCode, message string) *Error {
	return &Error{
		Code:    code,
		Message: message,
		Details: make(map[string]interface{}),
	}
}

// Wrap wraps an existing error with additional context
func Wrap(err error, code ErrorCode, message string) *Error {
	if err == nil {
		return nil
	}
	
	// If err is already an Error, preserve its details
	if e, ok := err.(*Error); ok {
		return &Error{
			Code:    code,
			Message: message,
			Details: e.Details,
			Cause:   err,
		}
	}
	
	return &Error{
		Code:    code,
		Message: message,
		Details: make(map[string]interface{}),
		Cause:   err,
	}
}

// WithDetails adds additional details to the error
func (e *Error) WithDetails(key string, value interface{}) *Error {
	if e.Details == nil {
		e.Details = make(map[string]interface{})
	}
	e.Details[key] = value
	return e
}

// WithDetailsMap adds multiple details to the error
func (e *Error) WithDetailsMap(details map[string]interface{}) *Error {
	if e.Details == nil {
		e.Details = make(map[string]interface{})
	}
	for k, v := range details {
		e.Details[k] = v
	}
	return e
}

// IsNotFound checks if an error indicates a resource was not found
func IsNotFound(err error) bool {
	if err == nil {
		return false
	}
	
	// Check standard errors
	if errors.Is(err, ErrNotFound) {
		return true
	}
	
	// Check Error type
	var e *Error
	if errors.As(err, &e) && e.Code == CodeNotFound {
		return true
	}
	
	// Check error message for common patterns
	errStr := strings.ToLower(err.Error())
	return strings.Contains(errStr, "not found") || 
		strings.Contains(errStr, "does not exist")
}

// IsAlreadyExists checks if an error indicates a resource already exists
func IsAlreadyExists(err error) bool {
	if err == nil {
		return false
	}
	
	// Check standard errors
	if errors.Is(err, ErrAlreadyExists) {
		return true
	}
	
	// Check Error type
	var e *Error
	if errors.As(err, &e) && e.Code == CodeAlreadyExists {
		return true
	}
	
	// Check error message for common patterns
	errStr := strings.ToLower(err.Error())
	return strings.Contains(errStr, "already exists") || 
		strings.Contains(errStr, "duplicate")
}

// IsTimeout checks if an error indicates a timeout
func IsTimeout(err error) bool {
	if err == nil {
		return false
	}
	
	// Check standard errors
	if errors.Is(err, ErrTimeout) {
		return true
	}
	
	// Check Error type
	var e *Error
	if errors.As(err, &e) && e.Code == CodeTimeout {
		return true
	}
	
	// Check error message for common patterns
	errStr := strings.ToLower(err.Error())
	return strings.Contains(errStr, "timeout") || 
		strings.Contains(errStr, "deadline exceeded")
}

// IsUnauthorized checks if an error indicates authorization failure
func IsUnauthorized(err error) bool {
	if err == nil {
		return false
	}
	
	// Check standard errors
	if errors.Is(err, ErrUnauthorized) {
		return true
	}
	
	// Check Error type
	var e *Error
	if errors.As(err, &e) && e.Code == CodeUnauthorized {
		return true
	}
	
	// Check error message for common patterns
	errStr := strings.ToLower(err.Error())
	return strings.Contains(errStr, "unauthorized") || 
		strings.Contains(errStr, "authentication failed")
}

// GetErrorCode returns the error code for an error
func GetErrorCode(err error) ErrorCode {
	if err == nil {
		return ""
	}
	
	var e *Error
	if errors.As(err, &e) {
		return e.Code
	}
	
	// Map standard errors to codes
	switch {
	case errors.Is(err, ErrNotFound):
		return CodeNotFound
	case errors.Is(err, ErrAlreadyExists):
		return CodeAlreadyExists
	case errors.Is(err, ErrInvalidInput):
		return CodeInvalidInput
	case errors.Is(err, ErrUnauthorized):
		return CodeUnauthorized
	case errors.Is(err, ErrForbidden):
		return CodeForbidden
	case errors.Is(err, ErrTimeout):
		return CodeTimeout
	case errors.Is(err, ErrInternal):
		return CodeInternal
	default:
		return CodeInternal
	}
}

// GetUserMessage returns a user-friendly error message that doesn't expose internal details
func GetUserMessage(err error) string {
	if err == nil {
		return ""
	}
	
	var e *Error
	if errors.As(err, &e) {
		// Return the high-level message without internal details
		return e.Message
	}
	
	// For standard errors, return generic messages
	switch {
	case IsNotFound(err):
		return "The requested resource was not found"
	case IsAlreadyExists(err):
		return "A resource with that name already exists"
	case IsTimeout(err):
		return "The operation timed out"
	case IsUnauthorized(err):
		return "Authentication failed"
	default:
		return "An internal error occurred"
	}
}

// SanitizeErrorMessage removes sensitive information from error messages
func SanitizeErrorMessage(message string) string {
	// Replace common sensitive patterns with redacted placeholders
	patterns := []struct {
		pattern     string
		replacement string
	}{
		{"secret [a-zA-Z0-9_-]+", "secret [REDACTED]"},
		{"token [a-zA-Z0-9_.-]+", "token [REDACTED]"},
		{"password [a-zA-Z0-9_!@#$%^&*()-=+]+", "password [REDACTED]"},
		{"key [a-zA-Z0-9_.-]+", "key [REDACTED]"},
		{"Bearer [a-zA-Z0-9_.-]+", "[REDACTED]"},
		{"AKIA[a-zA-Z0-9]+", "[REDACTED]"},
		{"aws_[a-zA-Z0-9_]+", "[REDACTED]"},
	}
	
	result := message
	for _, pattern := range patterns {
		// Simple string replacement for common patterns
		// In production, use proper regex replacement
		if strings.Contains(strings.ToLower(result), strings.Split(pattern.pattern, " ")[0]) {
			parts := strings.Fields(result)
			for i, part := range parts {
				if strings.Contains(strings.ToLower(part), strings.Split(pattern.pattern, " ")[0]) {
					// Check if the next word looks like a sensitive value
					if i+1 < len(parts) && len(parts[i+1]) > 6 {
						parts[i+1] = "[REDACTED]"
					}
				}
			}
			result = strings.Join(parts, " ")
		}
	}
	
	return result
}