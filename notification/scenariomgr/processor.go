package scenariomgr

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"golang.org/x/sync/errgroup"
	"opencsg.com/csghub-server/builder/store/cache"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/notification/notifychannel"
	notifychannelfactory "opencsg.com/csghub-server/notification/notifychannel/factory"
	"opencsg.com/csghub-server/notification/tmplmgr"
	"opencsg.com/csghub-server/notification/utils"
)

const (
	retryMessageKeyPrefix = "notification:retry_message:"
	retryMessageTTL       = 10 * 60
)

type MessageProcessor struct {
	config               *config.Config
	templateManager      *tmplmgr.TemplateManager
	notifyChannelFactory notifychannelfactory.Factory
	redis                cache.RedisClient
}

type channelResult struct {
	channelName string
	success     bool
	err         error
}

func NewMessageProcessor(config *config.Config, templateManager *tmplmgr.TemplateManager, notifyChannelFactory notifychannelfactory.Factory) *MessageProcessor {
	redis, err := cache.NewCache(context.Background(), cache.RedisConfig{
		Addr:     config.Redis.Endpoint,
		Username: config.Redis.User,
		Password: config.Redis.Password,
	})
	if err != nil {
		slog.Error("failed to create redis client", "error", err)
		return nil
	}
	return &MessageProcessor{
		config:               config,
		templateManager:      templateManager,
		notifyChannelFactory: notifyChannelFactory,
		redis:                redis,
	}
}

func (p *MessageProcessor) ProcessMessage(ctx context.Context, msg types.ScenarioMessage) error {
	scenario, ok := scenarioRegistry[msg.Scenario]
	if !ok {
		return fmt.Errorf("scenario %s not registered", msg.Scenario)
	}

	if len(scenario.Channels) == 0 {
		slog.Warn("no channels registered for scenario", "scenario", msg.Scenario, "msg_uuid", msg.MsgUUID)
		return nil
	}

	var channels []types.MessageChannel

	retryChannels, err := p.getRetryChannels(ctx, msg)
	if err != nil {
		slog.Warn("failed to check is retry message", "scenario", msg.Scenario, "msg_uuid", msg.MsgUUID, "error", err)
	}

	if len(retryChannels) > 0 {
		slog.Info("retry channels found", "scenario", msg.Scenario, "msg_uuid", msg.MsgUUID, "retry_channels", retryChannels)
		for _, channelName := range retryChannels {
			channels = append(channels, types.MessageChannel(channelName))
		}
	} else {
		channels = scenario.Channels
	}

	msgChannelResults := make([]channelResult, len(channels))

	var g errgroup.Group
	var mu sync.Mutex
	for i, channelName := range channels {
		i, channelName := i, channelName
		g.Go(func() error {
			err := p.processChannel(ctx, msg, scenario, channelName)
			mu.Lock()
			msgChannelResults[i] = channelResult{
				channelName: string(channelName),
				success:     err == nil,
				err:         err,
			}
			mu.Unlock()
			return err
		})
	}
	if err := g.Wait(); err == nil {
		slog.Info("all channels processed successfully", "scenario", msg.Scenario, "msg_uuid", msg.MsgUUID, "channels", scenario.Channels)
		err := p.cleanRetryChannels(ctx, msg)
		if err != nil {
			slog.Error("failed to clean retry channels", "scenario", msg.Scenario, "msg_uuid", msg.MsgUUID, "error", err)
		}
		return nil
	}

	var failedChannels []string
	var toRetryChannels []string
	var toRetryChannelResults []channelResult
	for _, result := range msgChannelResults {
		if !result.success {
			failedChannels = append(failedChannels, result.channelName)
			if utils.IsErrSendMsg(result.err) {
				toRetryChannels = append(toRetryChannels, result.channelName)
				toRetryChannelResults = append(toRetryChannelResults, result)
			}
		}
	}

	if len(toRetryChannelResults) > 0 {
		slog.Error("failed to process some channels, will retry later", "scenario", msg.Scenario, "msg_uuid", msg.MsgUUID, "channels", scenario.Channels, "failed_channels", failedChannels, "to_retry_channels", toRetryChannels)
		err := p.setRetryChannels(ctx, msg, toRetryChannels)
		if err != nil {
			slog.Error("failed to record to retry channels", "scenario", msg.Scenario, "msg_uuid", msg.MsgUUID, "to_retry_channels", toRetryChannels, "error", err)
		}
		return toRetryChannelResults[0].err
	}

	return nil
}

func (p *MessageProcessor) processChannel(ctx context.Context, msg types.ScenarioMessage, scenario *ScenarioDefinition, channelName types.MessageChannel) error {
	var (
		notificationData *NotificationData
		dataErr          error
		getDataFunc      GetDataFunc
	)

	if scenario.ChannelGetDataFunc != nil {
		getDataFunc = scenario.ChannelGetDataFunc[types.MessageChannel(channelName)]
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
				Message:  messageData,
				Payload:  messageData,
				Receiver: &notifychannel.Receiver{IsBroadcast: true}, // default to broadcast
			}
		}
	}

	if dataErr != nil {
		slog.Error("failed to get data for scenario", "scenario", msg.Scenario, "msg_uuid", msg.MsgUUID, "channel", channelName, "error", dataErr)
		return dataErr
	}

	channel, err := p.notifyChannelFactory.GetChannel(string(channelName))
	if err != nil {
		slog.Error("failed to get notify channel", "scenario", msg.Scenario, "msg_uuid", msg.MsgUUID, "channel", channelName, "error", err)
		return err
	}

	var sendReq notifychannel.NotifyRequest
	sendReq.Priority = msg.Priority
	sendReq.Message = notificationData.Message
	sendReq.Receiver = notificationData.Receiver

	if channel.IsFormatRequired() {
		sendReq.Payload = notificationData.Payload
		lang := notificationData.Receiver.GetLanguage()
		formattedData, formatErr := p.templateManager.Format(msg.Scenario, channelName, notificationData.Payload, lang)
		if formatErr != nil {
			slog.Error("failed to format data", "scenario", msg.Scenario, "msg_uuid", msg.MsgUUID, "channel", channelName, "error", formatErr)
			return formatErr
		}
		sendReq.FormattedData = formattedData
	}

	if err := channel.Send(ctx, &sendReq); err != nil {
		slog.Error("failed to send message", "scenario", msg.Scenario, "msg_uuid", msg.MsgUUID, "channel", channelName, "error", err)
		return err
	}

	return nil
}

func (p *MessageProcessor) setRetryChannels(ctx context.Context, msg types.ScenarioMessage, channelNames []string) (err error) {
	retryMessageKey := fmt.Sprintf("%s%s", retryMessageKeyPrefix, msg.MsgUUID)
	err = p.redis.SetEx(ctx, retryMessageKey, strings.Join(channelNames, ","), time.Duration(retryMessageTTL)*time.Second)
	return
}

func (p *MessageProcessor) cleanRetryChannels(ctx context.Context, msg types.ScenarioMessage) (err error) {
	retryMessageKey := fmt.Sprintf("%s%s", retryMessageKeyPrefix, msg.MsgUUID)
	err = p.redis.Del(ctx, retryMessageKey)
	return
}

func (p *MessageProcessor) getRetryChannels(ctx context.Context, msg types.ScenarioMessage) (channels []string, err error) {
	retryMessageKey := fmt.Sprintf("%s%s", retryMessageKeyPrefix, msg.MsgUUID)
	retryChannels, err := p.redis.Get(ctx, retryMessageKey)
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, nil
		}
		return nil, err
	}
	if retryChannels == "" {
		return nil, nil
	}
	return strings.Split(retryChannels, ","), nil
}
