package mq

import (
	"context"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
)

type MessageQueue interface {
	GetConn() *nats.Conn
	GetJetStream() error
	CreateOrUpdateStream(ctx context.Context, streamName string, streamCfg jetstream.StreamConfig) (jetstream.Stream, error)
	BuildEventStreamAndConsumer(cfg EventConfig, streamCfg jetstream.StreamConfig, consumerCfg jetstream.ConsumerConfig) (jetstream.Consumer, error)
	BuildFeeEventStream() error
	BuildMeterEventStream() error
	BuildOrderEventStream() error
	BuildDLQStream() error
	FetchMeterEventMessages(batch int) (jetstream.MessageBatch, error)
	FetchFeeEventMessages(batch int) (jetstream.MessageBatch, error)
	VerifyStreamByName(streamName string) error
	VerifyFeeEventStream() error
	VerifyMeteringStream() error
	VerifyDLQStream() error
	PublishData(subject string, data []byte) error
	PublishNotificationForSubscription(data []byte) error
	PublishFeeCreditData(data []byte) error
	PublishFeeTokenData(data []byte) error
	PublishFeeQuotaData(data []byte) error
	PublishFeeDataToDLQ(data []byte) error
	PublishMeterDataToDLQ(data []byte) error
	PublishOrderExpiredData(data []byte) error
	PublishSubscriptionData(data []byte) error
	BuildOrderConsumerWithName(consumerName string) (jetstream.Consumer, error)
	BuildRechargeEventStream() error
	VerifyRechargeStream() error
	PublishRechargeDurationData(data []byte) error
	FetchRechargeEventMessages(batch int) (jetstream.MessageBatch, error)
	PublishRechargeDataToDLQ(data []byte) error

	PublishHighPriorityMsg(msg types.ScenarioMessage) error
	BuildHighPriorityMsgStream(conf *config.Config) error
	BuildHighPriorityMsgConsumer() (jetstream.Consumer, error)

	PublishNormalPriorityMsg(msg types.ScenarioMessage) error
	BuildNormalPriorityMsgStream(conf *config.Config) error
	BuildNormalPriorityMsgConsumer() (jetstream.Consumer, error)
}
