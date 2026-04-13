//go:build !saas && !ee

package component

import (
	"opencsg.com/csghub-server/builder/sensitive"
	"opencsg.com/csghub-server/common/config"
)

func sensitiveChainOption(config *config.Config, provider string) []sensitive.ChainOption {
	return nil
}
