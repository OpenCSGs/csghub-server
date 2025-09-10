package component

import (
	"context"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
)

type StatComponent interface {
	GetStatSnap(ctx context.Context, req types.StatSnapshotReq) (*types.StatSnapshotResp, error)
	MakeStatSnap(ctx context.Context) error
	StatRunningDeploys(ctx context.Context) (map[int]*types.StatRunningDeploy, error)
}

func NewStatComponent(config *config.Config) (StatComponent, error) {
	return &statComponentImpl{
		config:          config,
		statSnapStore:   database.NewStatSnapStore(),
		deployTaskStore: database.NewDeployTaskStore(),
	}, nil
}

type statComponentImpl struct {
	config          *config.Config
	statSnapStore   database.StatSnapStore
	deployTaskStore database.DeployTaskStore
}
