//go:build ee

package scenarioregister

import (
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/notification/scenariomgr"
	"opencsg.com/csghub-server/notification/scenariomgr/scenario/reposync"
)

func extend(_ *scenariomgr.DataProvider) {
	scenariomgr.RegisterScenario(types.MessageScenarioRepoSync, &scenariomgr.ScenarioDefinition{
		Channels: []types.MessageChannel{
			types.MessageChannelLark,
		},
		DefaultGetDataFunc: reposync.GetRepoSyncNotification,
	})
}
