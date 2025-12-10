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
	"opencsg.com/csghub-server/accounting/utils"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/mq"
)

var (
	meterIdleDuration = 10 * time.Second
)

type Metering struct {
	sysMQ          mq.MessageQueue
	meterComp      component.MeteringComponent
	acctEvtComp    component.AccountingEventComponent
	chargingEnable bool
}

func NewMetering(natHandler mq.MessageQueue, config *config.Config) *Metering {
	meter := &Metering{
		sysMQ:          natHandler,
		meterComp:      component.NewMeteringComponent(),
		acctEvtComp:    component.NewAccountingEventComponent(),
		chargingEnable: config.Accounting.ChargingEnable,
	}
	return meter
}

func (m *Metering) Run() {
	go m.startMetering()
}

func (m *Metering) startMetering() {
	for {
		m.preReadMsgs()
		m.handleReadMsgs(10)
		time.Sleep(2 * meterIdleDuration)
	}
}

func (m *Metering) preReadMsgs() {
	var err error
	var i int = 0
	for {
		i++
		err = m.sysMQ.BuildMeterEventStream()
		if err != nil {
			tip := fmt.Sprintf("fail to build metering stream for the %d time", i)
			slog.Error(tip, slog.Any("error", err))
			time.Sleep(2 * time.Second)
			continue
		}
		err = m.sysMQ.BuildDLQStream()
		if err != nil {
			tip := fmt.Sprintf("fail to build DLQ stream in metering for the %d time", i)
			slog.Error(tip, slog.Any("error", err))
			time.Sleep(2 * time.Second)
			continue
		}
		break
	}
}

func (m *Metering) handleReadMsgs(failedLimit int) {
	failReadTime := 0
	for {
		if failReadTime >= failedLimit {
			break
		}
		err := m.sysMQ.VerifyMeteringStream()
		if err != nil {
			tip := fmt.Sprintf("fail to verify metering stream for the %d time", (failReadTime + 1))
			slog.Error(tip, slog.Any("err", err))
			failReadTime++
			continue
		}
		msgs, err := m.sysMQ.FetchMeterEventMessages(5)
		if err == nats.ErrTimeout {
			continue
		}

		if err != nil {
			tip := fmt.Sprintf("fail to fetch metering messages for the %d time", (failReadTime + 1))
			slog.Error(tip, slog.Any("err", err))
			failReadTime++
			continue
		}
		if msgs == nil {
			tip := fmt.Sprintf("metering msgs is null for the %d time", (failReadTime + 1))
			slog.Warn(tip)
			failReadTime++
			continue
		}

		for msg := range msgs.Messages() {
			m.handleMsgWithRetry(msg)
		}
	}
}

func (m *Metering) handleMsgWithRetry(msg jetstream.Msg) {
	strData := string(msg.Data())
	slog.Debug("Meter->received", slog.Any("msg.subject", msg.Subject()), slog.Any("msg.data", strData))
	// A maximum of 3 attempts
	var (
		err error                = nil
		evt *types.MeteringEvent = nil
	)
	for range 3 {
		evt, err = m.handleMsgData(msg)
		if err == nil {
			break
		}
	}

	if err != nil {
		slog.Error("metering handles a single msg with 3 retries", slog.Any("msg.data", strData), slog.Any("error", err))
		// move to DLQ for failed to handle message
		err = m.moveMsgToDLQWithReTry(msg, 5)
		if err != nil {
			tip := fmt.Sprintf("failed to move meter msg to DLQ with %d retries", 5)
			slog.Error(tip, slog.Any("msg.data", string(msg.Data())), slog.Any("error", err))
		}
	} else {
		if m.chargingEnable {
			err = m.pubFeeEventWithReTry(msg, evt, 5)
			if err != nil {
				tip := fmt.Sprintf("failed to pub fee event msg with %d retries", 5)
				slog.Error(tip, slog.Any("msg.data", string(msg.Data())), slog.Any("error", err))
				// todo: need more discuss on how to persist failed message finally
			}
		}
	}

	// ack for handle metering message done
	err = msg.Ack()
	if err != nil {
		slog.Warn("failed to ack after processing meter msg", slog.Any("msg.data", strData), slog.Any("error", err))
	}
}

func (c *Metering) checkDuplicatedEvent(ctx context.Context, event *types.MeteringEvent) (bool, *database.AccountMetering, error) {
	meter, err := c.meterComp.GetMeteringByEventUUID(ctx, event.Uuid)
	if err != nil {
		return false, nil, fmt.Errorf("failed to get metering by uuid, %v, %w", event.Uuid, err)
	}
	if meter != nil {
		return true, meter, types.ErrDuplicatedMeterByUUID
	}
	meter, err = c.meterComp.FindMeteringByCustomerIDAndRecordAtInMin(ctx, event.CustomerID, event.CreatedAt)
	if err != nil {
		return false, nil, fmt.Errorf("fail to check existing metering event in minute level, %v, %w", event, err)
	}
	if meter != nil {
		return true, meter, types.ErrDuplicatedMeterInMinute
	}
	return false, nil, nil
}

func (c *Metering) logAndVerifyEvent(ctx context.Context, event *types.MeteringEvent) error {
	var (
		existEvent   *database.AccountMetering
		isDuplicated bool
		err          error
	)
	if utils.IsNeedCheckMeteringInMinute(types.SceneType(event.Scene), event.ValueType) {
		isDuplicated, existEvent, err = c.checkDuplicatedEvent(ctx, event)
		if err != nil && !isDuplicated {
			return fmt.Errorf("check duplicated event, error: %w", err)
		}
	}
	err1 := c.acctEvtComp.AddNewAccountingEvent(ctx, event, isDuplicated)
	if err1 != nil {
		return fmt.Errorf("fail to save metering event log, %v, %w", event, err)
	}
	if isDuplicated {
		return fmt.Errorf("duplicated with event uuid %s, error: %w", existEvent.EventUUID, err)
	}
	return nil
}

func (m *Metering) handleMsgData(msg jetstream.Msg) (*types.MeteringEvent, error) {
	event, err := m.parseMessageData(msg)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = m.logAndVerifyEvent(ctx, event)
	if err != nil {
		return nil, fmt.Errorf("fail to log and verify event, error: %w", err)
	}

	err = m.meterComp.SaveMeteringEventRecord(ctx, event)
	if err != nil {
		return nil, fmt.Errorf("failed to save meter event, %v, %w", event, err)
	}
	return event, nil
}

func (m *Metering) parseMessageData(msg jetstream.Msg) (*types.MeteringEvent, error) {
	strData := string(msg.Data())
	evt := types.MeteringEvent{}
	err := json.Unmarshal(msg.Data(), &evt)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal meter event, %v, %w", strData, err)
	}
	return &evt, nil
}

func (m *Metering) pubFeeEventWithReTry(msg jetstream.Msg, evt *types.MeteringEvent, limit int) error {
	// A maximum of five attempts for pub fee event
	var err error
	for range limit {
		switch evt.ValueType {
		case types.TimeDurationMinType:
			err = m.sysMQ.PublishFeeCreditData(msg.Data())
		case types.TokenNumberType:
			err = m.sysMQ.PublishFeeTokenData(msg.Data())
		case types.QuotaNumberType:
			err = m.sysMQ.PublishFeeQuotaData(msg.Data())
		default:
			slog.Warn("unsupported metering event value type", slog.Any("value-type", evt.ValueType))
		}
		if err == nil {
			break
		}
	}
	return err
}

func (m *Metering) moveMsgToDLQWithReTry(msg jetstream.Msg, limit int) error {
	// A maximum of five attempts for move DLQ
	var err error
	for i := 0; i < limit; i++ {
		err = m.sysMQ.PublishMeterDataToDLQ(msg.Data())
		if err == nil {
			break
		}
	}
	return err
}
