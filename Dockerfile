# Build stage
FROM golang:1.23-alpine AS builder

# Install dependencies
RUN apk add --no-cache git make ca-certificates

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 make build

# Final stage
FROM gcr.io/distroless/static-debian12:nonroot

# Copy CA certificates
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Copy the binary
COPY --from=builder /app/build/capi-mcp-server /capi-mcp-server

# Use non-root user
USER nonroot:nonroot

# Expose the server port (will be configurable)
EXPOSE 8080

# Run the server
ENTRYPOINT ["/capi-mcp-server"]