package component

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go/jetstream"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/mq"
	notifychannelfactory "opencsg.com/csghub-server/notification/notifychannel/factory"
	scenariomgr "opencsg.com/csghub-server/notification/scenariomgr"
	"opencsg.com/csghub-server/notification/scenarioregister"
	"opencsg.com/csghub-server/notification/tmplmgr"
	"opencsg.com/csghub-server/notification/utils"
)

type NotificationComponent interface {
	GetUnreadCount(ctx context.Context, userUUID string) (int64, error)
	ListNotifications(ctx context.Context, uid string, req types.NotificationsRequest) ([]types.Notifications, int, error)
	MarkAsRead(ctx context.Context, uid string, req types.BatchNotificationOperationReq) error
	MarkAsUnread(ctx context.Context, uid string, req types.BatchNotificationOperationReq) error
	DeleteNotifications(ctx context.Context, uid string, req types.BatchNotificationOperationReq) error
	NotificationsSetting(ctx context.Context, uid string, req types.UpdateNotificationReq, location *time.Location) error
	GetNotificationSetting(ctx context.Context, uid string, location *time.Location) (*types.NotificationSetting, error)
	PollNewNotifications(ctx context.Context, userUUID string, limit int, location *time.Location) (*types.NewNotifications, error)
	PublishMessage(ctx context.Context, message types.ScenarioMessage) error
}

type notificationComponentImpl struct {
	conf                          *config.Config
	storage                       database.NotificationStore
	queue                         mq.MessageQueue
	highPriorityMessageConsumer   jetstream.Consumer
	normalPriorityMessageConsumer jetstream.Consumer
	highPriorityMsgCh             chan jetstream.Msg
	normalPriorityMsgCh           chan jetstream.Msg
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
	}

	n, err := mq.GetOrInit(conf)
	if err != nil {
		slog.Error("failed to init nats", slog.Any("error", err))
		return nil, err
	}
	nmc.queue = n

	if err = n.BuildHighPriorityMsgStream(conf); err != nil {
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

	if err = n.BuildNormalPriorityMsgStream(conf); err != nil {
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

	nmc.highPriorityMsgCh = make(chan jetstream.Msg, conf.Notification.HighPriorityMsgBufferSize)
	nmc.normalPriorityMsgCh = make(chan jetstream.Msg, conf.Notification.NormalPriorityMsgBufferSize)
	nmc.startMsgDispatcher(conf.Notification.MsgDispatcherCount, nmc.handleScenarioMessage)
	return nmc, nil
}

func (c *notificationComponentImpl) processHighPriorityMsg() error {
	slog.Info("start process high priority message")
	_, err := c.highPriorityMessageConsumer.Consume(func(msg jetstream.Msg) {
		slog.Debug("received high priority message", slog.String("data", string(msg.Data())))
		select {
		case c.highPriorityMsgCh <- msg:
			slog.Info("added high priority message to dispatcher")
		default:
			slog.Error("high priority message channel is full, rejecting message", slog.String("message", string(msg.Data())))
			if err := msg.Nak(); err != nil {
				slog.Error("failed to nak message", slog.Any("error", err))
			}
		}
	})
	if err != nil {
		slog.Error("failed to consume high priority message", slog.Any("error", err))
		return err
	}
	return nil
}

func (c *notificationComponentImpl) processNormalPriorityMsg() error {
	slog.Info("start process normal priority message")
	_, err := c.normalPriorityMessageConsumer.Consume(func(msg jetstream.Msg) {
		slog.Debug("received normal priority message", slog.String("data", string(msg.Data())))
		select {
		case c.normalPriorityMsgCh <- msg:
			slog.Info("added normal priority message to dispatcher")
		default:
			slog.Error("normal priority message channel is full, rejecting message", slog.String("message", string(msg.Data())))
			if err := msg.Nak(); err != nil {
				slog.Error("failed to nak message", slog.Any("error", err))
			}
		}
	})
	if err != nil {
		slog.Error("failed to consume normal priority message", slog.Any("error", err))
		return err
	}
	return nil
}

func (c *notificationComponentImpl) startMsgDispatcher(count int, handler func(jetstream.Msg)) {
	slog.Info("starting message dispatcher", slog.Int("dispatcher_count", count))
	for i := 0; i < count; i++ {
		go func(i int) {
			c.runMsgDispatcher(i, handler)
		}(i)
	}
}

func (c *notificationComponentImpl) runMsgDispatcher(id int, handler func(jetstream.Msg)) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("message dispatcher panic recovered, will restart", slog.Int("dispatcher_id", id), slog.Any("error", r))
			go c.runMsgDispatcher(id, handler)
		}
	}()

	slog.Info("message dispatcher started", slog.Int("dispatcher_id", id))
	for {
		select {
		case msg := <-c.highPriorityMsgCh:
			slog.Debug("message dispatcher received high priority message", slog.Int("dispatcher_id", id), slog.String("data", string(msg.Data())))
			handler(msg)
		default:
			select {
			case msg := <-c.highPriorityMsgCh:
				slog.Debug("dispatcher handled high priority message", slog.Int("dispatcher_id", id))
				handler(msg)
			case msg := <-c.normalPriorityMsgCh:
				slog.Debug("dispatcher handled normal priority message", slog.Int("dispatcher_id", id))
				handler(msg)
			}
		}
	}
}

func (c *notificationComponentImpl) handleScenarioMessage(msg jetstream.Msg) {
	var message types.ScenarioMessage
	if err := json.Unmarshal(msg.Data(), &message); err != nil {
		slog.Error("failed to unmarshal message", slog.Any("data", string(msg.Data())), slog.Any("error", err))
		if err := msg.Term(); err != nil {
			slog.Error("failed to term message", slog.Any("error", err))
		}
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- c.messageProcessor.ProcessMessage(ctx, message)
	}()

	select {
	case err := <-done:
		if err != nil {
			slog.Error("failed to handle scenario", slog.Any("scenario", message.Scenario), slog.Any("error", err))
			if utils.IsErrSendMsg(err) {
				slog.Info("failed to send message, will retry later", slog.Any("scenario", message.Scenario), slog.String("message id", message.MsgUUID), slog.Any("error", err))
				if err := msg.Nak(); err != nil {
					slog.Error("failed to nak message", slog.Any("error", err))
				}
				return
			}
			if err := msg.Term(); err != nil {
				slog.Error("failed to term message", slog.Any("scenario", message.Scenario), slog.String("message id", message.MsgUUID), slog.Any("error", err))
			}
		} else {
			slog.Info("scenario handled successfully", slog.Any("scenario", message.Scenario), slog.String("message id", message.MsgUUID))
			if err := msg.Ack(); err != nil {
				slog.Error("failed to ack message", slog.Any("scenario", message.Scenario), slog.String("message id", message.MsgUUID), slog.Any("error", err))
			}
		}
	case <-ctx.Done():
		slog.Error("scenario handling timeout", slog.Any("scenario", message.Scenario), slog.String("message id", message.MsgUUID), slog.Any("timeout", 30*time.Second))
		if err := msg.Nak(); err != nil {
			slog.Error("failed to nak message", slog.Any("scenario", message.Scenario), slog.String("message id", message.MsgUUID), slog.Any("error", err))
		}
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
			Template:         item.Template,
			Payload:          item.Payload,
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
			Template:         item.Template,
			Payload:          item.Payload,
			IsRead:           !item.ReadAt.IsZero(),
			ClickActionURL:   item.ActionURL,
			CreatedAt:        item.CreatedAt.Unix(),
			UpdatedAt:        item.UpdatedAt.Unix(),
		}
	}

	return result, total, nil
}

func (c *notificationComponentImpl) MarkAsRead(ctx context.Context, uid string, req types.BatchNotificationOperationReq) error {
	if req.MarkAll {
		return c.storage.MarkAllAsRead(ctx, uid)
	} else if len(req.IDs) == 0 {
		return nil
	}
	return c.storage.MarkAsRead(ctx, uid, req.IDs)
}

func (c *notificationComponentImpl) MarkAsUnread(ctx context.Context, uid string, req types.BatchNotificationOperationReq) error {
	if req.MarkAll {
		return c.storage.MarkAllAsUnread(ctx, uid)
	} else if len(req.IDs) == 0 {
		return nil
	}
	return c.storage.MarkAsUnread(ctx, uid, req.IDs)
}

func (c *notificationComponentImpl) DeleteNotifications(ctx context.Context, uid string, req types.BatchNotificationOperationReq) error {
	if req.MarkAll {
		return c.storage.DeleteAllNotifications(ctx, uid)
	} else if len(req.IDs) == 0 {
		return nil
	}
	return c.storage.DeleteNotifications(ctx, uid, req.IDs)
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
		slog.Info("publish message to high priority queue", slog.Any("message id", message.MsgUUID))
		if err := c.queue.PublishHighPriorityMsg(message); err != nil {
			return fmt.Errorf("failed to publish message to high priority queue: %w", err)
		}
	case types.MessagePriorityNormal:
		slog.Info("publish message to normal priority queue", slog.Any("message id", message.MsgUUID))
		if err := c.queue.PublishNormalPriorityMsg(message); err != nil {
			return fmt.Errorf("failed to publish message to normal priority queue: %w", err)
		}
	default:
		return fmt.Errorf("unsupported priority: %s", message.Priority)
	}
	return nil
}
