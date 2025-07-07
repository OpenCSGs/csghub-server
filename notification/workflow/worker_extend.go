//go:build !ee && !saas

package workflow

import (
	"opencsg.com/csghub-server/builder/temporal"
	"opencsg.com/csghub-server/common/config"
)

func extendWorker(_ *config.Config, _ temporal.Client) {
}
