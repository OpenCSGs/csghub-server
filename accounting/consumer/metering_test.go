package consumer

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	mockNats "opencsg.com/csghub-server/_mocks/github.com/nats-io/nats.go/jetstream"
	mockacct "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/accounting/component"
	mockmq "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/mq"
	"opencsg.com/csghub-server/accounting/component"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/mq"
)

func NewTestConsumerMetering(natHandler mq.MessageQueue, meterComp component.MeteringComponent, config *config.Config) *Metering {
	meter := &Metering{
		sysMQ:          natHandler,
		meterComp:      meterComp,
		chargingEnable: config.Accounting.ChargingEnable,
	}
	return meter
}

func TestConsumerMetering_preReadMsgs(t *testing.T) {
	cfg, err := config.LoadConfig()
	cfg.Accounting.ChargingEnable = true
	require.Nil(t, err)

	mq := mockmq.NewMockMessageQueue(t)
	mq.EXPECT().BuildMeterEventStream().Return(nil)
	mq.EXPECT().BuildDLQStream().Return(nil)

	meterComp := mockacct.NewMockMeteringComponent(t)

	meter := NewTestConsumerMetering(mq, meterComp, cfg)

	meter.preReadMsgs()
}

func TestConsumerMetering_handleReadMsgs(t *testing.T) {
	cfg, err := config.LoadConfig()
	cfg.Accounting.ChargingEnable = true
	require.Nil(t, err)

	mq := mockmq.NewMockMessageQueue(t)
	mq.EXPECT().VerifyMeteringStream().Return(nil)
	mq.EXPECT().FetchMeterEventMessages(5).Return(nil, errors.New("can not get msg"))

	meterComp := mockacct.NewMockMeteringComponent(t)
	meter := NewTestConsumerMetering(mq, meterComp, cfg)

	done := make(chan bool)
	go func() {
		meter.handleReadMsgs(1)
		close(done)
	}()

	<-done
}

func TestConsumerMetering_handleMsgWithRetry(t *testing.T) {

	cfg, err := config.LoadConfig()
	cfg.Accounting.ChargingEnable = true
	require.Nil(t, err)

	var event = types.METERING_EVENT{
		Uuid:         uuid.MustParse("e2a0683d-ff52-4caf-915d-1ab052c57322"),
		UserUUID:     "bd05a582-a185-42d7-bf19-ad8108c4523b",
		Value:        5000,
		ValueType:    1,
		Scene:        22,
		OpUID:        "",
		ResourceID:   "Autohub/gui_agent",
		ResourceName: "Autohub/gui_agent",
		CustomerID:   "gui_agent",
		CreatedAt:    time.Date(2024, time.November, 6, 13, 19, 0, 0, time.UTC),
		Extra:        "{}",
	}

	testData := []struct {
		typeStr   string
		typeValue int
	}{
		{"token", types.TokenNumberType},
		{"mintue", types.TimeDurationMinType},
		{"quota", types.QuotaNumberType},
	}

	for _, k := range testData {
		t.Run(k.typeStr, func(t *testing.T) {
			event.ValueType = k.typeValue
			str, err := json.Marshal(event)
			require.Nil(t, err)

			msg := mockNats.NewMockMsg(t)
			msg.EXPECT().Data().Return(str)
			msg.EXPECT().Subject().Return("")
			msg.EXPECT().Ack().Return(nil)

			mq := mockmq.NewMockMessageQueue(t)
			if k.typeStr == "token" {
				mq.EXPECT().PublishFeeTokenData(str).Return(nil)
			}
			if k.typeStr == "mintue" {
				mq.EXPECT().PublishFeeCreditData(str).Return(nil)
			}
			if k.typeStr == "quota" {
				mq.EXPECT().PublishFeeQuotaData(str).Return(nil)
			}

			meterComp := mockacct.NewMockMeteringComponent(t)
			meterComp.EXPECT().SaveMeteringEventRecord(mock.Anything, &event).Return(nil)

			meter := NewTestConsumerMetering(mq, meterComp, cfg)
			meter.handleMsgWithRetry(msg)
		})
	}

	t.Run("error", func(t *testing.T) {
		str := []byte("error error error")

		msg := mockNats.NewMockMsg(t)
		msg.EXPECT().Data().Return(str)
		msg.EXPECT().Subject().Return("")
		msg.EXPECT().Ack().Return(nil)

		mq := mockmq.NewMockMessageQueue(t)
		mq.EXPECT().PublishMeterDataToDLQ(str).Return(nil)

		meterComp := mockacct.NewMockMeteringComponent(t)

		meter := NewTestConsumerMetering(mq, meterComp, cfg)
		meter.handleMsgWithRetry(msg)
	})

}

func TestConsumerMetering_handleMsgData(t *testing.T) {
	cfg, err := config.LoadConfig()
	cfg.Accounting.ChargingEnable = true
	require.Nil(t, err)

	var event = types.METERING_EVENT{
		Uuid:         uuid.MustParse("e2a0683d-ff52-4caf-915d-1ab052c57322"),
		UserUUID:     "bd05a582-a185-42d7-bf19-ad8108c4523b",
		Value:        5000,
		ValueType:    1,
		Scene:        22,
		OpUID:        "",
		ResourceID:   "Autohub/gui_agent",
		ResourceName: "Autohub/gui_agent",
		CustomerID:   "gui_agent",
		CreatedAt:    time.Date(2024, time.November, 6, 13, 19, 0, 0, time.UTC),
		Extra:        "{}",
	}

	str, err := json.Marshal(event)
	require.Nil(t, err)

	msg := mockNats.NewMockMsg(t)
	msg.EXPECT().Data().Return(str)

	mq := mockmq.NewMockMessageQueue(t)

	meterComp := mockacct.NewMockMeteringComponent(t)
	meterComp.EXPECT().SaveMeteringEventRecord(mock.Anything, &event).Return(nil)

	meter := NewTestConsumerMetering(mq, meterComp, cfg)
	res, err := meter.handleMsgData(msg)
	require.Nil(t, err)
	require.Equal(t, event, *res)
}

func TestConsumerMetering_parseMessageData(t *testing.T) {
	cfg, err := config.LoadConfig()
	cfg.Accounting.ChargingEnable = true
	require.Nil(t, err)

	var event = types.METERING_EVENT{
		Uuid:         uuid.MustParse("e2a0683d-ff52-4caf-915d-1ab052c57322"),
		UserUUID:     "bd05a582-a185-42d7-bf19-ad8108c4523b",
		Value:        5000,
		ValueType:    1,
		Scene:        22,
		OpUID:        "",
		ResourceID:   "Autohub/gui_agent",
		ResourceName: "Autohub/gui_agent",
		CustomerID:   "gui_agent",
		CreatedAt:    time.Date(2024, time.November, 6, 13, 19, 0, 0, time.UTC),
		Extra:        "{}",
	}

	str, err := json.Marshal(event)
	require.Nil(t, err)

	msg := mockNats.NewMockMsg(t)
	msg.EXPECT().Data().Return(str)

	mq := mockmq.NewMockMessageQueue(t)

	meterComp := mockacct.NewMockMeteringComponent(t)

	meter := NewTestConsumerMetering(mq, meterComp, cfg)
	res, err := meter.parseMessageData(msg)
	require.Nil(t, err)
	require.Equal(t, event, *res)
}

func TestConsumerMetering_pubFeeEventWithReTry(t *testing.T) {
	cfg, err := config.LoadConfig()
	cfg.Accounting.ChargingEnable = true
	require.Nil(t, err)

	var event = types.METERING_EVENT{
		Uuid:         uuid.MustParse("e2a0683d-ff52-4caf-915d-1ab052c57322"),
		UserUUID:     "bd05a582-a185-42d7-bf19-ad8108c4523b",
		Value:        5000,
		ValueType:    1,
		Scene:        22,
		OpUID:        "",
		ResourceID:   "Autohub/gui_agent",
		ResourceName: "Autohub/gui_agent",
		CustomerID:   "gui_agent",
		CreatedAt:    time.Date(2024, time.November, 6, 13, 19, 0, 0, time.UTC),
		Extra:        "{}",
	}

	testData := []struct {
		typeStr   string
		typeValue int
	}{
		{"token", types.TokenNumberType},
		{"mintue", types.TimeDurationMinType},
		{"quota", types.QuotaNumberType},
	}

	for _, k := range testData {
		t.Run(k.typeStr, func(t *testing.T) {
			event.ValueType = k.typeValue
			str, err := json.Marshal(event)
			require.Nil(t, err)

			msg := mockNats.NewMockMsg(t)
			msg.EXPECT().Data().Return(str)

			mq := mockmq.NewMockMessageQueue(t)
			if k.typeStr == "token" {
				mq.EXPECT().PublishFeeTokenData(str).Return(nil)
			}
			if k.typeStr == "mintue" {
				mq.EXPECT().PublishFeeCreditData(str).Return(nil)
			}
			if k.typeStr == "quota" {
				mq.EXPECT().PublishFeeQuotaData(str).Return(nil)
			}

			meterComp := mockacct.NewMockMeteringComponent(t)

			meter := NewTestConsumerMetering(mq, meterComp, cfg)
			err = meter.pubFeeEventWithReTry(msg, &event, 1)
			require.Nil(t, err)
		})
	}
}

func TestConsumerMetering_moveMsgToDLQWithReTry(t *testing.T) {
	cfg, err := config.LoadConfig()
	cfg.Accounting.ChargingEnable = true
	require.Nil(t, err)

	var event = types.METERING_EVENT{
		Uuid:         uuid.MustParse("e2a0683d-ff52-4caf-915d-1ab052c57322"),
		UserUUID:     "bd05a582-a185-42d7-bf19-ad8108c4523b",
		Value:        5000,
		ValueType:    1,
		Scene:        22,
		OpUID:        "",
		ResourceID:   "Autohub/gui_agent",
		ResourceName: "Autohub/gui_agent",
		CustomerID:   "gui_agent",
		CreatedAt:    time.Date(2024, time.November, 6, 13, 19, 0, 0, time.UTC),
		Extra:        "{}",
	}

	str, err := json.Marshal(event)
	require.Nil(t, err)

	msg := mockNats.NewMockMsg(t)
	msg.EXPECT().Data().Return(str)

	mq := mockmq.NewMockMessageQueue(t)
	mq.EXPECT().PublishMeterDataToDLQ(str).Return(nil)

	meterComp := mockacct.NewMockMeteringComponent(t)

	meter := NewTestConsumerMetering(mq, meterComp, cfg)
	err = meter.moveMsgToDLQWithReTry(msg, 3)
	require.Nil(t, err)

}
