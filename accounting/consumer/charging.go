package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/nats-io/nats.go"
	"opencsg.com/csghub-server/accounting/component"
	"opencsg.com/csghub-server/accounting/types"
	"opencsg.com/csghub-server/common/config"
)

type Charging struct {
	NatsURL              string
	FeeRequestSubject    string
	MsgFetchTimeoutInSec int
	acctSMComp           *component.AccountingStatementComponent
	acctUserComp         *component.AccountingUserComponent
	acctEvtComp          *component.AccountingEventComponent
	notify               *Notify
	dlq                  *Dlq
	streamCfg            *nats.StreamConfig
	consumerConfig       *nats.ConsumerConfig
	CH                   chan int
	notifyTimeOut        *time.Timer
	dlqTimeout           *time.Timer
}

var (
	eventStreamName        string = "accountingEventStream"
	accountingConsumerName string = "accountingServerDurableConsumer"
	idleDuration                  = 10 * time.Second
)

func NewCharging(config *config.Config) *Charging {
	charge := &Charging{
		NatsURL:              config.Accounting.NatsURL,
		FeeRequestSubject:    config.Accounting.FeeRequestSubject,
		MsgFetchTimeoutInSec: config.Accounting.MsgFetchTimeoutInSEC,
		acctSMComp:           component.NewAccountingStatement(),
		acctUserComp:         component.NewAccountingUser(),
		acctEvtComp:          component.NewAccountingEvent(),
		notify:               NewNotify(config.Accounting.FeeNotifyNoBalanceSubject),
		dlq:                  NewDlq(),
		streamCfg: &nats.StreamConfig{
			Name:         eventStreamName,
			Subjects:     []string{config.Accounting.FeeRequestSubject},
			MaxConsumers: -1,
			MaxMsgs:      -1,
			MaxBytes:     -1,
		},
		consumerConfig: &nats.ConsumerConfig{
			Durable:       accountingConsumerName,
			AckPolicy:     nats.AckExplicitPolicy,
			DeliverPolicy: nats.DeliverAllPolicy,
			FilterSubject: config.Accounting.FeeRequestSubject,
		},
		CH:            make(chan int),
		notifyTimeOut: time.NewTimer(idleDuration),
		dlqTimeout:    time.NewTimer(idleDuration),
	}
	return charge
}

func (c *Charging) Run() {
	for {
		nc, err := c.buildNatsConn()
		if err != nil {
			slog.Error("fail to connect nats", slog.Any("NatsURL", c.NatsURL), slog.Any("err", err))
			time.Sleep(10 * time.Second)
			continue
		}
		go c.begin(nc)
		<-c.CH
		nc.Close()
		nc = nil
		time.Sleep(2 * idleDuration)
	}
}

func (c *Charging) buildNatsConn() (*nats.Conn, error) {
	nc, err := nats.Connect(c.NatsURL)
	return nc, err
}

func (c *Charging) begin(nc *nats.Conn) {
	go c.notify.Run(nc)
	go c.dlq.Run(nc)
	c.startCharging(nc)
}

func (c *Charging) startCharging(nc *nats.Conn) {
	// try 5 times set jetstream before re-connect nats
	for i := 0; i < 5; i++ {
		js, err := nc.JetStream()
		if err != nil {
			slog.Error("fail to get jetstream", slog.Any("err", err))
			continue
		}

		_, err = js.AddStream(c.streamCfg)
		if err != nil {
			slog.Warn("fail to add nats stream", slog.Any("streamName", eventStreamName), slog.Any("err", err))
			_, err = js.UpdateStream(c.streamCfg)
			if err != nil {
				slog.Warn("fail to update nats stream", slog.Any("streamName", eventStreamName), slog.Any("err", err))
				continue
			}
		}

		_, err = js.AddConsumer(eventStreamName, c.consumerConfig)
		if err != nil {
			slog.Error("fail to add consumer", slog.Any("consumer", eventStreamName), slog.Any("err", err))
			continue
		}

		sub, err := js.PullSubscribe(c.FeeRequestSubject, accountingConsumerName)
		if err != nil {
			slog.Error("fail to PullSubscribe", slog.Any("FeeRequestSubject", c.FeeRequestSubject), slog.Any("consumer", accountingConsumerName), slog.Any("err", err))
			continue
		}
		c.handleReadMsgs(sub)
	}
	c.CH <- 1
}

func (c *Charging) handleReadMsgs(sub *nats.Subscription) {
	failReadTime := 0
	for {
		// reset JetStream if read msg fail times is more than 10
		if failReadTime >= 10 {
			break
		}
		msgs, err := sub.Fetch(5, nats.MaxWait(time.Duration(c.MsgFetchTimeoutInSec)*time.Second))
		if err == nats.ErrTimeout {
			continue
		}
		if err != nil {
			slog.Error("fail to read NextMsg", slog.Any("subjectName", c.FeeRequestSubject), slog.Any("err", err))
			failReadTime++
			continue
		}
		if msgs == nil {
			slog.Warn("msgs is null", slog.Any("subjectName", c.FeeRequestSubject))
			failReadTime++
			continue
		}

		for _, msg := range msgs {
			c.handleRetryMsg(msg)
		}

	}
}

func (c *Charging) handleRetryMsg(msg *nats.Msg) {
	// A maximum of 3 attempts
	var err error = nil
	for j := 0; j < 3; j++ {
		err = c.handleMsgData(msg)
		if err != nil {
			tip := fmt.Sprintf("error happen when handle single msg for the %d time", (j + 1))
			slog.Error(tip, slog.Any("error", err))
		} else {
			break
		}
	}

	if err == nil {
		// handle message success
		err = msg.Ack()
		if err != nil {
			slog.Warn("fail to do msg ack for deal with message success", slog.Any("error", err))
		}
		return
	}

	// move DLQ for fail to handle message
	c.reTryMoveMsgToDLQ(msg)
}

func (c *Charging) reTryMoveMsgToDLQ(msg *nats.Msg) {
	// A maximum of three attempts for move DLQ
	var err error = nil
	for i := 0; i < 3; i++ {
		err = c.moveMsgToDLQ(msg)
		if err != nil {
			tip := fmt.Sprintf("fail to move DLQ for the %d time", (i + 1))
			slog.Warn(tip, slog.Any("msg.data", string(msg.Data)))
		} else {
			break
		}
	}

	if err == nil {
		err = msg.Ack()
		if err != nil {
			slog.Warn("fail to do msg ack for message handling retry 3 times", slog.Any("error", err))
		}
	}
}

func (c *Charging) moveMsgToDLQ(msg *nats.Msg) error {
	c.dlqTimeout.Reset(idleDuration)
	select {
	case c.dlq.CH <- msg.Data:
		return nil
	case <-c.dlqTimeout.C:
		return fmt.Errorf("try to move DLQ with timeout, %v", idleDuration)
	}
}

func (c *Charging) handleMsgData(msg *nats.Msg) error {
	event, eventExtra, err := c.parseMessageData(msg)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err = c.acctEvtComp.AddNewAccountingEvent(ctx, event)
	if err != nil {
		return fmt.Errorf("fail to record event log, %v, %w", event, err)
	}
	st, err := c.acctSMComp.FindStatementByEventID(ctx, event)
	if err != nil {
		return fmt.Errorf("fail to check event statement, %v, %w", event, err)
	}
	if st != nil {
		slog.Warn("duplicated event id", slog.Any("event", event))
		return nil
	}
	err = c.acctUserComp.CheckAccountingUser(ctx, event.UserID)
	if err != nil {
		return fmt.Errorf("fail to check user balance, %v, %w", event.UserID, err)
	}
	err = c.acctSMComp.AddNewStatement(ctx, event, eventExtra, c.getCredit(event))
	if err != nil {
		return fmt.Errorf("fail to add statement and change balance, %v, %w", event, err)
	}
	c.checkBalanceAndSendNotification(ctx, event)
	return nil
}

func (c *Charging) parseMessageData(msg *nats.Msg) (*types.ACC_EVENT, *types.ACC_EVENT_EXTRA, error) {
	strData := string(msg.Data)
	slog.Info("Sub received", slog.Any("msg.data", strData), slog.Any("msg.subject", msg.Subject))
	evt := types.ACC_EVENT{}
	err := json.Unmarshal(msg.Data, &evt)
	if err != nil {
		return nil, nil, fmt.Errorf("fail to unmarshal event, %v, %w", strData, err)
	}
	evtExtra := types.ACC_EVENT_EXTRA{
		CustomerID:       "",
		CustomerPrice:    0,
		PriceUnit:        "",
		CustomerDuration: 0,
	}
	if len(strings.TrimSpace(evt.Extra)) < 1 {
		// extra is null
		return &evt, &evtExtra, nil
	}
	var exMap map[string]string
	err = json.Unmarshal([]byte(evt.Extra), &exMap)
	if err != nil {
		return nil, nil, fmt.Errorf("fail to unmarshal event extra, %v, %w", strData, err)
	}
	evtExtra.CustomerID = exMap["customer_id"]
	cusPriceStr, exists := exMap["customer_price"]
	if exists {
		cusPriceFloat, err := strconv.ParseFloat(cusPriceStr, 64)
		if err != nil {
			return nil, nil, fmt.Errorf("fail to unmarshal event extra customer_price, %v, %w", strData, err)
		}
		evtExtra.CustomerPrice = cusPriceFloat
	}
	evtExtra.PriceUnit = exMap["price_unit"]
	cusDurStr, exists := exMap["customer_duration"]
	if exists {
		cusDurFloat, err := strconv.ParseFloat(cusDurStr, 64)
		if err != nil {
			return nil, nil, fmt.Errorf("fail to unmarshal event extra customer_duration, %v, %w", strData, err)
		}
		evtExtra.CustomerDuration = cusDurFloat
	}
	return &evt, &evtExtra, nil
}

func (c *Charging) checkBalanceAndSendNotification(ctx context.Context, event *types.ACC_EVENT) {
	account, err := c.acctUserComp.ListAccountingByUserID(ctx, event.UserID)
	if err != nil {
		slog.Warn("fail to query account", slog.Any("userid", event.UserID), slog.Any("error", err))
		return
	}
	if account == nil {
		return
	}
	if account.Balance <= 0 {
		// retry 3 times for notification
		for i := 0; i < 3; i++ {
			err = c.sendNotification(types.REASON_LACK_BALANCE, "insufficient funds", event)
			if err != nil {
				tip := fmt.Sprintf("fail to notify for the %d time", (i + 1))
				slog.Warn(tip, slog.Any("event", event), slog.Any("error", err))
			} else {
				break
			}
		}
	}
}

func (c *Charging) sendNotification(reasonCode int, reasonMsg string, event *types.ACC_EVENT) error {
	notify := types.ACC_NOTIFY{
		CreatedAt:  time.Now(),
		ReasonCode: reasonCode,
		ReasonMsg:  reasonMsg,
	}
	if event != nil {
		notify.Uuid = event.Uuid
		notify.UserID = event.UserID
	}
	c.notifyTimeOut.Reset(idleDuration)
	select {
	case c.notify.CH <- notify:
		return nil
	case <-c.notifyTimeOut.C:
		return fmt.Errorf("try to sent notification with timeout, %v", idleDuration)
	}
}

func (c *Charging) getCredit(event *types.ACC_EVENT) float64 {
	changeValue := event.Value
	if event.ValueType == 1 {
		changeValue = TokenToCredit(int64(event.Value))
	}
	return changeValue
}
