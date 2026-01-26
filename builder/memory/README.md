# Memory Adapter Architecture & Development Guide

This guide explains how CSGHub integrates memory backends and how to implement a new backend adapter.

## Architecture Overview

CSGHub uses an adapter pattern to isolate backend-specific logic from the API/handler layer.

```
csghub-server/component/memory.go
        |
        v
csghub-server/builder/memory/adapter.go   (Adapter interface)
        |
        v
csghub-server/builder/memory/registry.go  (adapter registry)
        |
        v
csghub-server/builder/memory/<backend>/   (backend implementation)
```

Key points:

- The **API layer** deals only with canonical request/response types in `common/types`.
- The **adapter** converts canonical requests/responses to backend-specific formats.
- The **registry** selects the adapter by name via `OPENCSG_MEMORY_BACKEND`.
- Each backend package registers itself with `init()` and a `RegisterAdapter` call.

## Adapter Interface

`csghub-server/builder/memory/adapter.go`

```go
type Adapter interface {
  CreateProject(ctx context.Context, req *types.CreateMemoryProjectRequest) (*types.MemoryProjectResponse, error)
  GetProject(ctx context.Context, req *types.GetMemoryProjectRequest) (*types.MemoryProjectResponse, error)
  ListProjects(ctx context.Context) ([]*types.MemoryProjectRef, error)
  DeleteProject(ctx context.Context, req *types.DeleteMemoryProjectRequest) error
  AddMemories(ctx context.Context, req *types.AddMemoriesRequest) (*types.AddMemoriesResponse, error)
  SearchMemories(ctx context.Context, req *types.SearchMemoriesRequest) (*types.SearchMemoriesResponse, error)
  ListMemories(ctx context.Context, req *types.ListMemoriesRequest) (*types.ListMemoriesResponse, error)
  DeleteMemories(ctx context.Context, req *types.DeleteMemoriesRequest) error
  Health(ctx context.Context) (*types.MemoryHealthResponse, error)
}
```

Optional interfaces (for adapter tuning):

```go
type TimeoutSetter interface { WithTimeout(time.Duration) }
type RetrySetter interface { WithRetry(uint) }
type DelaySetter interface { WithDelay(time.Duration) }
```

If your adapter implements these, the component will apply the configured timeout/retry/delay.

## Registering a New Backend

1) Create a new backend package:

```
csghub-server/builder/memory/<backend>/
```

2) Implement an adapter struct with the `Adapter` interface.

3) Register it in `init()`:

```go
func init() {
  memory.RegisterAdapter("your_backend", func(endpoint, basePath string, opts ...rpc.RequestOption) memory.Adapter {
    return New(endpoint, basePath, opts...)
  })
}
```

4) Add a blank import in `component/memory.go` so the registration runs:

```go
import _ "opencsg.com/csghub-server/builder/memory/your_backend"
```

## Recommended File Layout

```
csghub-server/builder/memory/your_backend/
  adapter.go   // Adapter implementation
  client.go    // HTTP/SDK client
  mapper.go    // Request/response mapping
  *_test.go    // Unit tests for mapping and client behavior
```

## Canonical Model Expectations

Canonical request/response types live in `csghub-server/common/types/memory.go`.

Key semantics:

- `AddMemoriesResponse` must return `created` as a list of memory messages.
- `SearchMemoriesResponse` and `ListMemoriesResponse` return a flat list of `MemoryMessage`.
- `MemoryMessage` `scopes` and `meta_data` are optional; omit empty values in responses.
- `DeleteMemoriesRequest` uses `uid` / `uids` (backend adapters may need to map prefixes or IDs).

## Backend-Specific Mapping (Example: MemMachine)

The MemMachine adapter demonstrates common patterns:

- Mapping `min_similarity` to the backend `score_threshold`.
- Converting MemMachine episodic/semantic shapes into a flat `MemoryMessage` list.
- Adding `agent_id` / `session_id` into backend metadata for add operations.
- Translating CSGHub `uid` prefixes for delete or list-by-uid requests.

See:

- `csghub-server/builder/memory/memmachine/mapper.go`
- `csghub-server/builder/memory/memmachine/client.go`

## Testing

Recommended test coverage:

- Request mapping (canonical → backend)
- Response mapping (backend → canonical)
- Error handling and status codes

Example:

```
cd csghub-server

go test ./builder/memory/memmachine
```

## Operational Notes

- Adapters should avoid per-request global checks; do expensive checks once at initialization.
- Avoid leaking backend-specific fields into canonical responses.
- When backend responses can be polymorphic, normalize into canonical structures.

## FAQ

**Q: How do I add a new backend without changing component code?**
A: Implement a new adapter, register it in `init()`, and add a blank import in `component/memory.go`.

**Q: Where do I put backend-specific config?**
A: Use existing `MemoryConfig` fields; if you need new fields, update `common/config/config.go`.
