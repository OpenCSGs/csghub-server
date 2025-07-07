package event

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/nats-io/nats.go/jetstream"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/mq"
)

var (
	CSGHubServerDurableConsumerName string = "NoBalanceConsumerForCSGHubServer"
	CSGHubServiceConsumerName       string = "DeployServiceConsumerForCSGHubServer"
	DefaultEventPublisher           EventPublisher
)

type EventPublisher struct {
	Connector    mq.MessageQueue
	SyncInterval int //in minutes
}

// NewNatsConnector initializes a new connection to the NATS server

func InitEventPublisher(cfg *config.Config, natsHandler mq.MessageQueue) error {
	var handler mq.MessageQueue
	var err error
	if natsHandler == nil {
		handler, err = mq.GetOrInit(cfg)
		if err != nil {
			return err
		}
	} else {
		handler = natsHandler
	}
	DefaultEventPublisher = EventPublisher{
		Connector:    handler,
		SyncInterval: cfg.Event.SyncInterval,
	}
	return nil
}

func (ec *EventPublisher) BuildServiceStream() error {
	return ec.Connector.BuildDeployServiceStream()
}

func (ec *EventPublisher) CreateServiceConsumer() (jetstream.Consumer, error) {
	return ec.Connector.BuildDeployServiceConsumerWithName(CSGHubServiceConsumerName)
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

func (ec *EventPublisher) PublishServiceEvent(message []byte) error {
	var err error
	for i := 0; i < 3; i++ {
		err = ec.Connector.VerifyDeployServiceStream()
		if err != nil {
			time.Sleep(2 * time.Second)
			continue
		}
		err = ec.Connector.PublishDeployServiceData(message)
		if err == nil {
			break
		}
		time.Sleep(1 * time.Second)
	}

	if err != nil {
		return fmt.Errorf("failed to publish service event for 3 retries, %w", err)
	}

	return nil
}

func (c *EventPublisher) ParseServiceMessageData(msg jetstream.Msg) (*types.ServiceEvent, error) {
	strData := string(msg.Data())
	evt := types.ServiceEvent{}
	err := json.Unmarshal(msg.Data(), &evt)
	if err != nil {
		return nil, fmt.Errorf("fail to unmarshal fee event, %v, %w", strData, err)
	}
	return &evt, nil
}
