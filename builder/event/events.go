package event

import (
	"fmt"
	"time"

	"github.com/nats-io/nats.go/jetstream"
	bldmq "opencsg.com/csghub-server/builder/mq"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/mq"
)

var (
	CSGHubServerDurableConsumerName string = "NoBalanceConsumerForCSGHubServer"
	CSGHubOrderExpiredConsumerName  string = "OrderExpiredConsumerForCSGHubServer"
	DefaultEventPublisher           EventPublisher
)

type EventPublisher struct {
	Connector    mq.MessageQueue
	SyncInterval int //in minutes
	MQ           bldmq.MessageQueue
	Cfg          *config.Config
}

// NewNatsConnector initializes a new connection to the NATS server
func InitEventPublisher(cfg *config.Config) error {
	handler, err := mq.GetOrInit(cfg)
	if err != nil {
		return fmt.Errorf("error creating message queue handler: %w", err)
	}

	mqFactory, err := bldmq.GetOrInitMessageQueueFactory(cfg)
	if err != nil {
		return fmt.Errorf("error creating message queue factory: %w", err)
	}
	mq, err := mqFactory.GetInstance()
	if err != nil {
		return fmt.Errorf("error creating message queue instance: %w", err)
	}

	DefaultEventPublisher = EventPublisher{
		Connector:    handler,
		SyncInterval: cfg.Event.SyncInterval,
		MQ:           mq,
		Cfg:          cfg,
	}
	return nil
}

func (ec *EventPublisher) CreateOrderExpiredConsumer() (jetstream.Consumer, error) {
	return ec.Connector.BuildOrderConsumerWithName(CSGHubOrderExpiredConsumerName)
}

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
	for i := 0; i < 3; i++ {
		err = ec.Connector.VerifyRechargeStream()
		if err != nil {
			time.Sleep(2 * time.Second)
			continue
		}
		err = ec.Connector.PublishRechargeDurationData(message)
		if err == nil {
			break
		}
		time.Sleep(1 * time.Second)
	}

	if err != nil {
		return fmt.Errorf("failed to publish recharge event for 3 retries, %w", err)
	}

	return nil
}
