package factory

import (
	"log/slog"

	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	email "opencsg.com/csghub-server/notification/notifychannel/channel/email"
	emailclient "opencsg.com/csghub-server/notification/notifychannel/channel/email/client"
	internalmsg "opencsg.com/csghub-server/notification/notifychannel/channel/internalmsg"
	"opencsg.com/csghub-server/notification/notifychannel/channel/sms"
	"opencsg.com/csghub-server/notification/notifychannel/channel/sms/client"
)

// Register channels
func registerChannels(config *config.Config, factory Factory) {
	// internal message channel
	internalMessageChannel := internalmsg.NewChannel(config, database.NewNotificationStore())
	factory.RegisterChannel(types.MessageChannelInternalMessage.String(), internalMessageChannel)

	// email channel
	registerEmailChannel(config, factory)

	// sms channel
	registerSMSChannel(config, factory)

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
	factory.RegisterChannel(types.MessageChannelEmail.String(), emailChannel)
}

func registerSMSChannel(config *config.Config, factory Factory) {
	smsService, err := client.NewAliyunSMSClient(config)
	if err != nil {
		slog.Error("failed to create aliyun sms client", "error", err)
		return
	}
	smsChannel := sms.NewSMSChannel(smsService)
	factory.RegisterChannel(types.MessageChannelSMS.String(), smsChannel)
}
