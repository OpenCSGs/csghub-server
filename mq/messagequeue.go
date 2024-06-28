package mq

import (
	"context"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

type MessageQueue interface {
	GetConn() *nats.Conn
	GetJetStream() error
	CreateOrUpdateStream(ctx context.Context, streamName string, streamCfg jetstream.StreamConfig) error
	BuildEventStream() error
	BuildNotifyStream() error
	BuildDLQStream() error
	FetchFeeEventMessages(batch int) (jetstream.MessageBatch, error)
	VerifyStreamByName(streamName string) error
	VerifyEventStream() error
	VerifyNotifyStream() error
	VerifyDLQStream() error
	PublishData(subject string, data []byte) error
	PublishNotificationForNoBalance(data []byte) error
	PublishDataToDLQ(data []byte) error
	BuildNotifyConsumer(consumerName string) (jetstream.Consumer, error)
}
