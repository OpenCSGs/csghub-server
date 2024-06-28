package event

import (
	"fmt"
	"time"

	"github.com/nats-io/nats.go/jetstream"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/mq"
)

var (
	CSGHubServerDurableConsumerName string = "NoBalanceConsumerForCSGHubServer"
	DefaultEventPublisher           EventPublisher
)

type EventPublisher struct {
	Connector         *mq.NatsHandler
	FeeRequestSubject string
	RecvNotifySubject string
	SyncInterval      int //in minutes
}

// NewNatsConnector initializes a new connection to the NATS server
func InitEventPublisher(cfg *config.Config) error {
	hander, err := mq.Init(cfg)
	if err != nil {
		return err
	}
	DefaultEventPublisher = EventPublisher{
		Connector:         hander,
		FeeRequestSubject: cfg.Nats.FeeSendSubject,
		RecvNotifySubject: cfg.Nats.FeeNotifyNoBalanceSubject,
		SyncInterval:      cfg.Event.SyncInterval,
	}
	return nil
}

func (ec *EventPublisher) CreateNoBalanceConsumer() (jetstream.Consumer, error) {
	return ec.Connector.BuildNotifyConsumer(CSGHubServerDurableConsumerName)
}

// Publish sends a message to the specified subject
func (ec *EventPublisher) PublishFeeEvent(message []byte) error {
	var err error
	for i := 0; i < 3; i++ {
		err = ec.Connector.VerifyEventStream()
		if err != nil {
			continue
		}
		err = ec.Connector.PublishData(ec.FeeRequestSubject, message)
		if err == nil {
			break
		}
		time.Sleep(1 * time.Second)
	}

	if err != nil {
		return fmt.Errorf("failed to publish event for retry 3 times, %w", err)
	}

	return nil
}
