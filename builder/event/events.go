package event

import (
	"fmt"
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
