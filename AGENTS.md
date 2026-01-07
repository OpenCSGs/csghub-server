# AGENTS Guidelines for This Repository

This repository is a Go project following the **microservice** architecture design. Services including:

- **API**: The API service handles HTTP requests and responses. It is the entry point for external clients to interact with the system.
- **User**: The User service handles user-related operations, such as user registration, login, and profile management. All requests are proxied to this service from the API service.
- **Accounting**: The Accounting service handles accounting-related operations, such as recording user token or hardware resource usage, or updating user balances. All requests are proxied to this service from the API service.
- **Moderation**: The Moderation service handles content moderation operations, such as flagging inappropriate text or images. All requests are proxied to this service from the API service.
- **DataViewer**: The DataViewer service handles dataset preview operations, such as fetching dataset metadata or previewing dataset files. All requests are proxied to this service from the API service.
- **Notification**: The Notification service handles sending notifications to users, such as email or push notifications. All requests are proxied to this service from the API service.
- **Payment**: The Payment service handles payment operations, such as processing payments or refunding payments. All requests are proxied to this service from the API service.
- **AIGateway**: The AIGateway service handles AI model inference operations, such as running AI models or generating AI outputs. It's another entry point for external clients to interact with the AI models. 
- **Runner**: The Runner service is a bridge between api service and Kubernetes cluster. It handles deployment of models, spaces.
- **LogCollector**: The LogCollector service handles collecting logs from Kubernetes cluster. All logs are sent to this service from the Runner service and API service.

# Structure
Every service follows the layered architecture design: handler -> component -> builder (database, rpc, git, etc.).

- The handler layer handles HTTP requests and responses.
- The component layer handles business logic and coordinates between different layers.
- The builder layer handles low-level operations, such as database access, RPC calls, or Git operations.
- Every golang file should have a corresponding `*_test.go` file for unit tests.

Folders relative to the root of the repository for each service:

| Service | Folder |
|---------|--------|
| API     | api    |
| User    | user   |
| Accounting | accounting |
| Moderation | moderation |
| DataViewer | dataviewer |
| Notification | notification |
| Payment | payment |
| AIGateway | aigateway |
| Runner | runner |
| LogCollector | logcollector |

## Function Examples

### Router

- `api/router/api.go` is an example of a router that registers HTTP routes and their corresponding handlers for common functionality across services.
- `accounting/router/api.go` is an example of a router that registers HTTP routes and their corresponding handlers for the Accounting service.
- `runner/router/api.go` is an example of a router that registers HTTP routes and their corresponding handlers for the runner service.

### Handler Layer

- `api/handler/space.go` is an example of a handler that deals with space-related HTTP requests.
- `api/handler/evaluation.go` is an example of a handler that deals with evaluation-related HTTP requests.

### Component Layer

- `component/space.go` is an example of a component that deals with space-related business logic.
- `component/evaluation.go` is an example of a component that deals with evaluation-related business logic.

### Database Builder Layer

- `builder/store/database/space.go` is an example of a builder that deals with space-related database operations.

### Database Migration

- `builder/store/database/migrations/20240201061926_create_spaces.go` is an example of a database migration script that creates a space table.
- use `go run cmd/csghub-server/main.go migration create_go` to generate a go database migration script.
- use `go run cmd/csghub-server/main.go migration create_sql` to generate a sql database migration script. 

### Space Deploy

- `builder/deploy/deployer.go` create build and deploy task in database, then create temporal workflow to run the task.
- `api/workflow/activity/deploy_activity.go` impletements temporal activities to run the build and deploy task by call runner api.
- `runner/handler/imagebuilder.go` implements runner api to trigger image builder process by call image builder component.
- `runner/component/imagebuilder.go` implements runner component to trigger deploy process by call knative api.
- `runner/handler/service.go` implements runner api to trigger deploy process by call deploy component.
- `runner/component/service.go` implements runner component to trigger deploy process by call knative api.
- `docker/spaces/builder/Dockerfile*` are Dockerfile that builds the space image.

### Cluster

- `component/cluster.go` is a component that deals with cluster-related business logic.

## Code Style & Conventions:

- Each layer's interface should only expose data structures defined within its own layer or common type definitions from the common.types package. For example, interfaces in the Component layer (such as UserComponent) should not return data structures from the underlying database layer (such as database.User structure), as the database layer is considered lower-level than the component layer.
- Write unit tests for new code.
- Use struct data types instead of primitive types for function parameters and return values.
- All variables should be named in camelCase.
- Variables should be declared at the smallest possible scope under `common/types`.

### Do

### Do Not

## Testing

- Use `make mock_gen GO_TAGS={go.buildTags}` to generate mock implementations for the interfaces.
- Use `make test GO_TAGS={go.buildTags}` to run all tests in project.
- Mock dependencies (e.g., database, RPC clients) using tools like `mockery`.

## Tools

- Search `Makefile` for running, building, testing, and linting tools.
- Swagger doc is generated by `swag` tool, and it will be served by handler layer. 

## Commit & Pull Request Guidelines:

- Each PR must include a clear description of the changes made and their impact, including root cause analysis if applicable, and solution details, and local test result.

## Specific Instructions
