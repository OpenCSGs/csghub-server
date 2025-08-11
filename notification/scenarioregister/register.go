package scenarioregister

import (
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/notification/scenariomgr"
	"opencsg.com/csghub-server/notification/scenariomgr/scenario/emailverifycode"
	"opencsg.com/csghub-server/notification/scenariomgr/scenario/internalnotification"
)

func Register(d *scenariomgr.DataProvider) {
	// register internal notification scenario
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

	// register asset management scenario
	scenariomgr.RegisterScenario(types.MessageScenarioAssetManagement, &scenariomgr.ScenarioDefinition{
		Channels: []types.MessageChannel{
			types.MessageChannelInternalMessage,
			types.MessageChannelEmail,
		},
		ChannelGetDataFunc: map[types.MessageChannel]scenariomgr.GetDataFunc{
			types.MessageChannelInternalMessage: internalnotification.GetSiteInternalMessageData,
			types.MessageChannelEmail:           internalnotification.GetEmailDataFunc(d.GetNotificationStorage()),
		},
	})

	// register user verify scenario
	scenariomgr.RegisterScenario(types.MessageScenarioUserVerify, &scenariomgr.ScenarioDefinition{
		Channels: []types.MessageChannel{
			types.MessageChannelInternalMessage,
			types.MessageChannelEmail,
		},
		ChannelGetDataFunc: map[types.MessageChannel]scenariomgr.GetDataFunc{
			types.MessageChannelInternalMessage: internalnotification.GetSiteInternalMessageData,
			types.MessageChannelEmail:           internalnotification.GetEmailDataFunc(d.GetNotificationStorage()),
		},
	})

	// register org verify scenario
	scenariomgr.RegisterScenario(types.MessageScenarioOrgVerify, &scenariomgr.ScenarioDefinition{
		Channels: []types.MessageChannel{
			types.MessageChannelInternalMessage,
			types.MessageChannelEmail,
		},
		ChannelGetDataFunc: map[types.MessageChannel]scenariomgr.GetDataFunc{
			types.MessageChannelInternalMessage: internalnotification.GetSiteInternalMessageData,
			types.MessageChannelEmail:           internalnotification.GetEmailDataFunc(d.GetNotificationStorage()),
		},
	})

	// register org member scenario
	scenariomgr.RegisterScenario(types.MessageScenarioOrgMember, &scenariomgr.ScenarioDefinition{
		Channels: []types.MessageChannel{
			types.MessageChannelInternalMessage,
			types.MessageChannelEmail,
		},
		ChannelGetDataFunc: map[types.MessageChannel]scenariomgr.GetDataFunc{
			types.MessageChannelInternalMessage: internalnotification.GetSiteInternalMessageData,
			types.MessageChannelEmail:           internalnotification.GetEmailDataFunc(d.GetNotificationStorage()),
		},
	})

	// register discussion scenario
	scenariomgr.RegisterScenario(types.MessageScenarioDiscussion, &scenariomgr.ScenarioDefinition{
		Channels: []types.MessageChannel{
			types.MessageChannelInternalMessage,
			types.MessageChannelEmail,
		},
		ChannelGetDataFunc: map[types.MessageChannel]scenariomgr.GetDataFunc{
			types.MessageChannelInternalMessage: internalnotification.GetSiteInternalMessageData,
			types.MessageChannelEmail:           internalnotification.GetEmailDataFunc(d.GetNotificationStorage()),
		},
	})

	// register email verify code scenario
	scenariomgr.RegisterScenario(types.MessageScenarioEmailVerifyCode, &scenariomgr.ScenarioDefinition{
		Channels: []types.MessageChannel{
			types.MessageChannelEmail,
		},
		ChannelGetDataFunc: map[types.MessageChannel]scenariomgr.GetDataFunc{
			types.MessageChannelEmail: emailverifycode.GetEmailData,
		},
	})

	// register deployment scenario
	scenariomgr.RegisterScenario(types.MessageScenarioDeployment, &scenariomgr.ScenarioDefinition{
		Channels: []types.MessageChannel{
			types.MessageChannelInternalMessage,
			types.MessageChannelEmail,
		},
		ChannelGetDataFunc: map[types.MessageChannel]scenariomgr.GetDataFunc{
			types.MessageChannelInternalMessage: internalnotification.GetSiteInternalMessageData,
			types.MessageChannelEmail:           internalnotification.GetEmailDataFunc(d.GetNotificationStorage()),
		},
	})

	extend(d)
}
