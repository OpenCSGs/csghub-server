//go:build !saas && !ee

package kube_scheduler

import "opencsg.com/csghub-server/common/types"

func NewApplier(config *types.Scheduler) Applier {
	return &DefaultOpApplier{}
}
