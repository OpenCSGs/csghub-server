//go:build saas

package scenarioregister

import (
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/notification/scenariomgr"
	"opencsg.com/csghub-server/notification/scenariomgr/scenario/internalnotification"
	"opencsg.com/csghub-server/notification/scenariomgr/scenario/rechargesuccess"
	"opencsg.com/csghub-server/notification/scenariomgr/scenario/reposync"
	"opencsg.com/csghub-server/notification/scenariomgr/scenario/weeklyrecharges"
)

func extend(d *scenariomgr.DataProvider) {
	// register repo sync scenario
	scenariomgr.RegisterScenario(types.MessageScenarioRepoSync, &scenariomgr.ScenarioDefinition{
		Channels: []types.MessageChannel{
			types.MessageChannelLark,
		},
		DefaultGetDataFunc: reposync.GetRepoSyncNotification,
	})

	// register recharge scenario
	scenariomgr.RegisterScenario(types.MessageScenarioRecharge, &scenariomgr.ScenarioDefinition{
		Channels: []types.MessageChannel{
			types.MessageChannelInternalMessage,
			types.MessageChannelEmail,
		},
		ChannelGetDataFunc: map[types.MessageChannel]scenariomgr.GetDataFunc{
			types.MessageChannelInternalMessage: internalnotification.GetSiteInternalMessageData,
			types.MessageChannelEmail:           internalnotification.GetEmailDataFunc(d.GetNotificationStorage()),
		},
	})

	// register low balance scenario
	scenariomgr.RegisterScenario(types.MessageScenarioLowBalance, &scenariomgr.ScenarioDefinition{
		Channels: []types.MessageChannel{
			types.MessageChannelInternalMessage,
			types.MessageChannelEmail,
		},
		ChannelGetDataFunc: map[types.MessageChannel]scenariomgr.GetDataFunc{
			types.MessageChannelInternalMessage: internalnotification.GetSiteInternalMessageData,
			types.MessageChannelEmail:           internalnotification.GetEmailDataFunc(d.GetNotificationStorage()),
		},
	})

	// register email recharge success
	scenariomgr.RegisterScenario(types.MessageScenarioRechargeSuccess, &scenariomgr.ScenarioDefinition{
		Channels: []types.MessageChannel{
			types.MessageChannelEmail,
		},
		ChannelGetDataFunc: map[types.MessageChannel]scenariomgr.GetDataFunc{
			types.MessageChannelEmail: rechargesuccess.GetEmailDataFunc(d.GetUserSvcClient()),
		},
	})

	// register email weekly recharges
	scenariomgr.RegisterScenario(types.MessageScenarioWeeklyRecharges, &scenariomgr.ScenarioDefinition{
		Channels: []types.MessageChannel{
			types.MessageChannelEmail,
		},
		ChannelGetDataFunc: map[types.MessageChannel]scenariomgr.GetDataFunc{
			types.MessageChannelEmail: weeklyrecharges.GetEmailData,
		},
	})

}
