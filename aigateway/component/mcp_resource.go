package component

import (
	"context"

	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
)

type MCPResourceComponent interface {
	List(ctx context.Context, filter *types.MCPFilter) ([]database.MCPResource, int, error)
}

type mcpResourceComponentImpl struct {
	mcpResStore database.MCPResourceStore
}

func NewMCPResourceComponent(config *config.Config) MCPResourceComponent {
	return &mcpResourceComponentImpl{
		mcpResStore: database.NewMCPResourceStore(),
	}
}

func (c *mcpResourceComponentImpl) List(ctx context.Context, filter *types.MCPFilter) ([]database.MCPResource, int, error) {
	return c.mcpResStore.List(ctx, filter)
}
