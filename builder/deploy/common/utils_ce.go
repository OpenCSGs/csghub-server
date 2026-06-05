//go:build !saas && !ee

package common

import "opencsg.com/csghub-server/common/types"

func GenerateScheduler(VXPUConfig map[string]string) *types.Scheduler {
	return nil
}
