package mq

import (
	"context"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

type MessageQueue interface {
	GetConn() *nats.Conn
	GetJetStream() error
	CreateOrUpdateStream(ctx context.Context, streamName string, streamCfg jetstream.StreamConfig) (jetstream.Stream, error)
	BuildEventStreamAndConsumer(cfg EventConfig, streamCfg jetstream.StreamConfig, consumerCfg jetstream.ConsumerConfig) (jetstream.Consumer, error)
	BuildFeeEventStream() error
	BuildMeterEventStream() error
	BuildNotifyStream() error
	BuildDLQStream() error
	FetchFeeEventMessages(batch int) (jetstream.MessageBatch, error)
	VerifyStreamByName(streamName string) error
	VerifyFeeEventStream() error
	VerifyMeteringStream() error
	VerifyNotifyStream() error
	VerifyDLQStream() error
	PublishData(subject string, data []byte) error
	PublishNotificationForNoBalance(data []byte) error
	PublishFeeCreditData(data []byte) error
	PublishFeeTokenData(data []byte) error
	PublishFeeQuotaData(data []byte) error
	PublishFeeDataToDLQ(data []byte) error
	PublishMeterDataToDLQ(data []byte) error
	PublishMeterDurationData(data []byte) error
	BuildNotifyConsumerWithName(consumerName string) (jetstream.Consumer, error)
}
