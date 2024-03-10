package database

import (
	"context"
	"fmt"
	"log/slog"
)

type DeploymentStore struct {
	db *DB
}

func NewDeploymentStore() *DeploymentStore {
	return &DeploymentStore{db: defaultDB}
}

type DeploymentStatus int

const (
	Building DeploymentStatus = iota + 1
	Deploying
	Starup
	Running
	Stopped
	BuildingFailed
	DeployFailed
	RuntimeError
)

type Deployment struct {
	ID        int64            `bun:",pk,autoincrement" json:"id"`
	SpaceID   int64            `bun:",notnull" json:"space_id"`
	Space     *Space           `bun:"rel:belongs-to,join:space_id=id" json:"space"`
	GitPath   string           `bun:",notnull" json:"git_path"`
	GitBranch string           `bun:",notnull" json:"git_branch"`
	Env       string           `bun:",notnull" json:"env"`
	Secrets   string           `bun:",notnull" json:"secrets"`
	Template  string           `bun:",notnull" json:"template"`
	Hardware  string           `bun:",notnull" json:"hardware"`
	Status    DeploymentStatus `bun:",notnull" json:"status"`
	times
}

func (s *DeploymentStore) Create(ctx context.Context, input Deployment) (*Deployment, error) {
	res, err := s.db.Core.NewInsert().Model(&input).Exec(ctx, &input)
	if err := assertAffectedOneRow(res, err); err != nil {
		slog.Error("create deployment task in db failed", slog.String("error", err.Error()))
		return nil, fmt.Errorf("create deployment task in db failed, error:%w", err)
	}

	return &input, nil
}
