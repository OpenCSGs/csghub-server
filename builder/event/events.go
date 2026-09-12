package event

import (
	"fmt"
	"log/slog"
	"time"

	bldmq "opencsg.com/csghub-server/builder/mq"
	"opencsg.com/csghub-server/common/config"
)

var (
	CSGHubServerDurableConsumerName string = "NoBalanceConsumerForCSGHubServer"
	CSGHubOrderExpiredConsumerName  string = "OrderExpiredConsumerForCSGHubServer"
	DefaultEventPublisher           EventPublisher
)

type EventPublisher struct {
	SyncInterval int //in minutes
	MQ           bldmq.MessageQueue
}

// NewNatsConnector initializes a new connection to the NATS server
func InitEventPublisher(cfg *config.Config) error {
	mqFactory, err := bldmq.GetOrInitMessageQueueFactory(cfg)
	if err != nil {
		return fmt.Errorf("error creating message queue factory: %w", err)
	}
	mq, err := mqFactory.GetInstance()
	if err != nil {
		return fmt.Errorf("error creating message queue instance: %w", err)
	}

	DefaultEventPublisher = EventPublisher{
		SyncInterval: cfg.Event.SyncInterval,
		MQ:           mq,
	}
	return nil
}

// Todo: update order code logic later
// func (ec *EventPublisher) CreateOrderExpiredConsumer() (jetstream.Consumer, error) {
// 	return ec.Connector.BuildOrderConsumerWithName(CSGHubOrderExpiredConsumerName)
// }

// Publish a message to the specified subject
func (ec *EventPublisher) PublishMeteringEvent(message []byte) error {
	var err error
	for range 3 {
		err = ec.MQ.Publish(bldmq.MeterDurationSendSubject, message)
		if err == nil {
			break
		}
		time.Sleep(1 * time.Second)
	}

	if err != nil {
		return fmt.Errorf("failed to publish metering event for 3 retries, %w", err)
	}

	return nil
}

func (ec *EventPublisher) PublishRechargeEvent(message []byte) error {
	var err error
	for range 3 {
		err = ec.MQ.Publish(bldmq.RechargeSucceedSubject, message)
		if err == nil {
			break
		}
		time.Sleep(1 * time.Second)
	}

	if err != nil {
		return fmt.Errorf("failed to publish payment recharge event for 3 retries, %w", err)
	}

	return nil
}

func (ec *EventPublisher) PublishLLMLogTrainingEvent(message []byte) error {
	var err error
	for range 3 {
		err = ec.MQ.Publish(bldmq.LLMLogSubject, message)
		if err == nil {
			break
		}
		time.Sleep(1 * time.Second)
	}

	if err != nil {
		return fmt.Errorf("failed to publish llmlog training event for 3 retries, %w", err)
	}

	return nil
}

// PublishDeployUpstreamSyncEvent publishes a deploy upstream sync event to the
// specified subject. It is fire-and-forget: publishing runs asynchronously so
// the caller (deploy lifecycle callback) is never blocked. Retries up to 3
// times with 1s delay inside the goroutine. The error return is always nil —
// failures are logged but never surfaced, because the Temporal cron
// reconciliation will catch up on any missed events.
func (ec *EventPublisher) PublishDeployUpstreamSyncEvent(subject string, message []byte) error {
	if ec.MQ == nil {
		return nil
	}
	go func() {
		var err error
		for range 3 {
			err = ec.MQ.Publish(subject, message)
			if err == nil {
				return
			}
			// Sleep before retrying. The fire-and-forget goroutine has no
			// cancellable context available (the caller's ctx may already be
			// done by the time this runs), so a plain timer is the simplest
			// correct backoff. 3 attempts × 1s is bounded and short.
			time.Sleep(1 * time.Second)
		}
		slog.Error("failed to publish deploy upstream sync event after 3 retries",
			slog.String("subject", subject), slog.Any("error", err))
	}()
	return nil
}

