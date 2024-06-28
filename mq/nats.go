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

var (
	nats_connect_timeout time.Duration = 2 * time.Second  // second
	nats_reconnect_wait  time.Duration = 10 * time.Second // second

	feeEvtStreamName    string = "accountingEventStream"           // to receive fee request
	feevEvtConsumerName string = "accountingServerDurableConsumer" // durable consumer name

	notifyStreamName   string = "accountingNotifyStream" // to publish notify message
	notifyConsumerName string = "accountingNotifyDurableConsumer"

	dlqStreamName  string = "accountingDlqStream"
	dlqSubjectName string = "accounting.dlq.fee"
)

type NatsHandler struct {
	conn *nats.Conn

	feeRequestSubject      string // subject name for fee request
	notifyNoBalanceSubject string // subject name for pub notification of no balance
	msgFetchTimeoutInSec   int

	feeEvtCfg         jetstream.StreamConfig
	feeConsumerCfg    jetstream.ConsumerConfig
	notifyEvtCfg      jetstream.StreamConfig
	notifyConsumerCfg jetstream.ConsumerConfig
	dlqEvtCfg         jetstream.StreamConfig

	js  jetstream.JetStream
	jss jetstream.Stream
	jsc jetstream.Consumer
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
	return &NatsHandler{
		conn: nc,
		feeEvtCfg: jetstream.StreamConfig{
			Name:         feeEvtStreamName,
			Subjects:     []string{config.Nats.FeeRequestSubject},
			MaxConsumers: -1,
			MaxMsgs:      -1,
			MaxBytes:     -1,
		},
		feeConsumerCfg: jetstream.ConsumerConfig{
			Durable:       feevEvtConsumerName,
			AckPolicy:     jetstream.AckExplicitPolicy,
			DeliverPolicy: jetstream.DeliverAllPolicy,
			FilterSubject: config.Nats.FeeRequestSubject,
		},
		feeRequestSubject: config.Nats.FeeRequestSubject,
		notifyEvtCfg: jetstream.StreamConfig{
			Name:         notifyStreamName,
			Subjects:     []string{config.Nats.FeeNotifyNoBalanceSubject},
			MaxConsumers: -1,
			MaxMsgs:      -1,
			MaxBytes:     -1,
		},
		notifyConsumerCfg: jetstream.ConsumerConfig{
			Durable:       notifyConsumerName,
			AckPolicy:     jetstream.AckExplicitPolicy,
			DeliverPolicy: jetstream.DeliverAllPolicy,
			FilterSubject: config.Nats.FeeRequestSubject,
		},
		notifyNoBalanceSubject: config.Nats.FeeNotifyNoBalanceSubject,
		dlqEvtCfg: jetstream.StreamConfig{
			Name:         dlqStreamName,
			Subjects:     []string{dlqSubjectName},
			MaxConsumers: -1,
			MaxMsgs:      -1,
			MaxBytes:     -1,
		},
		msgFetchTimeoutInSec: config.Nats.MsgFetchTimeoutInSEC,
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

func (nh *NatsHandler) CreateOrUpdateStream(ctx context.Context, streamName string, streamCfg jetstream.StreamConfig) error {
	err := nh.VerifyStreamByName(streamName)
	if err != nil && !errors.Is(err, jetstream.ErrStreamNotFound) {
		return err
	}
	jss, err := nh.js.CreateOrUpdateStream(ctx, streamCfg)
	if err != nil {
		return err
	}
	nh.jss = jss
	return err
}

func (nh *NatsHandler) BuildEventStream() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err := nh.GetJetStream()
	if err != nil {
		return err
	}

	err = nh.CreateOrUpdateStream(ctx, feeEvtStreamName, nh.feeEvtCfg)
	if err != nil {
		return err
	}

	jsc, err := nh.jss.CreateOrUpdateConsumer(ctx, nh.feeConsumerCfg)
	if err != nil {
		return err
	}
	nh.jsc = jsc
	return nil
}

func (nh *NatsHandler) BuildNotifyStream() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	err := nh.GetJetStream()
	if err != nil {
		return err
	}
	return nh.CreateOrUpdateStream(ctx, notifyStreamName, nh.notifyEvtCfg)
}

func (nh *NatsHandler) BuildDLQStream() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	err := nh.GetJetStream()
	if err != nil {
		return err
	}
	return nh.CreateOrUpdateStream(ctx, dlqStreamName, nh.dlqEvtCfg)
}

func (nh *NatsHandler) FetchFeeEventMessages(batch int) (jetstream.MessageBatch, error) {
	msgs, err := nh.jsc.Fetch(batch, jetstream.FetchMaxWait(time.Duration(nh.msgFetchTimeoutInSec)*time.Second))
	return msgs, err
}

func (nh *NatsHandler) VerifyStreamByName(streamName string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_, err := nh.js.Stream(ctx, streamName)
	return err
}

func (nh *NatsHandler) VerifyEventStream() error {
	return nh.VerifyStreamByName(feeEvtStreamName)
}

func (nh *NatsHandler) VerifyNotifyStream() error {
	return nh.VerifyStreamByName(notifyStreamName)
}

func (nh *NatsHandler) VerifyDLQStream() error {
	return nh.VerifyStreamByName(dlqStreamName)
}

func (nh *NatsHandler) PublishData(subject string, data []byte) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_, err := nh.js.Publish(ctx, subject, data)
	return err
}

func (nh *NatsHandler) PublishNotificationForNoBalance(data []byte) error {
	return nh.PublishData(nh.notifyNoBalanceSubject, data)
}

func (nh *NatsHandler) PublishDataToDLQ(data []byte) error {
	return nh.PublishData(dlqSubjectName, data)
}

func (nh *NatsHandler) BuildNotifyConsumer(consumerName string) (jetstream.Consumer, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	consumer, err := nh.js.CreateOrUpdateConsumer(ctx, notifyStreamName,
		jetstream.ConsumerConfig{
			Name:          consumerName,
			Durable:       consumerName,
			AckPolicy:     jetstream.AckExplicitPolicy,
			FilterSubject: nh.notifyNoBalanceSubject,
		},
	)
	return consumer, err
}
