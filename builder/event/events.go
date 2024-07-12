package event

import (
	"fmt"
	"time"

	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/mq"
)

var (
	CSGHubServerDurableConsumerName string = "NoBalanceConsumerForCSGHubServer"
	DefaultEventPublisher           EventPublisher
)

type EventPublisher struct {
	Connector    *mq.NatsHandler
	SyncInterval int //in minutes
}

// NewNatsConnector initializes a new connection to the NATS server
func InitEventPublisher(cfg *config.Config) error {
	hander, err := mq.Init(cfg)
	if err != nil {
		return err
	}
	DefaultEventPublisher = EventPublisher{
		Connector:    hander,
		SyncInterval: cfg.Event.SyncInterval,
	}
	return nil
}

// Publish a message to the specified subject
func (ec *EventPublisher) PublishMeteringEvent(message []byte) error {
	var err error
	for i := 0; i < 3; i++ {
		err = ec.Connector.VerifyMeteringStream()
		if err != nil {
			time.Sleep(2 * time.Second)
			continue
		}
		err = ec.Connector.PublishMeterDurationData(message)
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
