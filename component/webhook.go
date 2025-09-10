package component

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"reflect"
	"time"

	bldmq "opencsg.com/csghub-server/builder/mq"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/component/executors"
)

type WebHookComponent interface {
	HandleWebHook(ctx context.Context, event *types.WebHookRecvEvent) error
	DispatchWebHookEvent() error
}

type webHookComponentImpl struct {
	cfg *config.Config
	mq  bldmq.MessageQueue
}

func NewWebHookComponent(config *config.Config, mqFactory bldmq.MessageQueueFactory) (WebHookComponent, error) {
	// init heartbeat executor
	_, err := executors.NewHeartbeatExecutor(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create heartbeat executor error: %w", err)
	}

	// init cluster executor
	_, err = executors.NewClusterExecutor(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create cluster executor error: %w", err)
	}

	// init image builder
	_, err = executors.NewImageBuilderExecutor(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create image builder executor error: %w", err)
	}

	// init evaluation
	_, err = executors.NewArgoWorkflowExecutor(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create evaluation executor error: %w", err)
	}

	// init kservice executor
	_, err = executors.NewKServiceExecutor(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create kservice executor error: %w", err)
	}

	mq, err := mqFactory.GetInstance()
	if err != nil {
		return nil, fmt.Errorf("failed to get mq instance error: %w", err)
	}

	return &webHookComponentImpl{
		cfg: config,
		mq:  mq,
	}, nil
}

func (w *webHookComponentImpl) HandleWebHook(ctx context.Context, event *types.WebHookRecvEvent) error {
	buf, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal webhook event error: %w", err)
	}

	err = w.mq.Publish(w.cfg.MQ.WebHookEventRunnerSubject, buf)
	if err != nil {
		return fmt.Errorf("failed to publish webhook event error: %w", err)
	}
	return nil
}

func (w *webHookComponentImpl) DispatchWebHookEvent() error {
	err := w.mq.Subscribe(bldmq.SubscribeParams{
		Group:    bldmq.WebhookEventGroup,
		Topics:   []string{w.cfg.MQ.WebHookEventRunnerSubject},
		AutoACK:  true,
		Callback: w.dispatchMsgWithRetry,
	})

	if err != nil {
		return fmt.Errorf("failed to subscribe webhook event error: %w", err)
	}

	return nil
}

func (w *webHookComponentImpl) dispatchMsgWithRetry(raw []byte, meta bldmq.MessageMeta) error {
	strData := string(raw)
	slog.Debug("mq.webhook.received", slog.Any("msg.subject", meta.Topic), slog.Any("msg.data", strData))

	var err error = nil
	for range 3 {
		err = w.dispatchExecutor(raw)
		if err == nil {
			break
		}
		time.Sleep(1 * time.Second)
	}

	if err != nil {
		slog.Error("webhook dispatch a single msg with 3 retries", slog.Any("subject", meta.Topic), slog.Any("msg.data", strData), slog.Any("error", err))
		// return fmt.Errorf("failed to dispatch webhook event error: %w", err)
	}

	return nil
}

func (w *webHookComponentImpl) dispatchExecutor(raw []byte) error {
	var event types.WebHookRecvEvent

	err := json.Unmarshal(raw, &event)
	if err != nil {
		return fmt.Errorf("failed to unmarshal webhook event error: %w", err)
	}

	executor, ok := executors.EventExecutors[event.EventType]
	if !ok {
		return fmt.Errorf("no executor found for webhook event type %s", event.EventType)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	executorType := reflect.TypeOf(executor).String()
	err = executor.ProcessEvent(ctx, &event)
	if err != nil {
		return fmt.Errorf("failed to process webhook event by %s error: %w", executorType, err)
	}

	return nil
}
