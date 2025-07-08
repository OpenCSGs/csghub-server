package factory

import (
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	email "opencsg.com/csghub-server/notification/notifychannel/channel/email"
	emailclient "opencsg.com/csghub-server/notification/notifychannel/channel/email/client"
	internalmsg "opencsg.com/csghub-server/notification/notifychannel/channel/internalmsg"
)

const (
	ChannelNameInternalMessage = "internal-message"
	ChannelNameEmail           = "email"
)

// Register channels
func registerChannels(config *config.Config, factory Factory) {
	internalMessageChannel := internalmsg.NewChannel(config, database.NewNotificationStore())
	factory.RegisterChannel(ChannelNameInternalMessage, internalMessageChannel)

	emailChannel := email.NewChannel(config, emailclient.NewEmailService(config))
	factory.RegisterChannel(ChannelNameEmail, emailChannel)

	extendChannels(config, factory)
}
