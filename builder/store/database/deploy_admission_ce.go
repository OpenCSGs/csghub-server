//go:build !ee && !saas

package database

import (
	"context"
	"github.com/uptrace/bun"
)

func (s *deployTaskStoreImpl) writeDeployAdmission(_ context.Context, _ *Deploy, write func(bun.IDB) error) error {
	return write(s.db.Core)
}
func (s *deployTaskStoreImpl) writeDeployTaskAdmission(_ context.Context, _ int64, write func(bun.IDB) error) error {
	return write(s.db.Core)
}

func (s *deployTaskStoreImpl) DispatchPendingTask(_ context.Context, _ *DeployTask, submit func(DeployTaskStore) error) error {
	return submit(s)
}
