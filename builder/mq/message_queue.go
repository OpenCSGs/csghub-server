package mq

import (
	"fmt"
	"log/slog"
	"sync"

	"opencsg.com/csghub-server/common/config"
)

type MessageQueue interface {
	Publish(topic string, raw []byte) error
	Subscribe(params SubscribeParams) error
}

type MessageQueueFactory interface {
	GetInstance() (MessageQueue, error)
}

var _ MessageQueueFactory = (*messageQueueFactoryImpl)(nil)

type messageQueueFactoryImpl struct {
	nats  MessageQueue
	kafka MessageQueue
}

var (
	globalMQFactory MessageQueueFactory
	once            sync.Once
)

func GetOrInitMessageQueueFactory(config *config.Config) (MessageQueueFactory, error) {
	var err error
	once.Do(func() {
		globalMQFactory, err = initMQFactory(config)
	})
	return globalMQFactory, err
}

func initMQFactory(config *config.Config) (MessageQueueFactory, error) {
	var err error
	mqf := &messageQueueFactoryImpl{}

	if len(config.Nats.URL) > 0 {
		mqf.nats, err = NewNats(config)
		if err != nil {
			return nil, err
		}
		slog.Info("[nats] message queue initialized successfully")
		return mqf, nil
	}

	if len(config.Kafka.Servers) > 0 {
		mqf.kafka, err = NewKafka(config)
		if err != nil {
			return nil, err
		}
		slog.Info("[kafka] message queue initialized successfully")
	}

	return mqf, nil
}

func (f *messageQueueFactoryImpl) GetInstance() (MessageQueue, error) {
	if f.nats != nil {
		return f.nats, nil
	}
	if f.kafka != nil {
		return f.kafka, nil
	}
	return nil, fmt.Errorf("no message queue instance found")
}
