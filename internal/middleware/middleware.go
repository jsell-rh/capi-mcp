package middleware

import (
	"context"
	"fmt"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/google/uuid"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	
	"github.com/capi-mcp/capi-mcp-server/internal/errors"
	"github.com/capi-mcp/capi-mcp-server/internal/logging"
)

// RequestLogger is a middleware that logs all incoming requests
func RequestLogger(logger *logging.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Generate request ID
			requestID := r.Header.Get("X-Request-ID")
			if requestID == "" {
				requestID = uuid.New().String()
			}
			
			// Add request ID to context
			ctx := logging.ContextWithRequestID(r.Context(), requestID)
			
			// Add logger to context
			reqLogger := logger.WithContext(ctx).With(
				"method", r.Method,
				"path", r.URL.Path,
				"remote_addr", r.RemoteAddr,
			)
			ctx = logging.ContextWithLogger(ctx, reqLogger)
			
			// Create wrapped response writer to capture status code
			wrapped := &responseWriter{
				ResponseWriter: w,
				statusCode:     http.StatusOK,
			}
			
			// Log request start
			startTime := time.Now()
			reqLogger.Info("Request started")
			
			// Set request ID header
			wrapped.Header().Set("X-Request-ID", requestID)
			
			// Process request
			next.ServeHTTP(wrapped, r.WithContext(ctx))
			
			// Log request completion
			duration := time.Since(startTime)
			reqLogger.Info("Request completed",
				"status", wrapped.statusCode,
				"duration_ms", duration.Milliseconds(),
				"bytes_written", wrapped.bytesWritten,
			)
		})
	}
}

// ErrorHandler is a middleware that handles panics and errors
func ErrorHandler(logger *logging.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if err := recover(); err != nil {
					// Log the panic
					reqLogger := logging.LoggerFromContext(r.Context())
					reqLogger.Error("Panic recovered",
						"panic", fmt.Sprintf("%v", err),
						"stack_trace", string(debug.Stack()),
					)
					
					// Return internal server error
					http.Error(w, "Internal server error", http.StatusInternalServerError)
				}
			}()
			
			next.ServeHTTP(w, r)
		})
	}
}

// MCPErrorHandler wraps MCP handlers with error handling
func MCPErrorHandler(logger *logging.Logger, handler interface{}) interface{} {
	switch h := handler.(type) {
	case mcp.ToolFunc:
		return func(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
			// Add logging context
			toolLogger := logging.LoggerFromContext(ctx)
			
			// Execute with panic recovery
			defer func() {
				if err := recover(); err != nil {
					toolLogger.Error("Tool panic recovered",
						"panic", fmt.Sprintf("%v", err),
						"stack_trace", string(debug.Stack()),
					)
				}
			}()
			
			// Call the actual handler
			result, err := h(ctx, input)
			
			// Handle errors appropriately
			if err != nil {
				// Log the error with appropriate level
				switch errors.GetErrorCode(err) {
				case errors.CodeNotFound, errors.CodeInvalidInput:
					toolLogger.Warn("Tool error", "error", err)
				default:
					toolLogger.Error("Tool error", "error", err)
				}
				
				// Return sanitized error for client
				return nil, sanitizeError(err)
			}
			
			return result, nil
		}
		
	case mcp.ResourceFunc:
		return func(ctx context.Context, uri string) (string, string, error) {
			// Add logging context
			resourceLogger := logging.LoggerFromContext(ctx)
			
			// Execute with panic recovery
			defer func() {
				if err := recover(); err != nil {
					resourceLogger.Error("Resource panic recovered",
						"panic", fmt.Sprintf("%v", err),
						"stack_trace", string(debug.Stack()),
					)
				}
			}()
			
			// Call the actual handler
			content, mimeType, err := h(ctx, uri)
			
			// Handle errors appropriately
			if err != nil {
				// Log the error
				resourceLogger.Error("Resource error", "error", err)
				
				// Return sanitized error for client
				return "", "", sanitizeError(err)
			}
			
			return content, mimeType, nil
		}
		
	default:
		// Return the handler unchanged if we don't know how to wrap it
		return handler
	}
}

// sanitizeError converts internal errors to safe client errors
func sanitizeError(err error) error {
	if err == nil {
		return nil
	}
	
	// Get the user-friendly message
	userMessage := errors.GetUserMessage(err)
	code := errors.GetErrorCode(err)
	
	// Create a new error with sanitized information
	return errors.New(code, userMessage)
}

// responseWriter wraps http.ResponseWriter to capture response details
type responseWriter struct {
	http.ResponseWriter
	statusCode    int
	bytesWritten  int64
	headerWritten bool
}

func (rw *responseWriter) WriteHeader(statusCode int) {
	if !rw.headerWritten {
		rw.statusCode = statusCode
		rw.headerWritten = true
		rw.ResponseWriter.WriteHeader(statusCode)
	}
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	if !rw.headerWritten {
		rw.WriteHeader(http.StatusOK)
	}
	n, err := rw.ResponseWriter.Write(b)
	rw.bytesWritten += int64(n)
	return n, err
}

// RequestTimeout adds a timeout to requests
func RequestTimeout(timeout time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithTimeout(r.Context(), timeout)
			defer cancel()
			
			// Channel to signal when request is done
			done := make(chan struct{})
			
			go func() {
				next.ServeHTTP(w, r.WithContext(ctx))
				close(done)
			}()
			
			select {
			case <-done:
				// Request completed normally
			case <-ctx.Done():
				// Timeout occurred
				reqLogger := logging.LoggerFromContext(r.Context())
				reqLogger.Error("Request timeout",
					"timeout", timeout,
					"path", r.URL.Path,
				)
				http.Error(w, "Request timeout", http.StatusRequestTimeout)
			}
		})
	}
}

// CORS adds CORS headers for browser-based clients
func CORS(allowedOrigins []string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			
			// Check if origin is allowed
			allowed := false
			for _, allowedOrigin := range allowedOrigins {
				if allowedOrigin == "*" || allowedOrigin == origin {
					allowed = true
					break
				}
			}
			
			if allowed {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Request-ID")
				w.Header().Set("Access-Control-Max-Age", "86400")
			}
			
			// Handle preflight requests
			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			
			next.ServeHTTP(w, r)
		})
	}
}