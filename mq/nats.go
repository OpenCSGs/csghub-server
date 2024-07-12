package mq

import (
	"context"
	"errors"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"opencsg.com/csghub-server/common/config"
)

var _ MessageQueue = (*NatsHandler)(nil)

type EventConfig struct {
	StreamName   string // stream name of event
	ConsumerName string // durable consumer name
}

type DLQEventConfig struct {
	EventConfig
	MeterSubjectName string // subject of meter dlq
}

type RequestSubject struct {
	token    string // token subject
	quota    string // quota subject
	duration string // duration subject
}

var (
	nats_connect_timeout time.Duration = 2 * time.Second  // second
	nats_reconnect_wait  time.Duration = 10 * time.Second // second

	dlqCfg EventConfig = EventConfig{
		StreamName:   "accountingDlqStream", // DLQ
		ConsumerName: "accountingDlqDurableConsumer",
	}

	dlq DLQEventConfig = DLQEventConfig{
		EventConfig:      dlqCfg,
		MeterSubjectName: "accounting.dlq.meter",
	}

	meterCfg EventConfig = EventConfig{
		StreamName:   "meteringEventStream", // metering request
		ConsumerName: "metertingServerDurableConsumer",
	}
)

type NatsHandler struct {
	conn *nats.Conn

	msgFetchTimeoutInSec int
	meterReqSub          RequestSubject
	dlqEvtCfg            jetstream.StreamConfig
	meterEvtCfg          jetstream.StreamConfig
	meterConsumerCfg     jetstream.ConsumerConfig

	js       jetstream.JetStream
	meterJsc jetstream.Consumer
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

	dlqEC, _ := initStreamAndConsumerConfig(dlqCfg, []string{dlq.MeterSubjectName})
	meterEC, meterCC := initStreamAndConsumerConfig(meterCfg, []string{config.Nats.MeterRequestSubject})

	return &NatsHandler{
		conn:                 nc,
		msgFetchTimeoutInSec: config.Nats.MsgFetchTimeoutInSEC,
		meterReqSub: RequestSubject{
			duration: config.Nats.MeterDurationSendSubject,
			token:    config.Nats.MeterTokenSendSubject,
			quota:    config.Nats.MeterQuotaSendSubject,
		},
		dlqEvtCfg:        dlqEC,
		meterEvtCfg:      meterEC,
		meterConsumerCfg: meterCC,
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

func (nh *NatsHandler) BuildMeterEventStream() error {
	jsc, err := nh.BuildEventStreamAndConsumer(meterCfg, nh.meterEvtCfg, nh.meterConsumerCfg)
	if err != nil {
		return err
	}
	nh.meterJsc = jsc
	return nil
}

func (nh *NatsHandler) BuildDLQStream() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	err := nh.GetJetStream()
	if err != nil {
		return err
	}
	_, err = nh.CreateOrUpdateStream(ctx, dlqCfg.StreamName, nh.dlqEvtCfg)
	return err
}

func (nh *NatsHandler) FetchMeterEventMessages(batch int) (jetstream.MessageBatch, error) {
	msgs, err := nh.meterJsc.Fetch(batch, jetstream.FetchMaxWait(time.Duration(nh.msgFetchTimeoutInSec)*time.Second))
	return msgs, err
}

func (nh *NatsHandler) VerifyStreamByName(streamName string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_, err := nh.js.Stream(ctx, streamName)
	return err
}

func (nh *NatsHandler) VerifyMeteringStream() error {
	return nh.VerifyStreamByName(meterCfg.StreamName)
}

func (nh *NatsHandler) VerifyDLQStream() error {
	return nh.VerifyStreamByName(dlqCfg.StreamName)
}

func (nh *NatsHandler) PublishData(subject string, data []byte) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_, err := nh.js.Publish(ctx, subject, data)
	return err
}

func (nh *NatsHandler) PublishMeterDataToDLQ(data []byte) error {
	return nh.PublishData(dlq.MeterSubjectName, data)
}

func (nh *NatsHandler) PublishMeterDurationData(data []byte) error {
	return nh.PublishData(nh.meterReqSub.duration, data)
}
