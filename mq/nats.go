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

type DLQEventConfig struct {
	EventConfig
	FeeSubjectName      string // subject of fee dlq
	MeterSubjectName    string // subject of meter dlq
	RechargeSubjectName string // subject of recharge dlq
}

type RequestSubject struct {
	fee             string // fee charging subject
	token           string // token subject
	quota           string // quota subject
	subscription    string // subscription subject
	orderExpired    string // subject name for pub notification of order expired
	rechargeSucceed string //
	subChange       string // subject name for subscription change
}

var (
	nats_connect_timeout time.Duration = 2 * time.Second  // second
	nats_reconnect_wait  time.Duration = 10 * time.Second // second

	feeCfg = EventConfig{
		StreamName:   bldmq.AccountingEventGroup.StreamName, // fee request
		ConsumerName: bldmq.AccountingEventGroup.ConsumerName,
	}

	dlqCfg = EventConfig{
		StreamName:   bldmq.AccountingDlqGroup.StreamName, // DLQ
		ConsumerName: bldmq.AccountingDlqGroup.ConsumerName,
	}

	dlq = DLQEventConfig{
		EventConfig:         dlqCfg,
		FeeSubjectName:      bldmq.DLQFeeSubject,
		MeterSubjectName:    bldmq.DLQMeterSubject,
		RechargeSubjectName: bldmq.DLQRechargeSubject,
	}

	meterCfg = EventConfig{
		StreamName:   bldmq.MeteringEventGroup.StreamName, // metering request
		ConsumerName: bldmq.MeteringEventGroup.ConsumerName,
	}

	orderCfg = EventConfig{
		StreamName:   bldmq.AccountingOrderGroup.StreamName, // order
		ConsumerName: bldmq.AccountingOrderGroup.ConsumerName,
	}

	rechargeCfg = EventConfig{
		StreamName:   bldmq.RechargeGroup.StreamName,
		ConsumerName: bldmq.RechargeGroup.ConsumerName,
	}

	highPriorityMsgCfg = EventConfig{
		StreamName:   bldmq.HighPriorityMsgGroup.StreamName,
		ConsumerName: bldmq.HighPriorityMsgGroup.ConsumerName,
	}

	normalPriorityMsgCfg = EventConfig{
		StreamName:   bldmq.NormalPriorityMsgGroup.StreamName,
		ConsumerName: bldmq.NormalPriorityMsgGroup.ConsumerName,
	}
)

type NatsHandler struct {
	conn *nats.Conn

	msgFetchTimeoutInSec int
	feeReqSub            RequestSubject
	orderReqSub          RequestSubject
	rechargeReqSub       RequestSubject
	feeEvtCfg            jetstream.StreamConfig
	feeConsumerCfg       jetstream.ConsumerConfig
	dlqEvtCfg            jetstream.StreamConfig
	meterEvtCfg          jetstream.StreamConfig
	meterConsumerCfg     jetstream.ConsumerConfig
	orderEvtCfg          jetstream.StreamConfig
	orderConsumerCfg     jetstream.ConsumerConfig
	rechargeEvtCfg       jetstream.StreamConfig
	rechargeConsumerCfg  jetstream.ConsumerConfig

	js                   jetstream.JetStream
	feeJsc               jetstream.Consumer
	meterJsc             jetstream.Consumer
	orderJsc             jetstream.Consumer
	rechargeJsc          jetstream.Consumer
	highPriorityMsgJsc   jetstream.Consumer
	normalPriorityMsgJsc jetstream.Consumer

	highPriorityMsgEvtCfg        jetstream.StreamConfig
	highPriorityMsgConsumerCfg   jetstream.ConsumerConfig
	normalPriorityMsgEvtCfg      jetstream.StreamConfig
	normalPriorityMsgConsumerCfg jetstream.ConsumerConfig
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
	feeEC, feeCC := initStreamAndConsumerConfig(feeCfg,
		[]string{bldmq.FeeRequestSubject})
	dlqEC, _ := initStreamAndConsumerConfig(dlqCfg,
		[]string{dlq.FeeSubjectName, dlq.MeterSubjectName, dlq.RechargeSubjectName})
	meterEC, meterCC := initStreamAndConsumerConfig(meterCfg,
		[]string{bldmq.MeterRequestSubject})
	orderEC, orderCC := initStreamAndConsumerConfig(orderCfg,
		[]string{bldmq.OrderExpiredSubject})
	rechargeEC, rechargeCC := initStreamAndConsumerConfig(rechargeCfg,
		[]string{bldmq.RechargeSucceedSubject})
	highPriorityMsgEvtCfg, highPriorityMsgConsumerCfg := initStreamAndConsumerConfig(highPriorityMsgCfg,
		[]string{bldmq.HighPriorityMsgSubject})
	normalPriorityMsgEvtCfg, normalPriorityMsgConsumerCfg := initStreamAndConsumerConfig(normalPriorityMsgCfg,
		[]string{bldmq.NormalPriorityMsgSubject})

	return &NatsHandler{
		conn:                 nc,
		msgFetchTimeoutInSec: config.Nats.MsgFetchTimeoutInSEC,
		feeReqSub: RequestSubject{
			fee:          bldmq.FeeSendSubject,
			token:        bldmq.TokenSendSubject,
			quota:        bldmq.QuotaSendSubject,
			subscription: bldmq.SubscriptionSendSubject,
			subChange:    bldmq.NotifySubChangeSubject,
		},
		orderReqSub: RequestSubject{
			orderExpired: bldmq.OrderExpiredSubject,
		},
		rechargeReqSub: RequestSubject{
			rechargeSucceed: bldmq.RechargeSucceedSubject,
		},
		feeEvtCfg:                    feeEC,
		feeConsumerCfg:               feeCC,
		dlqEvtCfg:                    dlqEC,
		meterEvtCfg:                  meterEC,
		meterConsumerCfg:             meterCC,
		orderEvtCfg:                  orderEC,
		orderConsumerCfg:             orderCC,
		rechargeEvtCfg:               rechargeEC,
		rechargeConsumerCfg:          rechargeCC,
		highPriorityMsgEvtCfg:        highPriorityMsgEvtCfg,
		highPriorityMsgConsumerCfg:   highPriorityMsgConsumerCfg,
		normalPriorityMsgEvtCfg:      normalPriorityMsgEvtCfg,
		normalPriorityMsgConsumerCfg: normalPriorityMsgConsumerCfg,
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

func (nh *NatsHandler) BuildFeeEventStream() error {
	jsc, err := nh.BuildEventStreamAndConsumer(feeCfg, nh.feeEvtCfg, nh.feeConsumerCfg)
	if err != nil {
		return err
	}
	nh.feeJsc = jsc
	return nil
}

func (nh *NatsHandler) BuildMeterEventStream() error {
	jsc, err := nh.BuildEventStreamAndConsumer(meterCfg, nh.meterEvtCfg, nh.meterConsumerCfg)
	if err != nil {
		return err
	}
	nh.meterJsc = jsc
	return nil
}

func (nh *NatsHandler) BuildRechargeEventStream() error {
	jsc, err := nh.BuildEventStreamAndConsumer(rechargeCfg, nh.rechargeEvtCfg, nh.rechargeConsumerCfg)
	if err != nil {
		return err
	}
	nh.rechargeJsc = jsc
	return nil
}

func (nh *NatsHandler) BuildOrderEventStream() error {
	jsc, err := nh.BuildEventStreamAndConsumer(orderCfg, nh.orderEvtCfg, nh.orderConsumerCfg)
	if err != nil {
		return err
	}
	nh.orderJsc = jsc
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

func (nh *NatsHandler) FetchFeeEventMessages(batch int) (jetstream.MessageBatch, error) {
	msgs, err := nh.feeJsc.Fetch(batch, jetstream.FetchMaxWait(time.Duration(nh.msgFetchTimeoutInSec)*time.Second))
	return msgs, err
}

func (nh *NatsHandler) FetchMeterEventMessages(batch int) (jetstream.MessageBatch, error) {
	msgs, err := nh.meterJsc.Fetch(batch, jetstream.FetchMaxWait(time.Duration(nh.msgFetchTimeoutInSec)*time.Second))
	return msgs, err
}

func (nh *NatsHandler) FetchRechargeEventMessages(batch int) (jetstream.MessageBatch, error) {
	msgs, err := nh.rechargeJsc.Fetch(batch, jetstream.FetchMaxWait(time.Duration(nh.msgFetchTimeoutInSec)*time.Second))
	return msgs, err
}

func (nh *NatsHandler) VerifyStreamByName(streamName string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_, err := nh.js.Stream(ctx, streamName)
	return err
}

func (nh *NatsHandler) VerifyFeeEventStream() error {
	return nh.VerifyStreamByName(feeCfg.StreamName)
}

func (nh *NatsHandler) VerifyMeteringStream() error {
	return nh.VerifyStreamByName(meterCfg.StreamName)
}

func (nh *NatsHandler) VerifyRechargeStream() error {
	return nh.VerifyStreamByName(rechargeCfg.StreamName)
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

func (nh *NatsHandler) PublishNotificationForSubscription(data []byte) error {
	return nh.PublishData(nh.feeReqSub.subChange, data)
}

func (nh *NatsHandler) PublishFeeCreditData(data []byte) error {
	return nh.PublishData(nh.feeReqSub.fee, data)
}

func (nh *NatsHandler) PublishFeeTokenData(data []byte) error {
	return nh.PublishData(nh.feeReqSub.token, data)
}

func (nh *NatsHandler) PublishFeeQuotaData(data []byte) error {
	return nh.PublishData(nh.feeReqSub.quota, data)
}

func (nh *NatsHandler) PublishFeeDataToDLQ(data []byte) error {
	return nh.PublishData(dlq.FeeSubjectName, data)
}

func (nh *NatsHandler) PublishMeterDataToDLQ(data []byte) error {
	return nh.PublishData(dlq.MeterSubjectName, data)
}

func (nh *NatsHandler) PublishRechargeDataToDLQ(data []byte) error {
	return nh.PublishData(dlq.RechargeSubjectName, data)
}

func (nh *NatsHandler) PublishRechargeDurationData(data []byte) error {
	return nh.PublishData(nh.rechargeReqSub.rechargeSucceed, data)
}

func (nh *NatsHandler) PublishOrderExpiredData(data []byte) error {
	return nh.PublishData(nh.orderReqSub.orderExpired, data)
}

func (nh *NatsHandler) PublishSubscriptionData(data []byte) error {
	return nh.PublishData(nh.feeReqSub.subscription, data)
}

func (nh *NatsHandler) BuildOrderConsumerWithName(consumerName string) (jetstream.Consumer, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	ec := EventConfig{StreamName: orderCfg.StreamName, ConsumerName: consumerName}
	_, conCfg := initStreamAndConsumerConfig(ec, []string{nh.orderReqSub.orderExpired})
	consumer, err := nh.js.CreateOrUpdateConsumer(ctx, orderCfg.StreamName, conCfg)
	return consumer, err
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
