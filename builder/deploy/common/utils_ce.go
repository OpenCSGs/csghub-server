//go:build !saas && !ee

package common

import "opencsg.com/csghub-server/common/types"

func GenerateScheduler(config DeployConfig) *types.Scheduler {
	return nil
}
