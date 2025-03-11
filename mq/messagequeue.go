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
	BuildMeterEventStream() error
	BuildDLQStream() error
	FetchMeterEventMessages(batch int) (jetstream.MessageBatch, error)
	VerifyStreamByName(streamName string) error
	VerifyMeteringStream() error
	VerifyDLQStream() error
	VerifyDeployServiceStream() error
	PublishData(subject string, data []byte) error
	PublishMeterDataToDLQ(data []byte) error
	PublishMeterDurationData(data []byte) error
	PublishFeeCreditData(data []byte) error
	PublishFeeTokenData(data []byte) error
	PublishFeeQuotaData(data []byte) error
	PublishDeployServiceData(data []byte) error
	BuildDeployServiceConsumerWithName(consumerName string) (jetstream.Consumer, error)
}
