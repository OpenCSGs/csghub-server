package scenariomgr

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/notification/notifychannel"
	notifychannelfactory "opencsg.com/csghub-server/notification/notifychannel/factory"
	"opencsg.com/csghub-server/notification/tmplmgr"
)

type MessageProcessor struct {
	config               *config.Config
	templateManager      *tmplmgr.TemplateManager
	notifyChannelFactory notifychannelfactory.Factory
}

func NewMessageProcessor(config *config.Config, templateManager *tmplmgr.TemplateManager, notifyChannelFactory notifychannelfactory.Factory) *MessageProcessor {
	return &MessageProcessor{
		config:               config,
		templateManager:      templateManager,
		notifyChannelFactory: notifyChannelFactory,
	}
}

func (p *MessageProcessor) ProcessMessage(ctx context.Context, msg types.ScenarioMessage) error {
	scenario, ok := scenarioRegistry[msg.Scenario]
	if !ok {
		return fmt.Errorf("scenario %s not registered", msg.Scenario)
	}

	for _, channelName := range scenario.Channels {
		var (
			notificationData *NotificationData
			dataErr          error
			getDataFunc      GetDataFunc
		)

		if scenario.ChannelGetDataFunc != nil {
			getDataFunc = scenario.ChannelGetDataFunc[channelName]
		}

		if getDataFunc == nil {
			getDataFunc = scenario.DefaultGetDataFunc
		}

		if getDataFunc != nil {
			notificationData, dataErr = getDataFunc(ctx, p.config, msg)
		} else {
			// fallback: unmarshal parameters and create default receiver
			var messageData any
			if dataErr = json.Unmarshal([]byte(msg.Parameters), &messageData); dataErr == nil {
				notificationData = &NotificationData{
					MessageData: messageData,
					Receiver:    &notifychannel.Receiver{IsBroadcast: true}, // default to broadcast
				}
			}
		}

		if dataErr != nil {
			slog.Error("failed to get data for scenario", "scenario", msg.Scenario, "channel", channelName, "error", dataErr)
			continue
		}

		channel, err := p.notifyChannelFactory.GetChannel(string(channelName))
		if err != nil {
			slog.Error("failed to get notify channel", "scenario", msg.Scenario, "channel", channelName, "error", err)
			continue
		}

		var sendReq notifychannel.NotifyRequest
		sendReq.Priority = msg.Priority
		sendReq.MessageData = notificationData.MessageData
		sendReq.Receiver = notificationData.Receiver

		if channel.IsFormatRequired() {
			formattedData, formatErr := p.templateManager.Format(msg.Scenario, channelName, notificationData.MessageData)
			if formatErr != nil {
				slog.Error("failed to format data", "scenario", msg.Scenario, "channel", channelName, "error", formatErr)
				continue
			}
			sendReq.Payload = formattedData
		}

		if err := channel.Send(ctx, &sendReq); err != nil {
			slog.Error("failed to send message", "scenario", msg.Scenario, "channel", channelName, "error", err)
			continue
		}
	}

	return nil
}
