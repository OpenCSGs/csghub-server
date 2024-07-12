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
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/mq"
)

type Metering struct {
	sysMQ     *mq.NatsHandler
	meterComp *component.MeteringComponent
}

func NewMetering(natHandler *mq.NatsHandler, config *config.Config) *Metering {
	meter := &Metering{
		sysMQ:     natHandler,
		meterComp: component.NewMeteringComponent(),
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
		time.Sleep(10 * time.Second)
	}
}

func (m *Metering) preReadMsgs() {
	var err error = nil
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
		err error = nil
	)
	for j := 0; j < 3; j++ {
		_, err = m.handleMsgData(msg)
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
	}

	// ack for handle metering message done
	err = msg.Ack()
	if err != nil {
		slog.Warn("failed to ack after processing meter msg", slog.Any("msg.data", strData), slog.Any("error", err))
	}
}

func (m *Metering) handleMsgData(msg jetstream.Msg) (*types.METERING_EVENT, error) {
	event, err := m.parseMessageData(msg)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err = m.meterComp.SaveMeteringEventRecord(ctx, event)
	if err != nil {
		return nil, fmt.Errorf("failed to save meter event, %v, %w", event, err)
	}
	return event, nil
}

func (m *Metering) parseMessageData(msg jetstream.Msg) (*types.METERING_EVENT, error) {
	strData := string(msg.Data())
	evt := types.METERING_EVENT{}
	err := json.Unmarshal(msg.Data(), &evt)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal meter event, %v, %w", strData, err)
	}
	return &evt, nil
}

func (m *Metering) moveMsgToDLQWithReTry(msg jetstream.Msg, limit int) error {
	// A maximum of five attempts for move DLQ
	var err error = nil
	for i := 0; i < limit; i++ {
		err = m.sysMQ.PublishMeterDataToDLQ(msg.Data())
		if err == nil {
			break
		}
	}
	return err
}
