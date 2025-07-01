# CAPI MCP Server

A production-grade Model Context Protocol (MCP) server for the Kubernetes Cluster API (CAPI), enabling AI agents to manage Kubernetes clusters through a secure, standardized interface.

## Overview

The CAPI MCP Server acts as a bridge between AI agents (MCP clients) and CAPI management clusters. It provides a set of tools that abstract the complexity of CAPI into simple, reliable operations for cluster lifecycle management.

## Features

### V1.0 Scope
- **Infrastructure Provider**: AWS (via Cluster API Provider for AWS - CAPA)
- **Core Tools**:
  - `list_clusters` - List all managed workload clusters
  - `get_cluster` - Get detailed information for a specific cluster
  - `create_cluster` - Create a new workload cluster from templates
  - `delete_cluster` - Delete a workload cluster
  - `scale_cluster` - Scale worker nodes in a cluster
  - `get_cluster_kubeconfig` - Retrieve cluster access credentials
  - `get_cluster_nodes` - List nodes within a cluster
- **Security**: API key authentication, RBAC, secrets management
- **Observability**: Structured logging, Prometheus metrics

## Architecture

The server follows a modular, extensible design:
- **Proxy/Gateway Pattern** for bridging MCP and CAPI
- **Provider Interface** for future multi-cloud support
- **Asynchronous handling** of long-running CAPI operations
- **Security-first** approach with least-privilege access

See [architecture.md](architecture.md) for detailed design documentation.

## Development

### Prerequisites
- Go 1.23.x
- Docker & Docker Compose
- kubectl
- kind (for local testing)
- golangci-lint

### Quick Start

```bash
# Clone the repository
git clone https://github.com/capi-mcp/capi-mcp-server.git
cd capi-mcp-server

# Install dependencies
make deps

# Install development tools
make tools

# Run tests
make test

# Build the server
make build

# Run locally (requires API_KEY env var)
API_KEY=your-key make run
```

### Project Structure

```
/capi-mcp-server
├── /api/v1           # MCP tool/resource schemas
├── /cmd/server       # Application entry point
├── /internal         # Private application code
│   ├── /server       # MCP server engine
│   ├── /service      # Business logic
│   ├── /kube         # CAPI client wrapper
│   └── /config       # Configuration
├── /pkg              # Public libraries
│   ├── /provider     # Provider interface
│   └── /tools        # Tool implementations
├── /deploy           # Deployment artifacts
├── /test             # Test suites
└── /docs             # Documentation
```

## Deployment

The server is deployed as a Kubernetes workload using Helm:

```bash
helm install capi-mcp-server ./deploy/charts/capi-mcp-server \
  --set auth.apiKey=$API_KEY \
  --namespace capi-system
```

## Security

- **Authentication**: API key-based (Bearer token)
- **Authorization**: Kubernetes RBAC with least-privilege
- **Network**: Restricted with NetworkPolicies
- **Secrets**: Never logged, handled securely

## Contributing

Please read [CLAUDE.md](CLAUDE.md) for development guidelines and standards.

## Roadmap

See [roadmap.md](roadmap.md) for the project vision and development phases.

## AI Development Disclaimer

⚠️ **Important**: This repository was primarily created by Claude, Anthropic's AI assistant, working in collaboration with a human developer. The code, documentation, tests, and overall architecture were generated through AI-assisted development sessions.

While the code follows industry best practices and includes comprehensive testing, users should:
- Review all code before deploying to production environments
- Understand the security implications of the implementation
- Validate the code meets their specific requirements and compliance standards
- Consider the experimental nature of AI-generated code in critical systems

## License

[License details to be added]