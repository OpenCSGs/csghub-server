package memory

import (
	"context"

	"opencsg.com/csghub-server/common/types"
)

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
