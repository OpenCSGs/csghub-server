package mq

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
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
	MeterSubjectName string // subject of meter dlq
}

type RequestSubject struct {
	fee           string // fee charging subject
	token         string // token subject
	quota         string // quota subject
	duration      string // duration subject
	deployService string // service update subject
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
	svcCfg = EventConfig{
		StreamName:   "deployServiceUpdateStream", // order
		ConsumerName: "deployServiceUpdateConsumer",
	}
	siteInternalMsgCfg = EventConfig{
		StreamName:   "siteInternalMsgStream",
		ConsumerName: "siteInternalMsgConsumer",
	}

	siteInternalMailCfg = EventConfig{
		StreamName:   "siteInternalMailStream",
		ConsumerName: "siteInternalMailConsumer",
	}

	highPriorityMsgCfg = EventConfig{
		StreamName:   "highPriorityMsgStream",
		ConsumerName: "highPriorityMsgConsumer",
	}

	normalPriorityMsgCfg = EventConfig{
		StreamName:   "normalPriorityMsgStream",
		ConsumerName: "normalPriorityMsgConsumer",
	}
)

type NatsHandler struct {
	conn *nats.Conn

	msgFetchTimeoutInSec int
	feeReqSub            RequestSubject
	meterReqSub          RequestSubject
	serviceReqSub        RequestSubject
	dlqEvtCfg            jetstream.StreamConfig
	svcEvtCfg            jetstream.StreamConfig
	meterEvtCfg          jetstream.StreamConfig
	meterConsumerCfg     jetstream.ConsumerConfig

	js                   jetstream.JetStream
	meterJsc             jetstream.Consumer
	siteInternalMsgJsc   jetstream.Consumer
	highPriorityMsgJsc   jetstream.Consumer
	normalPriorityMsgJsc jetstream.Consumer

	siteInternalMsgEvtCfg       jetstream.StreamConfig
	siteInternalMsgConsumerCfg  jetstream.ConsumerConfig
	siteInternalMailEvtCfg      jetstream.StreamConfig
	siteInternalMailConsumerCfg jetstream.ConsumerConfig

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

	dlqEC, _ := initStreamAndConsumerConfig(dlqCfg, []string{dlq.MeterSubjectName})
	meterEC, meterCC := initStreamAndConsumerConfig(meterCfg, []string{config.Nats.MeterRequestSubject})
	svcEC, _ := initStreamAndConsumerConfig(svcCfg, []string{config.Nats.ServiceUpdateSubject})

	siteInternalMsgEvtCfg, siteInternalMsgConsumerCfg := initStreamAndConsumerConfig(siteInternalMsgCfg, []string{config.Nats.SiteInternalMsgSubject})
	siteInternalMailEvtCfg, siteInternalMailConsumerCfg := initStreamAndConsumerConfig(siteInternalMailCfg, []string{config.Nats.SiteInternalMailSubject})
	highPriorityMsgEvtCfg, highPriorityMsgConsumerCfg := initStreamAndConsumerConfig(highPriorityMsgCfg, []string{config.Nats.HighPriorityMsgSubject})
	normalPriorityMsgEvtCfg, normalPriorityMsgConsumerCfg := initStreamAndConsumerConfig(normalPriorityMsgCfg, []string{config.Nats.NormalPriorityMsgSubject})

	return &NatsHandler{
		conn:                 nc,
		msgFetchTimeoutInSec: config.Nats.MsgFetchTimeoutInSEC,
		meterReqSub: RequestSubject{
			duration: config.Nats.MeterDurationSendSubject,
			token:    config.Nats.MeterTokenSendSubject,
			quota:    config.Nats.MeterQuotaSendSubject,
		},
		serviceReqSub: RequestSubject{
			deployService: config.Nats.ServiceUpdateSubject,
		},
		dlqEvtCfg:                    dlqEC,
		meterEvtCfg:                  meterEC,
		meterConsumerCfg:             meterCC,
		svcEvtCfg:                    svcEC,
		siteInternalMsgEvtCfg:        siteInternalMsgEvtCfg,
		siteInternalMsgConsumerCfg:   siteInternalMsgConsumerCfg,
		siteInternalMailEvtCfg:       siteInternalMailEvtCfg,
		siteInternalMailConsumerCfg:  siteInternalMailConsumerCfg,
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

func (nh *NatsHandler) PublishFeeCreditData(data []byte) error {
	return nh.PublishData(nh.feeReqSub.fee, data)
}

func (nh *NatsHandler) PublishFeeTokenData(data []byte) error {
	return nh.PublishData(nh.feeReqSub.token, data)
}

func (nh *NatsHandler) PublishFeeQuotaData(data []byte) error {
	return nh.PublishData(nh.feeReqSub.quota, data)
}

func (nh *NatsHandler) BuildDeployServiceStream() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	err := nh.GetJetStream()
	if err != nil {
		return err
	}
	_, err = nh.CreateOrUpdateStream(ctx, svcCfg.StreamName, nh.svcEvtCfg)
	return err
}

func (nh *NatsHandler) VerifyDeployServiceStream() error {
	return nh.VerifyStreamByName(svcCfg.StreamName)
}
func (nh *NatsHandler) PublishDeployServiceData(data []byte) error {
	return nh.PublishData(nh.serviceReqSub.deployService, data)
}

func (nh *NatsHandler) BuildDeployServiceConsumerWithName(consumerName string) (jetstream.Consumer, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	ec := EventConfig{StreamName: svcCfg.StreamName, ConsumerName: consumerName}
	_, conCfg := initStreamAndConsumerConfig(ec, []string{nh.serviceReqSub.deployService})
	consumer, err := nh.js.CreateOrUpdateConsumer(ctx, svcCfg.StreamName, conCfg)
	return consumer, err
}

func (nh *NatsHandler) PublishSiteInternalMsg(msg types.NotificationMessage) error {
	if msg.MsgUUID == "" {
		return errors.New("msg uuid is empty")
	}
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	err = nh.PublishData(nh.siteInternalMsgEvtCfg.Subjects[0], data)
	return err
}

func (nh *NatsHandler) BuildSiteInternalMsgStream() error {
	jsc, err := nh.BuildEventStreamAndConsumer(siteInternalMsgCfg, nh.siteInternalMsgEvtCfg, nh.siteInternalMsgConsumerCfg)
	if err != nil {
		return err
	}
	nh.siteInternalMsgJsc = jsc
	return nil
}

func (nh *NatsHandler) BuildSiteInternalMsgConsumer() (jetstream.Consumer, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	consumer, err := nh.js.CreateOrUpdateConsumer(ctx, siteInternalMsgCfg.StreamName, nh.siteInternalMsgConsumerCfg)
	return consumer, err
}

func (nh *NatsHandler) PublishSiteInternalMail(msg types.MailMessage) error {
	if msg.MsgUUID == "" {
		return errors.New("msg uuid is empty")
	}
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	err = nh.PublishData(nh.siteInternalMailEvtCfg.Subjects[0], data)
	return err
}

func (nh *NatsHandler) BuildSiteInternalMailStream() error {
	jsc, err := nh.BuildEventStreamAndConsumer(siteInternalMailCfg, nh.siteInternalMailEvtCfg, nh.siteInternalMailConsumerCfg)
	if err != nil {
		return err
	}
	nh.siteInternalMsgJsc = jsc
	return nil
}

func (nh *NatsHandler) BuildSiteInternalMailConsumer() (jetstream.Consumer, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	consumer, err := nh.js.CreateOrUpdateConsumer(ctx, siteInternalMailCfg.StreamName, nh.siteInternalMailConsumerCfg)
	return consumer, err
}

func (nh *NatsHandler) BuildHighPriorityMsgStream() error {
	jsc, err := nh.BuildEventStreamAndConsumer(highPriorityMsgCfg, nh.highPriorityMsgEvtCfg, nh.highPriorityMsgConsumerCfg)
	if err != nil {
		return err
	}
	nh.highPriorityMsgJsc = jsc
	return nil
}

func (nh *NatsHandler) BuildNormalPriorityMsgStream() error {
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
