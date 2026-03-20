package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"opencsg.com/csghub-server/accounting/component"
	"opencsg.com/csghub-server/accounting/utils"
	bldmq "opencsg.com/csghub-server/builder/mq"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
)

type Metering interface {
	Run()
}

type MeteringImpl struct {
	meterComp      component.MeteringComponent
	acctEvtComp    component.AccountingEventComponent
	chargingEnable bool
	bldMQ          bldmq.MessageQueue
}

func NewMetering(config *config.Config, mqFactory bldmq.MessageQueueFactory) (Metering, error) {
	mq, err := mqFactory.GetInstance()
	if err != nil {
		return nil, fmt.Errorf("failed to get message queue factory instance error: %w", err)
	}
	meter := &MeteringImpl{
		meterComp:      component.NewMeteringComponent(),
		acctEvtComp:    component.NewAccountingEventComponent(),
		chargingEnable: config.Accounting.ChargingEnable,
		bldMQ:          mq,
	}
	return meter, nil
}

func (m *MeteringImpl) Run() {
	err := m.bldMQ.Subscribe(bldmq.SubscribeParams{
		Group: bldmq.MeteringEventGroup,
		Topics: []string{
			bldmq.MeterDurationSendSubject,
			bldmq.MeterTokenSendSubject,
			bldmq.MeterQuotaSendSubject,
		},
		AutoACK:  true,
		Callback: m.handleMsgWithRetry,
	})
	if err != nil {
		slog.Error("failed to subscribe metering event", slog.Any("error", err))
	}
}

func (m *MeteringImpl) handleMsgWithRetry(raw []byte, meta bldmq.MessageMeta) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	strData := string(raw)
	slog.DebugContext(ctx, "Meter->received", slog.Any("msg.subject", meta.Topic), slog.Any("msg.data", strData))
	// A maximum of 3 attempts
	retryLimit := 3
	var (
		err error                = nil
		evt *types.MeteringEvent = nil
	)
	for range retryLimit {
		evt, err = m.handleMsgData(ctx, raw)
		if err == nil {
			break
		}
		time.Sleep(1 * time.Second)
	}

	if err != nil {
		tip := fmt.Sprintf("handles a single metering msg with %d retries", retryLimit)
		slog.ErrorContext(ctx, tip, slog.Any("event", strData), slog.Any("error", err), slog.Any("topic", meta.Topic))
		// move to DLQ for failed to handle message
		err = m.moveMsgToDLQWithReTry(raw, retryLimit)
		if err != nil {
			tip := fmt.Sprintf("failed to move metering msg to DLQ with %d retries", retryLimit)
			slog.ErrorContext(ctx, tip, slog.Any("event", strData), slog.Any("error", err), slog.Any("topic", meta.Topic))
		}
		return err
	}

	if m.chargingEnable {
		err = m.pubFeeEventWithReTry(raw, evt, retryLimit)
		if err != nil {
			tip := fmt.Sprintf("failed to pub fee event msg with %d retries in metering consumer", retryLimit)
			slog.ErrorContext(ctx, tip, slog.Any("event", strData), slog.Any("error", err), slog.Any("topic", meta.Topic))
			return err
			// todo: need more discuss on how to persist failed message finally
		}
	}

	return nil
}

func (m *MeteringImpl) handleMsgData(ctx context.Context, raw []byte) (*types.MeteringEvent, error) {
	event, err := m.parseMessageData(raw)
	if err != nil {
		return nil, err
	}

	err = m.logAndVerifyEvent(ctx, event)
	if err != nil {
		return nil, fmt.Errorf("failed to log and verify metering event, error: %w", err)
	}

	err = m.meterComp.SaveMeteringEventRecord(ctx, event)
	if err != nil {
		return nil, fmt.Errorf("failed to save metering event, %v, %w", event, err)
	}
	return event, nil
}

func (c *MeteringImpl) logAndVerifyEvent(ctx context.Context, event *types.MeteringEvent) error {
	var (
		existEvent   *database.AccountMetering
		isDuplicated bool
		err          error
	)
	if utils.IsNeedCheckMeteringInMinute(types.SceneType(event.Scene), event.ValueType) {
		isDuplicated, existEvent, err = c.checkDuplicatedEvent(ctx, event)
		if err != nil && !isDuplicated {
			return fmt.Errorf("check duplicated metering event, error: %w", err)
		}
	}
	err1 := c.acctEvtComp.AddNewAccountingEvent(ctx, event, isDuplicated)
	if err1 != nil {
		return fmt.Errorf("failed to save metering event log, %v, %w", event, err)
	}
	if isDuplicated {
		return fmt.Errorf("duplicated with metering event uuid %s, error: %w", existEvent.EventUUID, err)
	}
	return nil
}

func (c *MeteringImpl) checkDuplicatedEvent(ctx context.Context, event *types.MeteringEvent) (bool, *database.AccountMetering, error) {
	meter, err := c.meterComp.GetMeteringByEventUUID(ctx, event.Uuid)
	if err != nil {
		return false, nil, fmt.Errorf("failed to get metering event by uuid, %v, %w", event.Uuid, err)
	}
	if meter != nil {
		return true, meter, types.ErrDuplicatedMeterByUUID
	}
	meter, err = c.meterComp.FindMeteringByCustomerIDAndRecordAtInMin(ctx, event.CustomerID, event.CreatedAt)
	if err != nil {
		return false, nil, fmt.Errorf("failed to check existing metering event in minute level, %v, %w", event, err)
	}
	if meter != nil {
		return true, meter, types.ErrDuplicatedMeterInMinute
	}
	return false, nil, nil
}

func (m *MeteringImpl) parseMessageData(raw []byte) (*types.MeteringEvent, error) {
	strData := string(raw)
	evt := types.MeteringEvent{}
	err := json.Unmarshal(raw, &evt)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal metering event, %v, %w", strData, err)
	}
	return &evt, nil
}

func (m *MeteringImpl) pubFeeEventWithReTry(raw []byte, evt *types.MeteringEvent, limit int) error {
	// A maximum of five attempts for pub fee event
	var err error
	for range limit {
		switch evt.ValueType {
		case types.TimeDurationMinType:
			err = m.bldMQ.Publish(bldmq.FeeSendSubject, raw)
		case types.TokenNumberType:
			err = m.bldMQ.Publish(bldmq.TokenSendSubject, raw)
		case types.QuotaNumberType:
			err = m.bldMQ.Publish(bldmq.QuotaSendSubject, raw)
		default:
			slog.Warn("unsupported metering event value type for pub fee event", slog.Any("value-type", evt.ValueType))
		}
		if err == nil {
			break
		}
		time.Sleep(1 * time.Second)
	}
	return err
}

func (m *MeteringImpl) moveMsgToDLQWithReTry(raw []byte, limit int) error {
	// A maximum of five attempts for move DLQ
	var err error
	for range limit {
		err = m.bldMQ.Publish(bldmq.DLQMeterSubject, raw)
		if err == nil {
			break
		}
		time.Sleep(1 * time.Second)
	}
	return err
}
