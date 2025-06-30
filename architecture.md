
## 1.0 System Overview & Architectural Patterns

The Cluster API (CAPI) MCP Server is a standalone service designed to act as a secure bridge between AI agents (MCP clients) and a CAPI management cluster. It translates high-level, tool-based requests from an agent into declarative, object-based operations on the Kubernetes API of the management cluster.

### 1.1 Architectural Pattern

The server's design is fundamentally based on a **Proxy/Gateway Pattern**. It exposes a standardized MCP interface to the outside world while hiding the complexity of the underlying CAPI and Kubernetes APIs.[12] It does not maintain its own state regarding clusters; instead, it serves as a stateless proxy whose source of truth is always the CAPI management cluster itself.

Conceptually, this system operates within a **Hierarchical Agent Pattern**.[13] The external AI agent acts as the "planner," responsible for decomposing a user's request into a sequence of steps. The CAPI MCP Server acts as the "executor" or "tool-user," providing a constrained set of powerful, reliable tools that the planner can invoke to accomplish its goals.

The data flow is as follows:
1.  An MCP Client (e.g., an AI agent in an IDE) sends a JSON-RPC request to the CAPI MCP Server to execute a tool (e.g., `scale_cluster`).
2.  The server's security middleware authenticates and authorizes the request.
3.  The server's Tool Provider routes the request to the appropriate business logic in the CAPI Service Layer.
4.  The CAPI Service Layer translates the tool's parameters into a CAPI object manipulation (e.g., fetching a `MachineDeployment` object and updating its `spec.replicas` field).
5.  This manipulation is performed via the CAPI Client Wrapper, which communicates with the Kubernetes API server of the management cluster.
6.  For long-running operations, the server monitors the status of the relevant CAPI resources and returns a final success or failure state to the agent.

### 1.2 Core Technology Stack

*   **Language**: Go (Golang), chosen for its high performance, robust concurrency model, static typing, and its position as the de facto language of the cloud-native ecosystem, including Kubernetes and Cluster API itself.
*   **MCP Framework**: The official `modelcontextprotocol/go-sdk` will be used to handle all protocol-level concerns, ensuring full compliance with the MCP specification.[11]
*   **Kubernetes Interaction**: The server will use the standard Kubernetes `k8s.io/client-go` and `sigs.k8s.io/controller-runtime/pkg/client` libraries. These provide type-safe, robust, and well-tested mechanisms for interacting with the Kubernetes API.

## 2.0 Core Component Design

The server is composed of several distinct, loosely coupled components, each with a specific responsibility. This modular design promotes maintainability, testability, and future extensibility.

*   **MCP Server Engine**: The main application entry point. Built using the `go-sdk`, this component is responsible for handling the underlying transport (e.g., HTTP with JSON-RPC), parsing incoming messages, and routing requests to the Tool Provider.
*   **Tool Provider**: The central hub for all exposed capabilities. It programmatically registers each of the seven tools, defining their names, detailed descriptions, and input schemas. It maps incoming tool calls from the Server Engine to the corresponding implementation in the CAPI Service Layer.
*   **CAPI Service Layer**: This is the core business logic layer. It contains functions like `CreateCluster`, `ListClusters`, and `ScaleMachineDeployment`. This layer is completely decoupled from the MCP protocol itself. Its responsibility is to orchestrate the steps needed to fulfill a request, such as validating inputs, interacting with the CAPI Client Wrapper, and handling the asynchronous nature of CAPI operations.
*   **Provider Interface**: A critical component for extensibility, this is a Go interface (`type Provider interface {...}`) that defines a contract for any infrastructure-specific logic. For example, it might include methods like `GetInfrastructureTemplateSpec()` or `ValidateProviderConfig()`. This allows new providers (e.g., for Azure or GCP) to be added by simply creating a new struct that satisfies this interface.
*   **AWS Provider Implementation**: The V1 implementation of the `Provider` interface. It contains logic specific to the Cluster API Provider for AWS (CAPA), such as details about `AWSCluster` and `AWSMachineTemplate` objects.
*   **CAPI Client Wrapper**: A dedicated internal package (`internal/kube`) that abstracts all direct interactions with the Kubernetes API. It provides high-level, easy-to-use functions like `GetCAPIClusterByName(ctx, name)` or `UpdateMachineDeployment(ctx, md)`. This isolates the rest of the application from the complexities of `client-go` and makes testing easier by allowing the client to be mocked.
*   **Configuration Manager**: A component responsible for loading, validating, and providing access to server configuration. It will follow the 12-factor app methodology, prioritizing environment variables for configuration, which can be populated from Kubernetes ConfigMaps or Secrets.[4, 14]
*   **Security Module**: A set of middleware functions that plug into the MCP Server Engine. These are responsible for enforcing the security policies defined in the Security Architecture section, primarily authentication and authorization on every incoming request.

### 2.1 Bridging the Asynchronous Gap

A primary architectural challenge is the impedance mismatch between the synchronous, function-call nature of MCP tools and the asynchronous, declarative nature of Cluster API. An AI agent calling `create_cluster` expects a definitive result to continue its workflow, but CAPI operations can take several minutes to complete.[5, 13] Simply creating the CAPI resources and returning `202 Accepted` is insufficient, as it leaves the agent blind to the operation's progress or potential failure.

To solve this, long-running mutating operations (`create_cluster`, `delete_cluster`, `scale_cluster`) will not return immediately. Instead, their implementation within the CAPI Service Layer will:
1.  Initiate the declarative operation by creating or modifying the necessary CAPI resources on the management cluster.
2.  Immediately begin watching the `status` fields of the relevant resources (e.g., the `Cluster` object's `status.phase` and `status.conditions`, or the `MachineDeployment`'s `status.updatedReplicas`).[10]
3.  The tool will block and wait for the resource to reach a terminal state (e.g., `Provisioned`, `Failed`) or until a configurable timeout is reached.
4.  The final status, including any error messages from the CAPI controllers, will be returned to the agent, providing a clear, actionable result. This makes the tools far more reliable for use in multi-step agentic workflows.

## 3.0 Source Code Organization

A standardized directory structure will be enforced to ensure a clean separation of concerns and facilitate navigation for developers.
/capi-mcp-server
├── /api
│   └── /v1           # Go structs defining MCP tool and resource JSON schemas
├── /cmd
│   └── /server       # main.go - application entry point and initialization
├── /internal
│   ├── /server       # Core MCP server engine, routing, and transport middleware
│   ├── /service      # The CAPI service layer (business logic)
│   ├── /kube         # CAPI Client Wrapper for all k8s API interactions
│   └── /config       # Configuration loading and validation
├── /pkg
│   ├── /provider     # The provider interface definition
│   │   └── /aws      # The AWS provider implementation
│   └── /tools        # The implementation of each of the 7 MCP tools
├── /deploy
│   ├── /charts       # Helm chart for deploying the server and its resources
│   └── /manifests    # Raw Kubernetes manifests for development
├── /test
│   ├── /integration  # Integration tests
│   └── /e2e          # End-to-end tests
├── go.mod
└── Dockerfile

