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
	VerifyStreamByName(streamName string) error
	PublishData(subject string, data []byte) error

	PublishHighPriorityMsg(msg types.ScenarioMessage) error
	BuildHighPriorityMsgStream(conf *config.Config) error
	BuildHighPriorityMsgConsumer() (jetstream.Consumer, error)

	PublishNormalPriorityMsg(msg types.ScenarioMessage) error
	BuildNormalPriorityMsgStream(conf *config.Config) error
	BuildNormalPriorityMsgConsumer() (jetstream.Consumer, error)

	PublishAgentSessionHistoryMsg(msg types.SessionHistoryMessageEnvelope) error
	BuildAgentSessionHistoryMsgStream(conf *config.Config) error
	BuildAgentSessionHistoryMsgConsumer() (jetstream.Consumer, error)
}
