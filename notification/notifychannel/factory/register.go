package factory

import (
	"log/slog"

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
	// internal message channel
	internalMessageChannel := internalmsg.NewChannel(config, database.NewNotificationStore())
	factory.RegisterChannel(ChannelNameInternalMessage, internalMessageChannel)

	// email channel
	registerEmailChannel(config, factory)

	extendChannels(config, factory)
}

func registerEmailChannel(config *config.Config, factory Factory) {
	var emailService emailclient.EmailService
	var err error
	if config.Notification.DirectMailEnabled {
		emailService, err = emailclient.NewDirectMailClient(config)
		if err != nil {
			slog.Error("failed to create direct mail client", "error", err)
			return
		}
	} else {
		emailService = emailclient.NewEmailService(config)
	}

	emailChannel := email.NewChannel(config, emailService)
	factory.RegisterChannel(ChannelNameEmail, emailChannel)
}
