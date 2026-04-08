package mq

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	bldmq "opencsg.com/csghub-server/builder/mq"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
)

var _ MessageQueue = (*NatsHandler)(nil)

type EventConfig struct {
	StreamName   string // stream name of event
	ConsumerName string // durable consumer name
}

var (
	nats_connect_timeout time.Duration = 2 * time.Second  // second
	nats_reconnect_wait  time.Duration = 10 * time.Second // second

	highPriorityMsgCfg = EventConfig{
		StreamName:   bldmq.HighPriorityMsgGroup.StreamName,
		ConsumerName: bldmq.HighPriorityMsgGroup.ConsumerName,
	}

	normalPriorityMsgCfg = EventConfig{
		StreamName:   bldmq.NormalPriorityMsgGroup.StreamName,
		ConsumerName: bldmq.NormalPriorityMsgGroup.ConsumerName,
	}

	agentSessionHistoryMsgCfg = EventConfig{
		StreamName:   bldmq.AgentSessionHistoryMsgGroup.StreamName,
		ConsumerName: bldmq.AgentSessionHistoryMsgGroup.ConsumerName,
	}
)

type NatsHandler struct {
	conn                      *nats.Conn
	msgFetchTimeoutInSec      int
	js                        jetstream.JetStream
	highPriorityMsgJsc        jetstream.Consumer
	normalPriorityMsgJsc      jetstream.Consumer
	agentSessionHistoryMsgJsc jetstream.Consumer

	highPriorityMsgEvtCfg             jetstream.StreamConfig
	highPriorityMsgConsumerCfg        jetstream.ConsumerConfig
	normalPriorityMsgEvtCfg           jetstream.StreamConfig
	normalPriorityMsgConsumerCfg      jetstream.ConsumerConfig
	agentSessionHistoryMsgEvtCfg      jetstream.StreamConfig
	agentSessionHistoryMsgConsumerCfg jetstream.ConsumerConfig
}

func initStreamAndConsumerConfig(cfg EventConfig, subjectNames []string) (jetstream.StreamConfig, jetstream.ConsumerConfig) {
	return jetstream.StreamConfig{
			Name: cfg.StreamName, Subjects: subjectNames,
			MaxConsumers: -1, MaxMsgs: -1, MaxBytes: -1,
		},
		jetstream.ConsumerConfig{
			Name: cfg.ConsumerName, Durable: cfg.ConsumerName, FilterSubject: subjectNames[0],
			AckPolicy: jetstream.AckExplicitPolicy, DeliverPolicy: jetstream.DeliverAllPolicy,
		}
}

func NewNats(config *config.Config) (*NatsHandler, error) {
	nc, err := nats.Connect(
		config.Nats.URL,
		nats.Timeout(nats_connect_timeout),
		nats.ReconnectWait(nats_reconnect_wait),
		nats.MaxReconnects(-1),
	)
	if err != nil {
		return nil, err
	}
	highPriorityMsgEvtCfg, highPriorityMsgConsumerCfg := initStreamAndConsumerConfig(highPriorityMsgCfg,
		[]string{bldmq.HighPriorityMsgSubject})
	normalPriorityMsgEvtCfg, normalPriorityMsgConsumerCfg := initStreamAndConsumerConfig(normalPriorityMsgCfg,
		[]string{bldmq.NormalPriorityMsgSubject})
	agentSessionHistoryMsgEvtCfg, agentSessionHistoryMsgConsumerCfg := initStreamAndConsumerConfig(agentSessionHistoryMsgCfg,
		[]string{bldmq.AgentSessionHistoryMsgSubject})
	return &NatsHandler{
		conn:                              nc,
		msgFetchTimeoutInSec:              config.Nats.MsgFetchTimeoutInSEC,
		highPriorityMsgEvtCfg:             highPriorityMsgEvtCfg,
		highPriorityMsgConsumerCfg:        highPriorityMsgConsumerCfg,
		normalPriorityMsgEvtCfg:           normalPriorityMsgEvtCfg,
		normalPriorityMsgConsumerCfg:      normalPriorityMsgConsumerCfg,
		agentSessionHistoryMsgEvtCfg:      agentSessionHistoryMsgEvtCfg,
		agentSessionHistoryMsgConsumerCfg: agentSessionHistoryMsgConsumerCfg,
	}, nil
}

func (nh *NatsHandler) GetConn() *nats.Conn {
	return nh.conn
}

func (nh *NatsHandler) GetJetStream() error {
	js, err := jetstream.New(nh.conn)
	if err != nil {
		return err
	}
	nh.js = js
	return nil
}

func (nh *NatsHandler) CreateOrUpdateStream(ctx context.Context, streamName string, streamCfg jetstream.StreamConfig) (jetstream.Stream, error) {
	err := nh.VerifyStreamByName(streamName)
	if err != nil && !errors.Is(err, jetstream.ErrStreamNotFound) {
		return nil, err
	}
	jss, err := nh.js.CreateOrUpdateStream(ctx, streamCfg)
	if err != nil {
		return nil, err
	}
	return jss, err
}

func (nh *NatsHandler) BuildEventStreamAndConsumer(cfg EventConfig, streamCfg jetstream.StreamConfig, consumerCfg jetstream.ConsumerConfig) (jetstream.Consumer, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err := nh.GetJetStream()
	if err != nil {
		return nil, err
	}

	jss, err := nh.CreateOrUpdateStream(ctx, cfg.StreamName, streamCfg)
	if err != nil {
		return nil, err
	}

	jsc, err := jss.CreateOrUpdateConsumer(ctx, consumerCfg)
	if err != nil {
		return nil, err
	}
	return jsc, nil
}

func (nh *NatsHandler) VerifyStreamByName(streamName string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_, err := nh.js.Stream(ctx, streamName)
	return err
}

func (nh *NatsHandler) PublishData(subject string, data []byte) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_, err := nh.js.Publish(ctx, subject, data)
	return err
}

func (nh *NatsHandler) BuildHighPriorityMsgStream(conf *config.Config) error {
	nh.highPriorityMsgConsumerCfg.AckWait = time.Duration(conf.Notification.HighPriorityMsgAckWait) * time.Second
	nh.highPriorityMsgConsumerCfg.MaxDeliver = conf.Notification.HighPriorityMsgMaxDeliver
	jsc, err := nh.BuildEventStreamAndConsumer(highPriorityMsgCfg, nh.highPriorityMsgEvtCfg, nh.highPriorityMsgConsumerCfg)
	if err != nil {
		return err
	}
	nh.highPriorityMsgJsc = jsc
	return nil
}

func (nh *NatsHandler) BuildNormalPriorityMsgStream(conf *config.Config) error {
	nh.normalPriorityMsgConsumerCfg.AckWait = time.Duration(conf.Notification.NormalPriorityMsgAckWait) * time.Second
	nh.normalPriorityMsgConsumerCfg.MaxDeliver = conf.Notification.NormalPriorityMsgMaxDeliver
	jsc, err := nh.BuildEventStreamAndConsumer(normalPriorityMsgCfg, nh.normalPriorityMsgEvtCfg, nh.normalPriorityMsgConsumerCfg)
	if err != nil {
		return err
	}
	nh.normalPriorityMsgJsc = jsc
	return nil
}

func (nh *NatsHandler) BuildAgentSessionHistoryMsgStream(conf *config.Config) error {
	nh.agentSessionHistoryMsgConsumerCfg.MaxDeliver = 3
	jsc, err := nh.BuildEventStreamAndConsumer(agentSessionHistoryMsgCfg, nh.agentSessionHistoryMsgEvtCfg, nh.agentSessionHistoryMsgConsumerCfg)
	if err != nil {
		return err
	}
	nh.agentSessionHistoryMsgJsc = jsc
	return nil
}

func (nh *NatsHandler) BuildHighPriorityMsgConsumer() (jetstream.Consumer, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	consumer, err := nh.js.CreateOrUpdateConsumer(ctx, highPriorityMsgCfg.StreamName, nh.highPriorityMsgConsumerCfg)
	return consumer, err
}

func (nh *NatsHandler) BuildNormalPriorityMsgConsumer() (jetstream.Consumer, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	consumer, err := nh.js.CreateOrUpdateConsumer(ctx, normalPriorityMsgCfg.StreamName, nh.normalPriorityMsgConsumerCfg)
	return consumer, err
}

func (nh *NatsHandler) PublishHighPriorityMsg(msg types.ScenarioMessage) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	return nh.PublishData(nh.highPriorityMsgEvtCfg.Subjects[0], data)
}

func (nh *NatsHandler) PublishNormalPriorityMsg(msg types.ScenarioMessage) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	return nh.PublishData(nh.normalPriorityMsgEvtCfg.Subjects[0], data)
}

func (nh *NatsHandler) BuildAgentSessionHistoryMsgConsumer() (jetstream.Consumer, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	consumer, err := nh.js.CreateOrUpdateConsumer(ctx, agentSessionHistoryMsgCfg.StreamName, nh.agentSessionHistoryMsgConsumerCfg)
	return consumer, err
}

func (nh *NatsHandler) PublishAgentSessionHistoryMsg(msg types.SessionHistoryMessageEnvelope) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	return nh.PublishData(nh.agentSessionHistoryMsgEvtCfg.Subjects[0], data)
}
