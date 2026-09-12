package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
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
	retryLimit     int
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
		retryLimit:     config.Accounting.RetryLimit,
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
		MaxAge:   time.Duration(24*7) * time.Hour,
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
	retryLimit := m.retryLimit
	var (
		err error                = nil
		evt *types.MeteringEvent = nil
	)
	for range retryLimit {
		evt, err = m.handleMsgData(ctx, raw)
		if err == nil {
			break
		}
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

	extraMap, err := m.parseMessageExtraData(event.Extra)
	if err != nil {
		return nil, fmt.Errorf("parse event extra data, %w", err)
	}

	err = m.logAndVerifyEvent(ctx, event)
	if err != nil {
		return nil, fmt.Errorf("failed to log and verify metering event, error: %w", err)
	}

	promptNum := int64(0)
	promptNumStr, promptOK := extraMap[types.PromptTokenNum]
	if promptOK {
		var parseErr error
		promptNum, parseErr = strconv.ParseInt(strings.TrimSpace(promptNumStr), 10, 64)
		if parseErr != nil || promptNum < 0 {
			return nil, fmt.Errorf("metering consumer convert prompt token num %s to int64 error %w", promptNumStr, parseErr)
		}
	}

	completionNum := int64(0)
	completionNumStr, completionOK := extraMap[types.CompletionTokenNum]
	if completionOK {
		var parseErr error
		completionNum, parseErr = strconv.ParseInt(strings.TrimSpace(completionNumStr), 10, 64)
		if parseErr != nil || completionNum < 0 {
			return nil, fmt.Errorf("metering consumer convert completion token num %s to int64 error %w", completionNumStr, err)
		}
	}

	promptTokenCachedNum := int64(0)
	promptTokenCacheNumStr, promptTokenCacheOK := extraMap[types.PromptTokenCacheNum]
	if promptTokenCacheOK {
		var parseErr error
		promptTokenCachedNum, parseErr = strconv.ParseInt(strings.TrimSpace(promptTokenCacheNumStr), 10, 64)
		if parseErr != nil || promptTokenCachedNum < 0 {
			return nil, fmt.Errorf("metering consumer convert prompt token cache num %s to int64 error %w", promptTokenCacheNumStr, parseErr)
		}
	}

	if promptTokenCachedNum > promptNum {
		return nil, fmt.Errorf("metering consumer prompt token cache num %d is greater than prompt token num %d", promptTokenCachedNum, promptNum)
	}

	duration := float64(0)
	durationStr, ok := extraMap[types.CompletionDuration]
	if ok && len(strings.TrimSpace(durationStr)) > 0 {
		var parseErr error
		duration, parseErr = strconv.ParseFloat(strings.TrimSpace(durationStr), 64)
		if parseErr != nil {
			return nil, fmt.Errorf("metering consumer failed to parse completion duration %s to float64 error %w", durationStr, parseErr)
		}
	}

	extra := types.MeteringExtra{
		EventDate:         utils.EventDateInLocalTZ(event.CreatedAt),
		PromptToken:       float64(promptNum),
		PromptCachedToken: float64(promptTokenCachedNum),
		CompletionToken:   float64(completionNum),
		DataType:          extraMap[types.CompletionDataType],
		Resolution:        extraMap[types.CompletionResolution],
		Duration:          duration,
		SkuUnitType:       utils.GetSkuUnitTypeByScene(types.SceneType(event.Scene)),
		APIKey:            extraMap[types.ConsumeApiKey],
	}

	err = m.meterComp.SaveMeteringEventRecord(ctx, event, extra)
	if err != nil {
		return nil, fmt.Errorf("failed to record metering event, %v, error: %w", event, err)
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

func (c *MeteringImpl) parseMessageExtraData(extra string) (map[string]string, error) {
	extraMap := make(map[string]string, 0)
	if len(strings.Trim(extra, " ")) == 0 {
		return extraMap, nil
	}
	err := json.Unmarshal([]byte(extra), &extraMap)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal metering event extra json data, %v, %w", extra, err)
	}
	return extraMap, nil
}

func (m *MeteringImpl) pubFeeEventWithReTry(raw []byte, evt *types.MeteringEvent, limit int) error {
	// A maximum of five attempts for pub fee event
	var err error
	for range limit {
		switch evt.ValueType {
		case types.TimeDurationMinType:
			err = m.bldMQ.Publish(bldmq.FeeSendSubject, raw)
		case types.TokenNumberType, types.CountNumberType:
			err = m.bldMQ.Publish(bldmq.TokenSendSubject, raw)
		case types.QuotaNumberType:
			err = m.bldMQ.Publish(bldmq.QuotaSendSubject, raw)
		default:
			slog.Warn("unsupported metering event value type for pub fee event", slog.Any("value-type", evt.ValueType))
		}
		if err == nil {
			break
		}
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
	}
	return err
}
