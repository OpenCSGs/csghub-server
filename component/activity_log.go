package component

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	bldmq "opencsg.com/csghub-server/builder/mq"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
)

const (
	activityLogBatchSize     = 50
	activityLogFlushInterval = 5 * time.Second
)

type ActivityLogComponent interface {
	PublishActivityLog(ctx context.Context, log *types.ActivityLog) error
	StartConsuming() error
	ListActivityLogs(ctx context.Context, req types.QueryActivityLogReq) ([]database.ActivityLog, int, error)
}

type activityLogComponentImpl struct {
	store       database.ActivityLogStore
	mq          bldmq.MessageQueue
	logCh       chan *database.ActivityLog
	logBuffer   []database.ActivityLog
	flushTicker *time.Ticker
}

var defaultActivityLogComponent ActivityLogComponent

func NewActivityLogComponent(config *config.Config, mqFactory bldmq.MessageQueueFactory) (ActivityLogComponent, error) {
	if defaultActivityLogComponent != nil {
		return defaultActivityLogComponent, nil
	}

	mq, err := mqFactory.GetInstance()
	if err != nil {
		return nil, err
	}

	c := &activityLogComponentImpl{
		store:       database.NewActivityLogStore(),
		mq:          mq,
		logCh:       make(chan *database.ActivityLog, 200),
		logBuffer:   make([]database.ActivityLog, 0, activityLogBatchSize),
		flushTicker: time.NewTicker(activityLogFlushInterval),
	}

	go c.runBatchWriter()
	defaultActivityLogComponent = c
	return c, nil
}

func (c *activityLogComponentImpl) PublishActivityLog(ctx context.Context, log *types.ActivityLog) error {
	data, err := json.Marshal(log)
	if err != nil {
		return err
	}
	return c.mq.Publish(bldmq.ActivityLogSendSubject, data)
}

func (c *activityLogComponentImpl) StartConsuming() error {
	return c.mq.Subscribe(bldmq.SubscribeParams{
		Group:    bldmq.ActivityLogGroup,
		Topics:   []string{bldmq.ActivityLogSendSubject},
		AutoACK:  true,
		Callback: c.handleActivityLogMsg,
	})
}

func (c *activityLogComponentImpl) handleActivityLogMsg(raw []byte, meta bldmq.MessageMeta) error {
	var msg types.ActivityLog
	if err := json.Unmarshal(raw, &msg); err != nil {
		slog.Error("failed to unmarshal activity log message", slog.Any("error", err))
		return err
	}

	dbLog := &database.ActivityLog{
		UserUUID:      msg.UserID,
		Username:      msg.Username,
		Action:        msg.Action,
		AuthType:      msg.AuthType,
		ResourceType:  msg.ResourceType,
		ResourceID:    msg.ResourceID,
		ResourceName:  msg.ResourceName,
		IPAddress:     msg.IPAddress,
		UserAgent:     msg.UserAgent,
		OperationTime: msg.OperationTime,
	}

	c.logCh <- dbLog
	return nil
}

func (c *activityLogComponentImpl) runBatchWriter() {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("activity log batch writer panicked, restarting", slog.Any("panic", r))
			go c.runBatchWriter()
		}
	}()

	for {
		select {
		case logEntry := <-c.logCh:
			c.logBuffer = append(c.logBuffer, *logEntry)
			if len(c.logBuffer) >= activityLogBatchSize {
				c.flushBuffer()
			}
		case <-c.flushTicker.C:
			c.flushBuffer()
		}
	}
}

func (c *activityLogComponentImpl) flushBuffer() {
	if len(c.logBuffer) == 0 {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	batch := make([]database.ActivityLog, len(c.logBuffer))
	copy(batch, c.logBuffer)
	c.logBuffer = c.logBuffer[:0]

	deduplicated := c.deduplicateLogs(batch)

	if err := c.store.BatchCreate(ctx, deduplicated); err != nil {
		slog.Error("failed to batch create activity logs", slog.Any("error", err), slog.Int("count", len(deduplicated)))
	}
}

type activityLogKey struct {
	UserUUID     string
	Action       string
	AuthType     string
	ResourceType string
	ResourceName string
	ResourceID   int64
}

func (c *activityLogComponentImpl) deduplicateLogs(logs []database.ActivityLog) []database.ActivityLog {
	seen := make(map[activityLogKey]struct{}, len(logs))
	result := make([]database.ActivityLog, 0, len(logs))
	for _, log := range logs {
		key := activityLogKey{
			UserUUID:     log.UserUUID,
			Action:       log.Action,
			AuthType:     log.AuthType,
			ResourceType: log.ResourceType,
			ResourceName: log.ResourceName,
			ResourceID:   log.ResourceID,
		}
		if _, exists := seen[key]; !exists {
			seen[key] = struct{}{}
			result = append(result, log)
		}
	}
	return result
}

func (c *activityLogComponentImpl) ListActivityLogs(ctx context.Context, req types.QueryActivityLogReq) ([]database.ActivityLog, int, error) {
	return c.store.FindByTimeAfter(ctx, req.After, req.Per, req.Page)
}
