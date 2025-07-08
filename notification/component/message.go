package component

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go/jetstream"
	"go.temporal.io/sdk/client"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/mq"
	"opencsg.com/csghub-server/notification/mailer"
	internalmsgworkflow "opencsg.com/csghub-server/notification/notifychannel/channel/internalmsg/workflow"
	notifychannelfactory "opencsg.com/csghub-server/notification/notifychannel/factory"
	scenariomgr "opencsg.com/csghub-server/notification/scenariomgr"
	"opencsg.com/csghub-server/notification/scenarioregister"
	"opencsg.com/csghub-server/notification/tmplmgr"
	"opencsg.com/csghub-server/notification/utils"
	"opencsg.com/csghub-server/notification/workflow"
)

type NotificationComponent interface {
	GetUnreadCount(ctx context.Context, userUUID string) (int64, error)
	ListNotifications(ctx context.Context, uid string, req types.NotificationsRequest) ([]types.Notifications, int, error)
	MarkAsRead(ctx context.Context, uid string, req types.MarkNotificationsAsReadReq) error
	// NotificationsSetting
	NotificationsSetting(ctx context.Context, uid string, req types.UpdateNotificationReq, location *time.Location) error
	//GET Setting
	GetNotificationSetting(ctx context.Context, uid string, location *time.Location) (*types.NotificationSetting, error)

	PollNewNotifications(ctx context.Context, userUUID string, limit int, location *time.Location) (*types.NewNotifications, error)

	PublishMessage(ctx context.Context, message types.ScenarioMessage) error
	PublishNotificationMessage(ctx context.Context, message types.NotificationMessage) error
}

type notificationComponentImpl struct {
	conf                          *config.Config
	consumer                      jetstream.Consumer
	storage                       database.NotificationStore
	mailer                        mailer.MailerInterface
	queue                         mq.MessageQueue
	highPriorityMessageConsumer   jetstream.Consumer
	normalPriorityMessageConsumer jetstream.Consumer
	messageProcessor              *scenariomgr.MessageProcessor
}

var _ NotificationComponent = (*notificationComponentImpl)(nil)

func NewMockNotificationComponent(storage database.NotificationStore) NotificationComponent {
	return &notificationComponentImpl{
		storage: storage,
	}
}

func NewNotificationComponent(conf *config.Config) (NotificationComponent, error) {
	nmc := &notificationComponentImpl{
		conf:    conf,
		storage: database.NewNotificationStore(),
		mailer:  mailer.NewMailer(conf),
	}

	n, err := mq.GetOrInit(conf)
	if err != nil {
		slog.Error("failed to init nats", slog.Any("error", err))
		return nil, err
	}
	nmc.queue = n

	if err = n.BuildSiteInternalMsgStream(); err != nil {
		slog.Error("failed to build site internal message stream", slog.Any("error", err))
		return nil, err
	}
	consumer, err := n.BuildSiteInternalMsgConsumer()
	if err != nil {
		slog.Error("failed to build site internal message consumer", slog.Any("error", err))
		return nil, err
	}

	nmc.consumer = consumer
	if err = nmc.processSiteInternalMessages(); err != nil {
		slog.Error("failed to start processing site internal messages", slog.Any("error", err))
		return nil, err
	}

	if err = n.BuildHighPriorityMsgStream(); err != nil {
		slog.Error("failed to build high priority message stream", slog.Any("error", err))
		return nil, err
	}
	highPriorityMessageConsumer, err := n.BuildHighPriorityMsgConsumer()
	if err != nil {
		slog.Error("failed to build high priority message consumer", slog.Any("error", err))
		return nil, err
	}
	nmc.highPriorityMessageConsumer = highPriorityMessageConsumer
	if err = nmc.processHighPriorityMsg(); err != nil {
		slog.Error("failed to process high priority message", slog.Any("error", err))
		return nil, err
	}

	if err = n.BuildNormalPriorityMsgStream(); err != nil {
		slog.Error("failed to build normal priority message stream", slog.Any("error", err))
		return nil, err
	}
	normalPriorityMessageConsumer, err := n.BuildNormalPriorityMsgConsumer()
	if err != nil {
		slog.Error("failed to build normal priority message consumer", slog.Any("error", err))
		return nil, err
	}
	nmc.normalPriorityMessageConsumer = normalPriorityMessageConsumer
	if err = nmc.processNormalPriorityMsg(); err != nil {
		slog.Error("failed to process normal priority message", slog.Any("error", err))
		return nil, err
	}

	channelFactory := notifychannelfactory.NewFactory(conf)
	templateManager := tmplmgr.NewTemplateManager()

	dataProvider := scenariomgr.NewDataProvider(nmc.storage)
	scenarioregister.Register(dataProvider)
	nmc.messageProcessor = scenariomgr.NewMessageProcessor(conf, templateManager, channelFactory)

	return nmc, nil
}

func (c *notificationComponentImpl) processHighPriorityMsg() error {
	slog.Info("start process high priority message")
	_, err := c.highPriorityMessageConsumer.Consume(c.handleScenarioMessage)
	if err != nil {
		slog.Error("failed to consume high priority message", slog.Any("error", err))
		return err
	}
	return nil
}

func (c *notificationComponentImpl) processNormalPriorityMsg() error {
	slog.Info("start process normal priority message")
	_, err := c.normalPriorityMessageConsumer.Consume(c.handleScenarioMessage)
	if err != nil {
		slog.Error("failed to consume normal priority message", slog.Any("error", err))
		return err
	}
	return nil
}

func (c *notificationComponentImpl) handleScenarioMessage(msg jetstream.Msg) {
	defer func() {
		if err := msg.Ack(); err != nil {
			slog.Error("failed to ack message", slog.Any("error", err))
		}
	}()

	var message types.ScenarioMessage
	if err := json.Unmarshal(msg.Data(), &message); err != nil {
		slog.Error("failed to unmarshal message", slog.Any("data", string(msg.Data())), slog.Any("error", err))
		return
	}

	go func() {
		slog.Debug("handle message", slog.Any("message", message))
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		if err := c.messageProcessor.ProcessMessage(ctx, message); err != nil {
			slog.Error("failed to process message", slog.Any("message", message), slog.Any("error", err))
			return
		}
	}()
}

func (c *notificationComponentImpl) processSiteInternalMessages() error {
	slog.Info("start processing site internal messages")
	_, err := c.consumer.Consume(c.handleSiteInternalMessage)
	if err != nil {
		slog.Error("failed to consume site internal message", slog.Any("error", err))
		return err
	}

	return nil
}

func (c *notificationComponentImpl) handleSiteInternalMessage(msg jetstream.Msg) {
	defer func() {
		if err := msg.Ack(); err != nil {
			slog.Error("failed to ack message", slog.Any("error", err))
		}
	}()

	slog.Debug("handle site internal message", slog.Any("data", string(msg.Data())))
	var message types.NotificationMessage
	if err := json.Unmarshal(msg.Data(), &message); err != nil {
		slog.Error("failed to unmarshal site internal message", slog.Any("data", string(msg.Data())), slog.Any("error", err))
		return
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		err := c.dispatchSiteInternalMessage(ctx, message)
		if err != nil {
			slog.Error("failed to create message task", slog.Any("message", message), slog.Any("error", err))
		}
	}()

}

func (c *notificationComponentImpl) dispatchSiteInternalMessage(ctx context.Context, message types.NotificationMessage) error {
	if !message.NotificationType.IsValid() {
		slog.Error("invalid notification type", slog.Any("notification_type", message.NotificationType))
		return fmt.Errorf("invalid notification type: %s", message.NotificationType)
	}
	if message.MsgUUID == "" {
		slog.Error("msg_uuid is empty", slog.Any("message", message))
		return fmt.Errorf("msg_uuid is empty")
	}

	if message.CreateAt.IsZero() {
		message.CreateAt = time.Now()
	}

	siteInternalMessage := &database.NotificationMessage{
		MsgUUID:          message.MsgUUID,
		GroupID:          "",
		NotificationType: message.NotificationType.String(),
		SenderUUID:       message.SenderUUID,
		Summary:          message.Summary,
		Title:            message.Title,
		Content:          message.Content,
		ActionURL:        message.ClickActionURL,
		Priority:         message.Priority,
	}

	if len(message.UserUUIDs) == 0 {
		// broadcast site internal message to all users
		if err := c.broadcastSiteInternalMessage(ctx, siteInternalMessage); err != nil {
			slog.Error("failed to broadcast site internal message", slog.Any("message id", message.MsgUUID), slog.Any("error", err))
			return err
		}
	} else {
		// send site internal message to users
		if err := c.sendSiteInternalMessageToUsers(ctx, siteInternalMessage, message.UserUUIDs); err != nil {
			slog.Error("failed to send site internal message to users", slog.Any("message id", message.MsgUUID), slog.Any("error", err))
			return err
		}
		slog.Info("send site internal message to users successfully", slog.Any("message id", message.MsgUUID))
	}
	// send email to users
	// TODO: refactor later
	go c.sendEmail(message.UserUUIDs, message)

	return nil
}

func (c *notificationComponentImpl) broadcastSiteInternalMessage(ctx context.Context, siteInternalMessage *database.NotificationMessage) error {
	slog.Info("broadcast site internal message to all users", slog.Any("message id", siteInternalMessage.MsgUUID))

	existed, err := c.storage.IsNotificationMessageExists(ctx, siteInternalMessage.MsgUUID)
	if err != nil {
		slog.Error("failed to check if notification message exists", slog.Any("message id", siteInternalMessage.MsgUUID), slog.Any("error", err))
		return err
	}
	if existed {
		slog.Info("site internal message already exists, skipped", slog.Any("message id", siteInternalMessage.MsgUUID))
		return nil
	}

	if err := c.storage.CreateNotificationMessage(ctx, siteInternalMessage); err != nil {
		slog.Error("failed to create site internal message", slog.Any("message id", siteInternalMessage.MsgUUID), slog.Any("error", err))
		return err
	}
	slog.Info("create site internal message successfully", slog.Any("message id", siteInternalMessage.MsgUUID))

	// start workflow to send site internal message to all users
	workflowClient := workflow.GetWorkflowClient()
	if workflowClient == nil {
		return fmt.Errorf("workflow client is nil")
	}
	workflowOptions := client.StartWorkflowOptions{
		TaskQueue: internalmsgworkflow.WorkflowBroadcastInternalMessageQueueName,
	}
	we, err := workflowClient.ExecuteWorkflow(context.Background(), workflowOptions, internalmsgworkflow.BroadcastInternalMessageWorkflow, siteInternalMessage.MsgUUID, c.conf.Notification.BroadcastUserPageSize)
	if err != nil {
		slog.Error("failed to start broadcast message workflow", slog.Any("message id", siteInternalMessage.MsgUUID), slog.Any("error", err))
		return err
	}
	slog.Info("start broadcast message workflow", slog.Any("workflow id", we.GetID()), slog.Any("message id", siteInternalMessage.MsgUUID))
	return nil
}

func (c *notificationComponentImpl) sendSiteInternalMessageToUsers(ctx context.Context, siteInternalMessage *database.NotificationMessage, users []string) error {
	if err := c.storage.CreateNotificationMessageForUsers(ctx, siteInternalMessage, users); err != nil {
		slog.Error("failed to send site internal message for users", slog.Any("message id", siteInternalMessage.MsgUUID), slog.Any("users", strings.Join(users, ",")), slog.Any("error", err))
		return fmt.Errorf("failed to send site internal message for users: %w", err)
	}
	slog.Info("send site internal message for users successfully", slog.Any("message id", siteInternalMessage.MsgUUID), slog.Any("user uuids", users))
	return nil
}

func (c *notificationComponentImpl) sendEmail(users []string, msg types.NotificationMessage) {
	var settings []*database.NotificationSetting
	var err error
	if len(users) != 0 {
		settings, err = c.storage.GetSettingByUserUUIDs(context.Background(), users)
		if err != nil {
			slog.Error("failed to get settings", slog.Any("users", users), slog.Any("error", err))
			return
		}
	} else {
		// all
		settings, err = c.storage.GetAllSettings(context.Background())
		if err != nil {
			slog.Error("failed to get settings", slog.Any("users", users), slog.Any("error", err))
			return
		}
	}

	var userEmails []string
	for _, setting := range settings {
		if setting.IsEmailNotificationEnabled {
			userEmails = append(userEmails, setting.EmailAddress)
		}
	}

	if len(userEmails) == 0 {
		return
	}
	body, err := c.mailer.FormatNotifyMsg(msg)
	if err != nil {
		slog.Error("failed to format message", slog.Any("error", err))
		return
	}
	emailReq := types.EmailReq{
		To:          userEmails,
		Subject:     msg.Title,
		Body:        body,
		ContentType: types.ContentTypeTextHTML,
	}
	if err := c.mailer.Send(emailReq); err != nil {
		slog.Error("failed to send email", slog.Any("users emails", userEmails), slog.Any("error", err))
		return
	}
}

func (c *notificationComponentImpl) PollNewNotifications(ctx context.Context, userUUID string, limit int, location *time.Location) (*types.NewNotifications, error) {
	var nextPollTime time.Time
	result := make([]types.Notifications, 0)

	setting, err := c.storage.GetSetting(ctx, userUUID)
	if err != nil {
		return nil, err
	}

	messages, total, err := c.storage.GetUnNotifiedMessages(ctx, userUUID, limit)
	if err != nil {
		return nil, err
	}

	if total > limit {
		nextPollTime = time.Now().Add(10 * time.Second)
	} else {
		nextPollTime = time.Now().Add(1 * time.Minute)
	}

	if len(messages) == 0 {
		return &types.NewNotifications{
			NextPollTime: nextPollTime,
			Data:         result,
		}, nil
	}

	msgUUIDs := make([]string, len(messages))
	for i, msg := range messages {
		msgUUIDs[i] = msg.MsgUUID
	}

	if err := c.storage.MarkAsNotified(ctx, userUUID, msgUUIDs); err != nil {
		return nil, err
	}

	if setting != nil && setting.IsDoNotDisturbEnabled {
		now := time.Now().In(location)
		currentTimeInLocal := time.Date(2000, 1, 1, now.Hour(), now.Minute(), now.Second(), 0, location)

		startTime := setting.DoNotDisturbStartTime
		startTimeInLocal := time.Date(2000, 1, 1, startTime.Hour(), startTime.Minute(), 0, 0, location)
		endTime := setting.DoNotDisturbEndTime
		endTimeInLocal := time.Date(2000, 1, 1, endTime.Hour(), endTime.Minute(), 0, 0, location)

		if endTimeInLocal.Before(startTimeInLocal) {
			if currentTimeInLocal.After(startTimeInLocal) || currentTimeInLocal.Before(endTimeInLocal) {
				return &types.NewNotifications{
					NextPollTime: nextPollTime,
					Data:         result,
				}, nil
			}
		} else {
			if currentTimeInLocal.After(startTimeInLocal) && currentTimeInLocal.Before(endTimeInLocal) {
				return &types.NewNotifications{
					NextPollTime: nextPollTime,
					Data:         result,
				}, nil
			}
		}
	}

	for _, item := range messages {
		if setting != nil && !utils.IsStringInArray(item.NotificationType, setting.SubNotificationType) {
			continue
		}
		result = append(result, types.Notifications{
			ID:               item.ID,
			UserUUID:         item.UserUUID,
			SenderUUID:       item.SenderUUID,
			NotificationType: item.NotificationType,
			Title:            item.Title,
			Summary:          item.Summary,
			Content:          item.Content,
			IsRead:           !item.ReadAt.IsZero(),
			ClickActionURL:   item.ActionURL,
			CreatedAt:        item.CreatedAt.Unix(),
			UpdatedAt:        item.UpdatedAt.Unix(),
		})
	}

	return &types.NewNotifications{
		NextPollTime: nextPollTime,
		Data:         result,
	}, nil
}

func (c *notificationComponentImpl) GetUnreadCount(ctx context.Context, userUUID string) (int64, error) {
	return c.storage.GetUnreadCount(ctx, userUUID)
}

func (c *notificationComponentImpl) ListNotifications(ctx context.Context, uid string, req types.NotificationsRequest) ([]types.Notifications, int, error) {
	params := database.ListNotificationsParams{
		UserUUID:         uid,
		NotificationType: req.NotificationType,
		TitleKeyword:     req.Title,
		Page:             req.Page,
		PageSize:         req.PageSize,
		UnreadOnly:       req.UnReadOnly,
	}

	list, total, err := c.storage.ListNotificationMessages(ctx, params)
	if err != nil {
		return nil, 0, err
	}

	result := make([]types.Notifications, len(list))
	for i, item := range list {
		result[i] = types.Notifications{
			ID:               item.ID,
			UserUUID:         item.UserUUID,
			SenderUUID:       item.SenderUUID,
			NotificationType: item.NotificationType,
			Title:            item.Title,
			Summary:          item.Summary,
			Content:          item.Content,
			IsRead:           !item.ReadAt.IsZero(),
			ClickActionURL:   item.ActionURL,
			CreatedAt:        item.CreatedAt.Unix(),
			UpdatedAt:        item.UpdatedAt.Unix(),
		}
	}

	return result, total, nil
}

func (c *notificationComponentImpl) MarkAsRead(ctx context.Context, uid string, req types.MarkNotificationsAsReadReq) error {
	if req.MarkAll {
		return c.storage.MarkAllAsRead(ctx, uid)
	} else if len(req.IDs) == 0 {
		return nil
	}
	return c.storage.MarkAsRead(ctx, uid, req.IDs)
}

func (c *notificationComponentImpl) NotificationsSetting(ctx context.Context, uid string, req types.UpdateNotificationReq, location *time.Location) error {
	setting, err := c.storage.GetSetting(ctx, uid)
	if err != nil {
		return err
	}
	isNew := false
	if setting == nil {
		setting = &database.NotificationSetting{
			UserUUID:            uid,
			SubNotificationType: req.SubNotificationType,
			MessageTTL:          database.DefaultMessageTTL,
		}
		isNew = true
	}

	setting.SubNotificationType = req.SubNotificationType

	if req.IsEmailNotificationEnabled {
		if req.EmailAddress == "" {
			return fmt.Errorf("email address is empty")
		}
	}

	if req.EmailAddress != "" {
		setting.EmailAddress = req.EmailAddress
	}

	setting.IsEmailNotificationEnabled = req.IsEmailNotificationEnabled
	if req.IsSMSNotificationEnabled {
		if req.PhoneNumber == "" {
			return fmt.Errorf("phone number is empty")
		}
	}

	if req.PhoneNumber != "" {
		setting.PhoneNumber = req.PhoneNumber
	}

	setting.IsSMSNotificationEnabled = req.IsSMSNotificationEnabled

	setting.IsDoNotDisturbEnabled = req.IsDoNotDisturbEnabled
	setting.DoNotDisturbStartTime = req.DoNotDisturbStartTime
	setting.DoNotDisturbEndTime = req.DoNotDisturbEndTime

	if req.MessageTTL > 0 {
		setting.MessageTTL = time.Duration(req.MessageTTL)
	}

	if isNew {
		return c.storage.CreateSetting(ctx, setting)
	}
	return c.storage.UpdateSetting(ctx, setting)
}

func (c *notificationComponentImpl) GetNotificationSetting(ctx context.Context, uid string, location *time.Location) (*types.NotificationSetting, error) {
	setting, err := c.storage.GetSetting(ctx, uid)
	if err != nil {
		return nil, err
	}
	if setting == nil {
		return &types.NotificationSetting{
			UserUUID:            uid,
			SubNotificationType: types.NotificationTypeAll(),
			MessageTTL:          int64(database.DefaultMessageTTL),
		}, nil
	}

	startTime := setting.DoNotDisturbStartTime
	startTimeInLocal := time.Date(2000, 1, 1, startTime.Hour(), startTime.Minute(), 0, 0, location)
	endTime := setting.DoNotDisturbEndTime
	endTimeInLocal := time.Date(2000, 1, 1, endTime.Hour(), endTime.Minute(), 0, 0, location)

	return &types.NotificationSetting{
		UserUUID:                   setting.UserUUID,
		SubNotificationType:        setting.SubNotificationType,
		EmailAddress:               setting.EmailAddress,
		IsEmailNotificationEnabled: setting.IsEmailNotificationEnabled,
		MessageTTL:                 int64(setting.MessageTTL),
		PhoneNumber:                setting.PhoneNumber,
		IsSMSNotificationEnabled:   setting.IsSMSNotificationEnabled,
		DoNotDisturbStart:          startTimeInLocal.Format("15:04"),
		DoNotDisturbEnd:            endTimeInLocal.Format("15:04"),
		IsDoNotDisturbEnabled:      setting.IsDoNotDisturbEnabled,
		CreatedAt:                  setting.CreatedAt,
		UpdatedAt:                  setting.UpdatedAt,
	}, nil
}

func (c *notificationComponentImpl) PublishMessage(ctx context.Context, message types.ScenarioMessage) error {
	message.MsgUUID = uuid.New().String()
	message.CreatedAt = time.Now()

	switch message.Priority {
	case types.MessagePriorityHigh:
		slog.Debug("publish message to high priority queue", slog.Any("message id", message.MsgUUID))
		if err := c.queue.PublishHighPriorityMsg(message); err != nil {
			return fmt.Errorf("failed to publish message to high priority queue: %w", err)
		}
	case types.MessagePriorityNormal:
		slog.Debug("publish message to normal priority queue", slog.Any("message id", message.MsgUUID))
		if err := c.queue.PublishNormalPriorityMsg(message); err != nil {
			return fmt.Errorf("failed to publish message to normal priority queue: %w", err)
		}
	default:
		return fmt.Errorf("unsupported priority: %s", message.Priority)
	}
	return nil
}

func (c *notificationComponentImpl) PublishNotificationMessage(ctx context.Context, message types.NotificationMessage) error {
	if err := c.queue.PublishSiteInternalMsg(message); err != nil {
		return fmt.Errorf("failed to publish message to site internal message queue: %w", err)
	}
	slog.Info("published notification message to site internal message queue", slog.Any("message id", message.MsgUUID))
	return nil
}
