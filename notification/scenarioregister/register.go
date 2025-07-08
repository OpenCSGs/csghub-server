package scenarioregister

import (
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/notification/scenariomgr"
	"opencsg.com/csghub-server/notification/scenariomgr/scenario/internalnotification"
)

func Register(d *scenariomgr.DataProvider) {
	scenariomgr.RegisterScenario(types.MessageScenarioInternalNotification, &scenariomgr.ScenarioDefinition{
		Channels: []types.MessageChannel{
			types.MessageChannelInternalMessage,
			types.MessageChannelEmail,
		},
		ChannelGetDataFunc: map[types.MessageChannel]scenariomgr.GetDataFunc{
			types.MessageChannelInternalMessage: internalnotification.GetSiteInternalMessageData,
			types.MessageChannelEmail:           internalnotification.NewGetInternalNotificationEmailDataFunc(d.GetNotificationStorage()),
		},
	})

	extend(d)
}
