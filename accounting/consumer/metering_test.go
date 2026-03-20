package consumer

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	mockacct "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/accounting/component"
	mockmq "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/mq"
	"opencsg.com/csghub-server/accounting/component"
	bldmq "opencsg.com/csghub-server/builder/mq"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
)

func NewTestConsumerMetering(
	meterComp component.MeteringComponent,
	acctEvtComp component.AccountingEventComponent,
	config *config.Config,
	mq bldmq.MessageQueue) Metering {
	meter := &MeteringImpl{
		meterComp:      meterComp,
		acctEvtComp:    acctEvtComp,
		chargingEnable: config.Accounting.ChargingEnable,
		bldMQ:          mq,
	}
	return meter
}

func createTestMeteringEvent() *types.MeteringEvent {
	return &types.MeteringEvent{
		Uuid:         uuid.New(),
		UserUUID:     "test-user-uuid",
		Value:        100,
		ValueType:    types.TimeDurationMinType,
		Scene:        int(types.SceneSpace),
		OpUID:        "test-op-uid",
		ResourceID:   "test-resource-id",
		ResourceName: "test-resource-name",
		CustomerID:   "test-customer-id",
		CreatedAt:    time.Now(),
		Extra:        `{"key":"value"}`,
	}
}

func TestMeteringImpl_ParseMessageData_Success(t *testing.T) {
	metering := &MeteringImpl{}
	event := createTestMeteringEvent()
	data, err := json.Marshal(event)
	require.NoError(t, err)

	result, err := metering.parseMessageData(data)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, event.Uuid, result.Uuid)
	require.Equal(t, event.UserUUID, result.UserUUID)
	require.Equal(t, event.Value, result.Value)
	require.Equal(t, event.ValueType, result.ValueType)
}

func TestMeteringImpl_ParseMessageData_InvalidJSON(t *testing.T) {
	metering := &MeteringImpl{}
	invalidData := []byte(`{invalid json`)

	_, err := metering.parseMessageData(invalidData)

	require.Error(t, err)
}

func TestMeteringImpl_CheckDuplicatedEvent_NotFound(t *testing.T) {
	mockMeterComp := mockacct.NewMockMeteringComponent(t)
	mockAcctEvtComp := mockacct.NewMockAccountingEventComponent(t)
	metering := &MeteringImpl{
		meterComp:   mockMeterComp,
		acctEvtComp: mockAcctEvtComp,
	}

	ctx := context.Background()
	event := createTestMeteringEvent()

	mockMeterComp.EXPECT().GetMeteringByEventUUID(ctx, event.Uuid).Return(nil, nil)
	mockMeterComp.EXPECT().FindMeteringByCustomerIDAndRecordAtInMin(ctx, event.CustomerID, event.CreatedAt).Return(nil, nil)

	isDuplicated, existEvent, err := metering.checkDuplicatedEvent(ctx, event)

	require.NoError(t, err)
	require.False(t, isDuplicated)
	require.Nil(t, existEvent)
}

func TestMeteringImpl_CheckDuplicatedEvent_FoundByUUID(t *testing.T) {
	mockMeterComp := mockacct.NewMockMeteringComponent(t)
	mockAcctEvtComp := mockacct.NewMockAccountingEventComponent(t)
	metering := &MeteringImpl{
		meterComp:   mockMeterComp,
		acctEvtComp: mockAcctEvtComp,
	}

	ctx := context.Background()
	event := createTestMeteringEvent()
	existingMetering := &database.AccountMetering{
		EventUUID: event.Uuid,
	}

	mockMeterComp.EXPECT().GetMeteringByEventUUID(ctx, event.Uuid).Return(existingMetering, nil)

	isDuplicated, existEvent, err := metering.checkDuplicatedEvent(ctx, event)

	require.Error(t, err)
	require.True(t, errors.Is(err, types.ErrDuplicatedMeterByUUID))
	require.True(t, isDuplicated)
	require.NotNil(t, existEvent)
}

func TestMeteringImpl_CheckDuplicatedEvent_FoundInMinute(t *testing.T) {
	mockMeterComp := mockacct.NewMockMeteringComponent(t)
	mockAcctEvtComp := mockacct.NewMockAccountingEventComponent(t)
	metering := &MeteringImpl{
		meterComp:   mockMeterComp,
		acctEvtComp: mockAcctEvtComp,
	}

	ctx := context.Background()
	event := createTestMeteringEvent()
	existingMetering := &database.AccountMetering{
		EventUUID:  event.Uuid,
		CustomerID: event.CustomerID,
	}

	mockMeterComp.EXPECT().GetMeteringByEventUUID(ctx, event.Uuid).Return(nil, nil)
	mockMeterComp.EXPECT().FindMeteringByCustomerIDAndRecordAtInMin(ctx, event.CustomerID, event.CreatedAt).Return(existingMetering, nil)

	isDuplicated, existEvent, err := metering.checkDuplicatedEvent(ctx, event)

	require.Error(t, err)
	require.True(t, errors.Is(err, types.ErrDuplicatedMeterInMinute))
	require.True(t, isDuplicated)
	require.NotNil(t, existEvent)
}

func TestMeteringImpl_CheckDuplicatedEvent_GetByUUIDError(t *testing.T) {
	mockMeterComp := mockacct.NewMockMeteringComponent(t)
	mockAcctEvtComp := mockacct.NewMockAccountingEventComponent(t)
	metering := &MeteringImpl{
		meterComp:   mockMeterComp,
		acctEvtComp: mockAcctEvtComp,
	}

	ctx := context.Background()
	event := createTestMeteringEvent()
	testErr := errors.New("database error")

	mockMeterComp.EXPECT().GetMeteringByEventUUID(ctx, event.Uuid).Return(nil, testErr)

	_, _, err := metering.checkDuplicatedEvent(ctx, event)

	require.Error(t, err)
}

func TestMeteringImpl_LogAndVerifyEvent_Success(t *testing.T) {
	mockMeterComp := mockacct.NewMockMeteringComponent(t)
	mockAcctEvtComp := mockacct.NewMockAccountingEventComponent(t)
	metering := &MeteringImpl{
		meterComp:   mockMeterComp,
		acctEvtComp: mockAcctEvtComp,
	}

	ctx := context.Background()
	event := createTestMeteringEvent()

	mockMeterComp.EXPECT().GetMeteringByEventUUID(ctx, event.Uuid).Return(nil, nil)
	mockMeterComp.EXPECT().FindMeteringByCustomerIDAndRecordAtInMin(ctx, event.CustomerID, event.CreatedAt).Return(nil, nil)
	mockAcctEvtComp.EXPECT().AddNewAccountingEvent(ctx, event, false).Return(nil)

	err := metering.logAndVerifyEvent(ctx, event)

	require.NoError(t, err)
}

func TestMeteringImpl_LogAndVerifyEvent_Duplicated(t *testing.T) {
	mockMeterComp := mockacct.NewMockMeteringComponent(t)
	mockAcctEvtComp := mockacct.NewMockAccountingEventComponent(t)
	metering := &MeteringImpl{
		meterComp:   mockMeterComp,
		acctEvtComp: mockAcctEvtComp,
	}

	ctx := context.Background()
	event := createTestMeteringEvent()
	existingMetering := &database.AccountMetering{
		EventUUID: event.Uuid,
	}

	mockMeterComp.EXPECT().GetMeteringByEventUUID(ctx, event.Uuid).Return(existingMetering, nil)
	mockAcctEvtComp.EXPECT().AddNewAccountingEvent(ctx, event, true).Return(nil)

	err := metering.logAndVerifyEvent(ctx, event)

	require.Error(t, err)
	require.True(t, errors.Is(err, types.ErrDuplicatedMeterByUUID))
}

func TestMeteringImpl_LogAndVerifyEvent_AddEventError(t *testing.T) {
	mockMeterComp := mockacct.NewMockMeteringComponent(t)
	mockAcctEvtComp := mockacct.NewMockAccountingEventComponent(t)
	metering := &MeteringImpl{
		meterComp:   mockMeterComp,
		acctEvtComp: mockAcctEvtComp,
	}

	ctx := context.Background()
	event := createTestMeteringEvent()
	testErr := errors.New("add event error")

	mockMeterComp.EXPECT().GetMeteringByEventUUID(ctx, event.Uuid).Return(nil, nil)
	mockMeterComp.EXPECT().FindMeteringByCustomerIDAndRecordAtInMin(ctx, event.CustomerID, event.CreatedAt).Return(nil, nil)
	mockAcctEvtComp.EXPECT().AddNewAccountingEvent(ctx, event, false).Return(testErr)

	err := metering.logAndVerifyEvent(ctx, event)

	require.Error(t, err)
}

func TestMeteringImpl_HandleMsgData_Success(t *testing.T) {
	mockMeterComp := mockacct.NewMockMeteringComponent(t)
	mockAcctEvtComp := mockacct.NewMockAccountingEventComponent(t)
	metering := &MeteringImpl{
		meterComp:   mockMeterComp,
		acctEvtComp: mockAcctEvtComp,
	}

	ctx := context.Background()
	event := createTestMeteringEvent()
	data, _ := json.Marshal(event)

	mockMeterComp.EXPECT().GetMeteringByEventUUID(mock.Anything, mock.Anything).Return(nil, nil)
	mockMeterComp.EXPECT().FindMeteringByCustomerIDAndRecordAtInMin(mock.Anything, mock.Anything, mock.Anything).Return(nil, nil)
	mockAcctEvtComp.EXPECT().AddNewAccountingEvent(mock.Anything, mock.Anything, mock.Anything).Return(nil)
	mockMeterComp.EXPECT().SaveMeteringEventRecord(ctx, mock.Anything).Return(nil)

	result, err := metering.handleMsgData(ctx, data)

	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestMeteringImpl_HandleMsgData_ParseError(t *testing.T) {
	mockMeterComp := mockacct.NewMockMeteringComponent(t)
	mockAcctEvtComp := mockacct.NewMockAccountingEventComponent(t)
	metering := &MeteringImpl{
		meterComp:   mockMeterComp,
		acctEvtComp: mockAcctEvtComp,
	}

	ctx := context.Background()
	invalidData := []byte(`{invalid json`)

	_, err := metering.handleMsgData(ctx, invalidData)

	require.Error(t, err)
}

func TestMeteringImpl_HandleMsgData_SaveError(t *testing.T) {
	mockMeterComp := mockacct.NewMockMeteringComponent(t)
	mockAcctEvtComp := mockacct.NewMockAccountingEventComponent(t)
	metering := &MeteringImpl{
		meterComp:   mockMeterComp,
		acctEvtComp: mockAcctEvtComp,
	}

	ctx := context.Background()
	event := createTestMeteringEvent()
	data, _ := json.Marshal(event)
	testErr := errors.New("save error")

	mockMeterComp.EXPECT().GetMeteringByEventUUID(mock.Anything, mock.Anything).Return(nil, nil)
	mockMeterComp.EXPECT().FindMeteringByCustomerIDAndRecordAtInMin(mock.Anything, mock.Anything, mock.Anything).Return(nil, nil)
	mockAcctEvtComp.EXPECT().AddNewAccountingEvent(mock.Anything, mock.Anything, mock.Anything).Return(nil)
	mockMeterComp.EXPECT().SaveMeteringEventRecord(ctx, mock.Anything).Return(testErr)

	_, err := metering.handleMsgData(ctx, data)

	require.Error(t, err)
}

func TestMeteringImpl_PubFeeEventWithReTry_Success_TimeDuration(t *testing.T) {
	mockMQ := mockmq.NewMockMessageQueue(t)
	metering := &MeteringImpl{
		bldMQ: mockMQ,
	}

	event := createTestMeteringEvent()
	event.ValueType = types.TimeDurationMinType
	data, _ := json.Marshal(event)

	mockMQ.EXPECT().Publish(bldmq.FeeSendSubject, data).Return(nil)

	err := metering.pubFeeEventWithReTry(data, event, 3)

	require.NoError(t, err)
}

func TestMeteringImpl_PubFeeEventWithReTry_Success_Token(t *testing.T) {
	mockMQ := mockmq.NewMockMessageQueue(t)
	metering := &MeteringImpl{
		bldMQ: mockMQ,
	}

	event := createTestMeteringEvent()
	event.ValueType = types.TokenNumberType
	data, _ := json.Marshal(event)

	mockMQ.EXPECT().Publish(bldmq.TokenSendSubject, data).Return(nil)

	err := metering.pubFeeEventWithReTry(data, event, 3)

	require.NoError(t, err)
}

func TestMeteringImpl_PubFeeEventWithReTry_Success_Quota(t *testing.T) {
	mockMQ := mockmq.NewMockMessageQueue(t)
	metering := &MeteringImpl{
		bldMQ: mockMQ,
	}

	event := createTestMeteringEvent()
	event.ValueType = types.QuotaNumberType
	data, _ := json.Marshal(event)

	mockMQ.EXPECT().Publish(bldmq.QuotaSendSubject, data).Return(nil)

	err := metering.pubFeeEventWithReTry(data, event, 3)

	require.NoError(t, err)
}

func TestMeteringImpl_PubFeeEventWithReTry_AllRetryFailed(t *testing.T) {
	mockMQ := mockmq.NewMockMessageQueue(t)
	metering := &MeteringImpl{
		bldMQ: mockMQ,
	}

	event := createTestMeteringEvent()
	event.ValueType = types.TimeDurationMinType
	data, _ := json.Marshal(event)
	testErr := errors.New("publish error")

	mockMQ.EXPECT().Publish(bldmq.FeeSendSubject, data).Return(testErr).Times(3)

	err := metering.pubFeeEventWithReTry(data, event, 3)

	require.Error(t, err)
}

func TestMeteringImpl_PubFeeEventWithReTry_UnsupportedType(t *testing.T) {
	mockMQ := mockmq.NewMockMessageQueue(t)
	metering := &MeteringImpl{
		bldMQ: mockMQ,
	}

	event := createTestMeteringEvent()
	event.ValueType = 999
	data, _ := json.Marshal(event)

	err := metering.pubFeeEventWithReTry(data, event, 3)

	require.NoError(t, err)
}

func TestMeteringImpl_MoveMsgToDLQWithReTry_Success(t *testing.T) {
	mockMQ := mockmq.NewMockMessageQueue(t)
	metering := &MeteringImpl{
		bldMQ: mockMQ,
	}

	data := []byte(`test data`)

	mockMQ.EXPECT().Publish(bldmq.DLQMeterSubject, data).Return(nil)

	err := metering.moveMsgToDLQWithReTry(data, 3)

	require.NoError(t, err)
}

func TestMeteringImpl_MoveMsgToDLQWithReTry_AllRetryFailed(t *testing.T) {
	mockMQ := mockmq.NewMockMessageQueue(t)
	metering := &MeteringImpl{
		bldMQ: mockMQ,
	}

	data := []byte(`test data`)
	testErr := errors.New("publish error")

	mockMQ.EXPECT().Publish(bldmq.DLQMeterSubject, data).Return(testErr).Times(3)

	err := metering.moveMsgToDLQWithReTry(data, 3)

	require.Error(t, err)
}

func TestMeteringImpl_HandleMsgWithRetry_Success(t *testing.T) {
	mockMeterComp := mockacct.NewMockMeteringComponent(t)
	mockAcctEvtComp := mockacct.NewMockAccountingEventComponent(t)
	mockMQ := mockmq.NewMockMessageQueue(t)
	metering := &MeteringImpl{
		meterComp:      mockMeterComp,
		acctEvtComp:    mockAcctEvtComp,
		chargingEnable: true,
		bldMQ:          mockMQ,
	}

	event := createTestMeteringEvent()
	data, _ := json.Marshal(event)
	meta := bldmq.MessageMeta{
		Topic: bldmq.MeterDurationSendSubject,
	}

	mockMeterComp.EXPECT().GetMeteringByEventUUID(mock.Anything, mock.Anything).Return(nil, nil)
	mockMeterComp.EXPECT().FindMeteringByCustomerIDAndRecordAtInMin(mock.Anything, mock.Anything, mock.Anything).Return(nil, nil)
	mockAcctEvtComp.EXPECT().AddNewAccountingEvent(mock.Anything, mock.Anything, mock.Anything).Return(nil)
	mockMeterComp.EXPECT().SaveMeteringEventRecord(mock.Anything, mock.Anything).Return(nil)
	mockMQ.EXPECT().Publish(bldmq.FeeSendSubject, data).Return(nil)

	err := metering.handleMsgWithRetry(data, meta)

	require.NoError(t, err)
}

func TestMeteringImpl_HandleMsgWithRetry_ParseError(t *testing.T) {
	mockMeterComp := mockacct.NewMockMeteringComponent(t)
	mockAcctEvtComp := mockacct.NewMockAccountingEventComponent(t)
	mockMQ := mockmq.NewMockMessageQueue(t)
	metering := &MeteringImpl{
		meterComp:      mockMeterComp,
		acctEvtComp:    mockAcctEvtComp,
		chargingEnable: true,
		bldMQ:          mockMQ,
	}

	invalidData := []byte(`{invalid json`)
	meta := bldmq.MessageMeta{
		Topic: bldmq.MeterDurationSendSubject,
	}

	mockMQ.EXPECT().Publish(bldmq.DLQMeterSubject, invalidData).Return(nil)

	err := metering.handleMsgWithRetry(invalidData, meta)

	require.NoError(t, err)
}

func TestMeteringImpl_HandleMsgWithRetry_ChargingDisabled(t *testing.T) {
	mockMeterComp := mockacct.NewMockMeteringComponent(t)
	mockAcctEvtComp := mockacct.NewMockAccountingEventComponent(t)
	mockMQ := mockmq.NewMockMessageQueue(t)
	metering := &MeteringImpl{
		meterComp:      mockMeterComp,
		acctEvtComp:    mockAcctEvtComp,
		chargingEnable: false,
		bldMQ:          mockMQ,
	}

	event := createTestMeteringEvent()
	data, _ := json.Marshal(event)
	meta := bldmq.MessageMeta{
		Topic: bldmq.MeterDurationSendSubject,
	}

	mockMeterComp.EXPECT().GetMeteringByEventUUID(mock.Anything, mock.Anything).Return(nil, nil)
	mockMeterComp.EXPECT().FindMeteringByCustomerIDAndRecordAtInMin(mock.Anything, mock.Anything, mock.Anything).Return(nil, nil)
	mockAcctEvtComp.EXPECT().AddNewAccountingEvent(mock.Anything, mock.Anything, mock.Anything).Return(nil)
	mockMeterComp.EXPECT().SaveMeteringEventRecord(mock.Anything, mock.Anything).Return(nil)

	err := metering.handleMsgWithRetry(data, meta)

	require.NoError(t, err)
}

func TestMeteringImpl_HandleMsgWithRetry_HandleFailed(t *testing.T) {
	mockMeterComp := mockacct.NewMockMeteringComponent(t)
	mockAcctEvtComp := mockacct.NewMockAccountingEventComponent(t)
	mockMQ := mockmq.NewMockMessageQueue(t)
	metering := &MeteringImpl{
		meterComp:      mockMeterComp,
		acctEvtComp:    mockAcctEvtComp,
		chargingEnable: true,
		bldMQ:          mockMQ,
	}

	event := createTestMeteringEvent()
	data, _ := json.Marshal(event)
	meta := bldmq.MessageMeta{
		Topic: bldmq.MeterDurationSendSubject,
	}
	testErr := errors.New("handle error")

	mockMeterComp.EXPECT().GetMeteringByEventUUID(mock.Anything, mock.Anything).Return(nil, nil)
	mockMeterComp.EXPECT().FindMeteringByCustomerIDAndRecordAtInMin(mock.Anything, mock.Anything, mock.Anything).Return(nil, nil)
	mockAcctEvtComp.EXPECT().AddNewAccountingEvent(mock.Anything, mock.Anything, mock.Anything).Return(testErr)
	mockMQ.EXPECT().Publish(bldmq.DLQMeterSubject, data).Return(nil)

	err := metering.handleMsgWithRetry(data, meta)

	require.NoError(t, err)
}

func TestMeteringImpl_HandleMsgWithRetry_PubFeeFailed(t *testing.T) {
	mockMeterComp := mockacct.NewMockMeteringComponent(t)
	mockAcctEvtComp := mockacct.NewMockAccountingEventComponent(t)
	mockMQ := mockmq.NewMockMessageQueue(t)
	metering := &MeteringImpl{
		meterComp:      mockMeterComp,
		acctEvtComp:    mockAcctEvtComp,
		chargingEnable: true,
		bldMQ:          mockMQ,
	}

	event := createTestMeteringEvent()
	data, _ := json.Marshal(event)
	meta := bldmq.MessageMeta{
		Topic: bldmq.MeterDurationSendSubject,
	}
	testErr := errors.New("publish error")

	mockMeterComp.EXPECT().GetMeteringByEventUUID(mock.Anything, mock.Anything).Return(nil, nil)
	mockMeterComp.EXPECT().FindMeteringByCustomerIDAndRecordAtInMin(mock.Anything, mock.Anything, mock.Anything).Return(nil, nil)
	mockAcctEvtComp.EXPECT().AddNewAccountingEvent(mock.Anything, mock.Anything, mock.Anything).Return(nil)
	mockMeterComp.EXPECT().SaveMeteringEventRecord(mock.Anything, mock.Anything).Return(nil)
	mockMQ.EXPECT().Publish(bldmq.FeeSendSubject, data).Return(testErr).Times(3)

	err := metering.handleMsgWithRetry(data, meta)

	require.Error(t, err)
}
