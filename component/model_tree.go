package component

import (
	"context"

	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
)

type ModelTreeComponent interface {
	GetModelTree(ctx context.Context, currentUser, namespace, name string) (*types.ModelTree, error)
	ProcessModelTree(ctx context.Context, relations []*types.ModelNode, currentRepo database.Repository)
	ScanModelTree(ctx context.Context, req types.ScanModels) error
}
