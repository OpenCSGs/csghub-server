package memmachine

import (
	"context"
	"time"

	"opencsg.com/csghub-server/builder/memory"
	"opencsg.com/csghub-server/builder/rpc"
	"opencsg.com/csghub-server/common/types"
)

type Adapter struct {
	client *Client
}

func New(endpoint, basePath string, opts ...rpc.RequestOption) *Adapter {
	return &Adapter{client: NewClient(endpoint, basePath, opts...)}
}

func init() {
	memory.RegisterAdapter("memmachine", func(endpoint, basePath string, opts ...rpc.RequestOption) memory.Adapter {
		return New(endpoint, basePath, opts...)
	})
}

func (a *Adapter) WithTimeout(timeout time.Duration) {
	a.client.SetTimeout(timeout)
}

func (a *Adapter) WithRetry(attempts uint) {
	a.client.WithRetry(attempts)
}

func (a *Adapter) WithDelay(delay time.Duration) {
	a.client.WithDelay(delay)
}

func (a *Adapter) CreateProject(ctx context.Context, req *types.CreateMemoryProjectRequest) (*types.MemoryProjectResponse, error) {
	return a.client.CreateProject(ctx, req)
}

func (a *Adapter) GetProject(ctx context.Context, req *types.GetMemoryProjectRequest) (*types.MemoryProjectResponse, error) {
	return a.client.GetProject(ctx, req)
}

func (a *Adapter) ListProjects(ctx context.Context) ([]*types.MemoryProjectRef, error) {
	return a.client.ListProjects(ctx)
}

func (a *Adapter) DeleteProject(ctx context.Context, req *types.DeleteMemoryProjectRequest) error {
	return a.client.DeleteProject(ctx, req)
}

func (a *Adapter) AddMemories(ctx context.Context, req *types.AddMemoriesRequest) (*types.AddMemoriesResponse, error) {
	return a.client.AddMemories(ctx, req)
}

func (a *Adapter) SearchMemories(ctx context.Context, req *types.SearchMemoriesRequest) (*types.SearchMemoriesResponse, error) {
	return a.client.SearchMemories(ctx, req)
}

func (a *Adapter) ListMemories(ctx context.Context, req *types.ListMemoriesRequest) (*types.ListMemoriesResponse, error) {
	return a.client.ListMemories(ctx, req)
}

func (a *Adapter) DeleteMemories(ctx context.Context, req *types.DeleteMemoriesRequest) error {
	return a.client.DeleteMemories(ctx, req)
}

func (a *Adapter) Health(ctx context.Context) (*types.MemoryHealthResponse, error) {
	return a.client.Health(ctx)
}

var _ memory.Adapter = (*Adapter)(nil)
