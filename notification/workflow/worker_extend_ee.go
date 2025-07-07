//go:build ee || saas

package workflow

import (
	"opencsg.com/csghub-server/builder/temporal"
	"opencsg.com/csghub-server/common/config"

	// blank import to register workers via their init() function.
	// lark is only available in ee and saas
	_ "opencsg.com/csghub-server/notification/notifychannel/channel/lark/workflow"
)

func extendWorker(_ *config.Config, _ temporal.Client) {
}
