package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"opencsg.com/csghub-server/accounting/component"
	"opencsg.com/csghub-server/builder/store/database"
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
	acctPriceComp *component.AccountingPriceComponent
	notify        *Notify
	dlq           *FeeDlq
	notifyTimeOut *time.Timer
	dlqTimeout    *time.Timer
}

func NewCharging(natHandler *mq.NatsHandler, config *config.Config) *Charging {
	charge := &Charging{
		sysMQ:         natHandler,
		acctSMComp:    component.NewAccountingStatement(),
		acctUserComp:  component.NewAccountingUser(),
		acctEvtComp:   component.NewAccountingEvent(),
		acctPriceComp: component.NewAccountingPrice(),
		notify:        NewNotify(natHandler),
		dlq:           NewFeeDlq(natHandler),
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
		c.handleReadMsgs(10)
		time.Sleep(2 * idleDuration)
	}
}

func (c *Charging) preReadMsgs() {
	var err error = nil
	var i int = 0
	for {
		i++
		err = c.sysMQ.BuildFeeEventStream()
		if err != nil {
			tip := fmt.Sprintf("fail to build event stream for the %d time", i)
			slog.Error(tip, slog.Any("error", err))
			time.Sleep(2 * time.Second)
			continue
		}
		break
	}
}

func (c *Charging) handleReadMsgs(failedLimit int) {
	failReadTime := 0
	for {
		if failReadTime >= failedLimit {
			break
		}
		err := c.sysMQ.VerifyFeeEventStream()
		if err != nil {
			tip := fmt.Sprintf("fail to verify stream for the %d time", (failReadTime + 1))
			slog.Error(tip, slog.Any("error", err))
			failReadTime++
			continue
		}
		msgs, err := c.sysMQ.FetchFeeEventMessages(5)
		if err == nats.ErrTimeout {
			continue
		}

		if err != nil {
			tip := fmt.Sprintf("fail to fetch event messages for the %d time", (failReadTime + 1))
			slog.Error(tip, slog.Any("error", err))
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
	slog.Debug("Fee->received", slog.Any("msg.subject", msg.Subject()), slog.Any("msg.data", strData))
	// A maximum of 3 attempts
	var err error = nil
	for j := 0; j < 3; j++ {
		err = c.handleMsgData(msg)
		if err == nil {
			break
		}
	}

	if err != nil {
		slog.Error("accounting handles a single msg with 3 retries", slog.Any("msg.data", strData), slog.Any("error", err))
		// move DLQ for fail to handle message
		err = c.moveMsgToDLQWithReTry(msg, 5)
		if err != nil {
			tip := fmt.Sprintf("failed to move fee msg to DLQ with %d retries", 5)
			slog.Error(tip, slog.Any("msg.data", strData), slog.Any("error", err))
		}
	}

	// The code for acknowledging the processing of a single metering message is done.
	err = msg.Ack()
	if err != nil {
		slog.Warn("failed to ack after processing fee msg", slog.Any("msg.data", strData), slog.Any("error", err))
	}
}

func (c *Charging) moveMsgToDLQWithReTry(msg jetstream.Msg, limit int) error {
	// A maximum of five attempts for move DLQ
	var err error = nil
	for i := 0; i < limit; i++ {
		err = c.moveMsgToDLQ(msg)
		if err == nil {
			break
		}
	}
	return err
}

func (c *Charging) moveMsgToDLQ(msg jetstream.Msg) error {
	c.dlqTimeout.Reset(idleDuration)
	select {
	case c.dlq.CH <- msg.Data():
		return nil
	case <-c.dlqTimeout.C:
		return fmt.Errorf("try to move fee DLQ with timeout, %v", idleDuration)
	}
}

func (c *Charging) handleMsgData(msg jetstream.Msg) error {
	event, err := c.parseMessageData(msg)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	err = c.acctEvtComp.AddNewAccountingEvent(ctx, event)
	if err != nil {
		return fmt.Errorf("fail to record fee event log, %v, %w", event, err)
	}
	st, err := c.acctSMComp.FindStatementByEventID(ctx, event)
	if err != nil {
		return fmt.Errorf("fail to check fee event statement, %v, %w", event, err)
	}
	if st != nil {
		slog.Warn("duplicated fee event id", slog.Any("event", event))
		return nil
	}
	priceReq := types.ACCT_PRICE_REQ{
		SKUType:    GetSKUTypeByScene(types.SceneType(event.Scene)),
		ResourceID: event.ResourceID,
		PriceTime:  event.CreatedAt,
	}
	ap, err := c.acctPriceComp.GetLatestByTime(ctx, priceReq)
	if err != nil {
		slog.Warn("did not find a valid price", slog.Any("priceReq", priceReq), slog.Any("error", err), slog.Any("event", event))
		return nil
	}
	err = c.acctUserComp.CheckAccountingUser(ctx, event.UserUUID)
	if err != nil {
		return fmt.Errorf("fail to check user balance, %v, %w", event.UserUUID, err)
	}
	slog.Debug("use sku price", slog.Any("price", ap), slog.Any("event", event))
	eventReq := c.buildPriceEvent(event, ap)
	err = c.acctSMComp.AddNewStatement(ctx, eventReq)
	if err != nil {
		return fmt.Errorf("fail to add statement and change balance, %v, %w", event, err)
	}
	c.checkBalanceAndSendNotification(ctx, event)
	return nil
}

func (c *Charging) buildPriceEvent(event *types.ACCT_EVENT, ap *database.AccountPrice) *types.ACCT_EVENT_REQ {
	cost := float64(ap.SkuPrice) * float64(event.Value) / float64(ap.SkuUnit)
	return &types.ACCT_EVENT_REQ{
		EventUUID:        event.Uuid,
		UserUUID:         event.UserUUID,
		Value:            (0 - cost),
		Scene:            types.SceneType(event.Scene),
		OpUID:            event.OpUID,
		CustomerID:       event.CustomerID,
		EventDate:        event.CreatedAt,
		Price:            float64(ap.SkuPrice),
		Consumption:      float64(event.Value),
		ValueType:        event.ValueType,
		ResourceID:       event.ResourceID,
		ResourceName:     event.ResourceName,
		SkuID:            ap.ID,
		RecordedAt:       event.CreatedAt,
		SkuUnit:          ap.SkuUnit,
		SkuUnitType:      ap.SkuUnitType,
		SkuPriceCurrency: ap.SkuPriceCurrency,
	}
}

func (c *Charging) parseMessageData(msg jetstream.Msg) (*types.ACCT_EVENT, error) {
	strData := string(msg.Data())
	evt := types.ACCT_EVENT{}
	err := json.Unmarshal(msg.Data(), &evt)
	if err != nil {
		return nil, fmt.Errorf("fail to unmarshal fee event, %v, %w", strData, err)
	}
	return &evt, nil
}

func (c *Charging) checkBalanceAndSendNotification(ctx context.Context, event *types.ACCT_EVENT) {
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
			err = c.sendNotification(int(types.ACCTLackBalance), "insufficient funds", event)
			if err == nil {
				break
			}
		}
		if err != nil {
			slog.Error("fail to notify for retry 3 times", slog.Any("event", event), slog.Any("error", err))
		}
	}
}

func (c *Charging) sendNotification(reasonCode int, reasonMsg string, event *types.ACCT_EVENT) error {
	notify := types.ACCT_NOTIFY{
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

func GetSKUTypeByScene(scene types.SceneType) types.SKUType {
	switch scene {
	case types.SceneModelInference:
		return types.SKUCSGHub
	case types.SceneSpace:
		return types.SKUCSGHub
	case types.SceneModelFinetune:
		return types.SKUCSGHub
	case types.SceneMultiSync:
		return types.SKUCSGHub
	case types.SceneStarship:
		return types.SKUStarship
	}
	return types.SKUReserve
}
