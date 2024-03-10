package database

import (
	"context"
	"fmt"
	"log/slog"
)

type DeploymentTaskStore struct {
	db *DB
}

func NewDeploymentTaskStore() *DeploymentTaskStore {
	return &DeploymentTaskStore{db: defaultDB}
}

type DeploymentTask struct {
	ID           int64            `bun:",pk,autoincrement" json:"id"`
	DeploymentID int64            `bun:",notnull" json:"deployment_id"`
	Deployment   *Deployment      `bun:"rel:belongs-to,join:deployment_id=id" json:"deployment"`
	TaskType     int              `bun:",notnull" json:"task_type"`
	Message      string           `bun:"" json:"message"`
	Status       DeploymentStatus `bun:",notnull" json:"status"`
	times
}

func (s *DeploymentTaskStore) Create(ctx context.Context, input DeploymentTask) (*DeploymentTask, error) {
	res, err := s.db.Core.NewInsert().Model(&input).Exec(ctx, &input)
	if err := assertAffectedOneRow(res, err); err != nil {
		slog.Error("create deployment task in db failed", slog.String("error", err.Error()))
		return nil, fmt.Errorf("create deployment task in db failed, error:%w", err)
	}

	return &input, nil
}
