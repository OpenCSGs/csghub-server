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
	"github.com/nats-io/nats.go/jetstream"
	"opencsg.com/csghub-server/accounting/component"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/mq"
)

var (
	idleDuration = 10 * time.Second
)

type Charging struct {
	sysMQ         *mq.NatsHandler
	acctSMComp    *component.AccountingStatementComponent
	acctUserComp  *component.AccountingUserComponent
	acctEvtComp   *component.AccountingEventComponent
	notify        *Notify
	dlq           *Dlq
	notifyTimeOut *time.Timer
	dlqTimeout    *time.Timer
}

func NewCharging(natHandler *mq.NatsHandler, config *config.Config) *Charging {
	charge := &Charging{
		sysMQ:         natHandler,
		acctSMComp:    component.NewAccountingStatement(),
		acctUserComp:  component.NewAccountingUser(),
		acctEvtComp:   component.NewAccountingEvent(),
		notify:        NewNotify(natHandler),
		dlq:           NewDlq(natHandler),
		notifyTimeOut: time.NewTimer(idleDuration),
		dlqTimeout:    time.NewTimer(idleDuration),
	}
	return charge
}

func (c *Charging) Run() {
	go c.startCharging()
	go c.notify.Run()
	go c.dlq.Run()

}

func (c *Charging) startCharging() {
	for {
		c.preReadMsgs()
		c.handleReadMsgs()
		time.Sleep(2 * idleDuration)
	}
}

func (c *Charging) preReadMsgs() {
	var err error = nil
	var i int = 0
	for {
		i++
		err = c.sysMQ.BuildEventStream()
		if err != nil {
			tip := fmt.Sprintf("fail to build event stream for the %d time", i)
			slog.Error(tip, slog.Any("err", err))
			time.Sleep(2 * time.Second)
			continue
		}
		break
	}
}

func (c *Charging) handleReadMsgs() {
	failReadTime := 0
	for {
		if failReadTime >= 10 {
			break
		}
		err := c.sysMQ.VerifyEventStream()
		if err != nil {
			tip := fmt.Sprintf("fail to verify stream for the %d time", (failReadTime + 1))
			slog.Error(tip, slog.Any("err", err))
			failReadTime++
			continue
		}
		msgs, err := c.sysMQ.FetchFeeEventMessages(5)
		if err == nats.ErrTimeout {
			continue
		}

		if err != nil {
			tip := fmt.Sprintf("fail to fetch event messages for the %d time", (failReadTime + 1))
			slog.Error(tip, slog.Any("err", err))
			failReadTime++
			continue
		}
		if msgs == nil {
			tip := fmt.Sprintf("msgs is null for the %d time", (failReadTime + 1))
			slog.Warn(tip)
			failReadTime++
			continue
		}

		for msg := range msgs.Messages() {
			c.handleMsgWithRetry(msg)
		}

	}
}

func (c *Charging) handleMsgWithRetry(msg jetstream.Msg) {
	strData := string(msg.Data())
	slog.Info("Sub received", slog.Any("msg.data", strData), slog.Any("msg.subject", msg.Subject()))
	// A maximum of 3 attempts
	var err error = nil
	for j := 0; j < 3; j++ {
		err = c.handleMsgData(msg)
		if err == nil {
			break
		}
	}

	if err != nil {
		slog.Error("error happen when handle single msg with retry 3 times", slog.Any("msg.data", strData), slog.Any("error", err))
		// move DLQ for fail to handle message
		c.moveMsgToDLQWithReTry(msg)
	} else {
		// handle message success
		err = msg.Ack()
		if err != nil {
			slog.Warn("fail to do msg ack for deal with message success", slog.Any("error", err))
		}
	}

}

func (c *Charging) moveMsgToDLQWithReTry(msg jetstream.Msg) {
	// A maximum of three attempts for move DLQ
	var err error = nil
	for i := 0; i < 5; i++ {
		err = c.moveMsgToDLQ(msg)
		if err == nil {
			break
		}
	}

	if err != nil {
		slog.Error("fail to move msg to DLQ with retry 5 times", slog.Any("error", err))
	} else {
		// move dlq success
		err = msg.Ack()
		if err != nil {
			slog.Warn("fail to do msg ack for move msg to DLQ success", slog.Any("msg.data", string(msg.Data())), slog.Any("error", err))
		}
	}
}

func (c *Charging) moveMsgToDLQ(msg jetstream.Msg) error {
	c.dlqTimeout.Reset(idleDuration)
	select {
	case c.dlq.CH <- msg.Data():
		return nil
	case <-c.dlqTimeout.C:
		return fmt.Errorf("try to move DLQ with timeout, %v", idleDuration)
	}
}

func (c *Charging) handleMsgData(msg jetstream.Msg) error {
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
	err = c.acctUserComp.CheckAccountingUser(ctx, event.UserUUID)
	if err != nil {
		return fmt.Errorf("fail to check user balance, %v, %w", event.UserUUID, err)
	}
	err = c.acctSMComp.AddNewStatement(ctx, event, eventExtra, c.getCredit(event))
	if err != nil {
		return fmt.Errorf("fail to add statement and change balance, %v, %w", event, err)
	}
	c.checkBalanceAndSendNotification(ctx, event)
	return nil
}

func (c *Charging) parseMessageData(msg jetstream.Msg) (*types.ACC_EVENT, *types.ACC_EVENT_EXTRA, error) {
	strData := string(msg.Data())
	evt := types.ACC_EVENT{}
	err := json.Unmarshal(msg.Data(), &evt)
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
	account, err := c.acctUserComp.GetAccountingByUserID(ctx, event.UserUUID)
	if err != nil {
		slog.Warn("fail to query account before check account balance", slog.Any("user_uuid", event.UserUUID), slog.Any("error", err))
		return
	}
	if account == nil {
		return
	}
	if account.Balance <= 0 {
		// retry 3 times for notification
		for i := 0; i < 3; i++ {
			err = c.sendNotification(types.REASON_LACK_BALANCE, "insufficient funds", event)
			if err == nil {
				break
			}
		}
		if err != nil {
			slog.Error("fail to notify for retry 3 times", slog.Any("event", event), slog.Any("error", err))
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
		notify.UserUUID = event.UserUUID
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
