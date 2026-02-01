package mq

import (
	"fmt"
	"log/slog"

	"opencsg.com/csghub-server/common/config"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
)

var _ MessageQueue = (*Kafka)(nil)

type Kafka struct {
	cfg      *config.Config
	producer *kafka.Producer
}

func NewKafka(config *config.Config) (MessageQueue, error) {
	conf := &kafka.ConfigMap{
		"bootstrap.servers": config.Kafka.Servers,
	}
	p, err := kafka.NewProducer(conf)
	if err != nil {
		return nil, fmt.Errorf("[kafka] failed to create producer error: %w", err)
	}
	return &Kafka{
		cfg:      config,
		producer: p,
	}, nil
}

func (k *Kafka) Publish(topic string, data []byte) error {
	err := k.producer.Produce(&kafka.Message{
		TopicPartition: kafka.TopicPartition{Topic: &topic, Partition: kafka.PartitionAny},
		Value:          data,
	}, nil)
	if err != nil {
		return fmt.Errorf("[kafka] failed to produce message error: %w", err)
	}
	return nil
}

func (k *Kafka) Subscribe(params SubscribeParams) error {
	conf := &kafka.ConfigMap{
		"bootstrap.servers":  k.cfg.Kafka.Servers,
		"group.id":           params.Group.ConsumerName,
		"auto.offset.reset":  "earliest",
		"enable.auto.commit": "false",
	}

	consumer, err := kafka.NewConsumer(conf)
	if err != nil {
		return fmt.Errorf("[kafka] failed to create consumer error: %w", err)
	}

	err = consumer.SubscribeTopics(params.Topics, nil)
	if err != nil {
		return fmt.Errorf("[kafka] failed to subscribe topics error: %w", err)
	}

	go k.startConsumer(consumer, params)

	return nil
}

func (k *Kafka) startConsumer(consumer *kafka.Consumer, params SubscribeParams) {
	for {
		ev := consumer.Poll(100)
		if ev == nil {
			continue
		}

		switch e := ev.(type) {
		case *kafka.Message:
			err := params.Callback(e.Value, MessageMeta{
				Topic: *e.TopicPartition.Topic,
			})

			if err != nil {
				slog.Error("[kafka] failed to invoke callback for message", slog.Any("error", err))
			}

			if params.AutoACK {
				if err == nil || !params.IsRedeliverForCBFailed {
					_, err = consumer.Commit()
					if err != nil {
						slog.Error("[kafka] failed to commit message", slog.Any("topics", params.Topics), slog.Any("error", err))
					}
				}
			}
		case kafka.Error:
			slog.Error("[kafka] consumer error", slog.Any("error", e))
		}
	}

}

func (k *Kafka) PurgeStream(streamName string) error {
	return fmt.Errorf("PurgeStream is not supported for Kafka message queue")
}

func (k *Kafka) DeleteMessagesByFilter(streamName string, filter func(data []byte) bool) error {
	return fmt.Errorf("DeleteMessagesByFilter is not supported for Kafka message queue")
}
