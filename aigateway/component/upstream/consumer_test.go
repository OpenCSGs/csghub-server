package upstream

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	mockupstream "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/aigateway/component/upstream"
	mockmq "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/mq"
	bldmq "opencsg.com/csghub-server/builder/mq"
	commontypes "opencsg.com/csghub-server/common/types"
	"github.com/stretchr/testify/mock"
)

// noopAcker is a test double for bldmq.MessageAcker that records ACK/NAK calls.
type noopAcker struct {
	acked bool
	naked bool
}

func (a *noopAcker) Ack() error      { a.acked = true; return nil }
func (a *noopAcker) Nak() error      { a.naked = true; return nil }
func (a *noopAcker) InProgress() error { return nil }

func TestUpstreamSyncConsumer_Start(t *testing.T) {
	mockMQ := mockmq.NewMockMessageQueue(t)
	mockSync := mockupstream.NewMockAIGatewayUpstreamSyncComponent(t)

	consumer := NewUpstreamSyncConsumer(mockMQ, mockSync)

	mockMQ.EXPECT().Subscribe(mock.MatchedBy(func(p bldmq.SubscribeParams) bool {
		return p.Group.StreamName == bldmq.DeployUpstreamSyncGroup.StreamName &&
			len(p.Topics) == 3 &&
			p.Topics[0] == bldmq.DeployUpstreamSyncRunningSubject &&
			p.Topics[1] == bldmq.DeployUpstreamSyncStopSubject &&
			p.Topics[2] == bldmq.DeployUpstreamSyncDeleteSubject &&
			p.AutoACK == false &&
			p.IsRedeliverForCBFailed == true &&
			p.MaxDeliver == 5 &&
			p.AckWait == 30*time.Second
	})).Return(nil)

	err := consumer.Start()
	require.NoError(t, err)
}

func TestUpstreamSyncConsumer_HandleRunningEvent(t *testing.T) {
	mockMQ := mockmq.NewMockMessageQueue(t)
	mockSync := mockupstream.NewMockAIGatewayUpstreamSyncComponent(t)

	var callback bldmq.MessageCallback
	mockMQ.EXPECT().Subscribe(mock.MatchedBy(func(p bldmq.SubscribeParams) bool {
		callback = p.Callback
		return true
	})).Return(nil)

	consumer := NewUpstreamSyncConsumer(mockMQ, mockSync)
	require.NoError(t, consumer.Start())
	require.NotNil(t, callback)

	event := commontypes.DeployUpstreamSyncEvent{
		DeployID: 42,
		Deploy: &commontypes.DeployUpstreamInfo{
			DeployID:      42,
			RepoPath:      "ns/model-a",
			SvcName:       "svc-42",
			LegacyModelID: "ns/model-a:16",
		},
	}
	raw, _ := json.Marshal(event)

	mockSync.EXPECT().SyncRunningDeploy(mock.Anything, mock.MatchedBy(func(i *commontypes.DeployUpstreamInfo) bool {
		return i != nil && i.DeployID == 42
	})).Return(nil)

	acker := &noopAcker{}
	err := callback(raw, bldmq.MessageMeta{Topic: bldmq.DeployUpstreamSyncRunningSubject, Acker: acker})
	require.NoError(t, err)
	require.True(t, acker.acked, "message should be ACK'd on success")
}

func TestUpstreamSyncConsumer_HandleStopEvent(t *testing.T) {
	mockMQ := mockmq.NewMockMessageQueue(t)
	mockSync := mockupstream.NewMockAIGatewayUpstreamSyncComponent(t)

	var callback bldmq.MessageCallback
	mockMQ.EXPECT().Subscribe(mock.MatchedBy(func(p bldmq.SubscribeParams) bool {
		callback = p.Callback
		return true
	})).Return(nil)

	consumer := NewUpstreamSyncConsumer(mockMQ, mockSync)
	require.NoError(t, consumer.Start())
	require.NotNil(t, callback)

	event := commontypes.DeployUpstreamSyncEvent{DeployID: 99}
	raw, _ := json.Marshal(event)

	mockSync.EXPECT().DisableDeployTarget(mock.Anything, int64(99)).Return(nil)

	acker := &noopAcker{}
	err := callback(raw, bldmq.MessageMeta{Topic: bldmq.DeployUpstreamSyncStopSubject, Acker: acker})
	require.NoError(t, err)
	require.True(t, acker.acked, "message should be ACK'd on success")
}

func TestUpstreamSyncConsumer_HandleDeleteEvent(t *testing.T) {
	mockMQ := mockmq.NewMockMessageQueue(t)
	mockSync := mockupstream.NewMockAIGatewayUpstreamSyncComponent(t)

	var callback bldmq.MessageCallback
	mockMQ.EXPECT().Subscribe(mock.MatchedBy(func(p bldmq.SubscribeParams) bool {
		callback = p.Callback
		return true
	})).Return(nil)

	consumer := NewUpstreamSyncConsumer(mockMQ, mockSync)
	require.NoError(t, consumer.Start())
	require.NotNil(t, callback)

	event := commontypes.DeployUpstreamSyncEvent{DeployID: 77}
	raw, _ := json.Marshal(event)

	mockSync.EXPECT().DeleteDeployTarget(mock.Anything, int64(77)).Return(nil)

	acker := &noopAcker{}
	err := callback(raw, bldmq.MessageMeta{Topic: bldmq.DeployUpstreamSyncDeleteSubject, Acker: acker})
	require.NoError(t, err)
	require.True(t, acker.acked, "message should be ACK'd on success")
}

func TestUpstreamSyncConsumer_HandleUnknownTopic(t *testing.T) {
	mockMQ := mockmq.NewMockMessageQueue(t)
	mockSync := mockupstream.NewMockAIGatewayUpstreamSyncComponent(t)

	var callback bldmq.MessageCallback
	mockMQ.EXPECT().Subscribe(mock.MatchedBy(func(p bldmq.SubscribeParams) bool {
		callback = p.Callback
		return true
	})).Return(nil)

	consumer := NewUpstreamSyncConsumer(mockMQ, mockSync)
	require.NoError(t, consumer.Start())
	require.NotNil(t, callback)

	event := commontypes.DeployUpstreamSyncEvent{DeployID: 1}
	raw, _ := json.Marshal(event)

	// Unknown topic — should return nil and ACK without calling any sync method.
	acker := &noopAcker{}
	err := callback(raw, bldmq.MessageMeta{Topic: "unknown.topic", Acker: acker})
	require.NoError(t, err)
	require.True(t, acker.acked, "unknown topic should still be ACK'd")
}

func TestUpstreamSyncConsumer_HandleInvalidJSON(t *testing.T) {
	mockMQ := mockmq.NewMockMessageQueue(t)
	mockSync := mockupstream.NewMockAIGatewayUpstreamSyncComponent(t)

	var callback bldmq.MessageCallback
	mockMQ.EXPECT().Subscribe(mock.MatchedBy(func(p bldmq.SubscribeParams) bool {
		callback = p.Callback
		return true
	})).Return(nil)

	consumer := NewUpstreamSyncConsumer(mockMQ, mockSync)
	require.NoError(t, consumer.Start())
	require.NotNil(t, callback)

	// Invalid JSON — should return nil and ACK (poison pill protection).
	acker := &noopAcker{}
	err := callback([]byte("not json"), bldmq.MessageMeta{Topic: bldmq.DeployUpstreamSyncRunningSubject, Acker: acker})
	require.NoError(t, err)
	require.True(t, acker.acked, "invalid JSON should be ACK'd to avoid infinite redelivery")
}

func TestUpstreamSyncConsumer_HandleSyncError(t *testing.T) {
	mockMQ := mockmq.NewMockMessageQueue(t)
	mockSync := mockupstream.NewMockAIGatewayUpstreamSyncComponent(t)

	var callback bldmq.MessageCallback
	mockMQ.EXPECT().Subscribe(mock.MatchedBy(func(p bldmq.SubscribeParams) bool {
		callback = p.Callback
		return true
	})).Return(nil)

	consumer := NewUpstreamSyncConsumer(mockMQ, mockSync)
	require.NoError(t, consumer.Start())
	require.NotNil(t, callback)

	event := commontypes.DeployUpstreamSyncEvent{
		DeployID: 55,
		Deploy: &commontypes.DeployUpstreamInfo{
			DeployID:      55,
			RepoPath:      "ns/model-b",
			SvcName:       "svc-55",
			LegacyModelID: "ns/model-b:1j",
		},
	}
	raw, _ := json.Marshal(event)

	// SyncRunningDeploy returns an error — callback should return the error
	// so the MQ layer NAKs the message for redelivery.
	mockSync.EXPECT().SyncRunningDeploy(mock.Anything, mock.MatchedBy(func(i *commontypes.DeployUpstreamInfo) bool {
		return i != nil && i.DeployID == 55
	})).Return(assertError("db error"))

	acker := &noopAcker{}
	err := callback(raw, bldmq.MessageMeta{Topic: bldmq.DeployUpstreamSyncRunningSubject, Acker: acker})
	require.Error(t, err, "sync error should propagate for NAK/redelivery")
	require.False(t, acker.acked, "message should NOT be ACK'd on sync error")
}

// assertError is a helper to create a simple error for test expectations.
func assertError(msg string) error {
	return &testError{msg: msg}
}

type testError struct{ msg string }

func (e *testError) Error() string { return e.msg }

func TestUpstreamSyncConsumer_RejectStaleRunningEvent(t *testing.T) {
	mockMQ := mockmq.NewMockMessageQueue(t)
	mockSync := mockupstream.NewMockAIGatewayUpstreamSyncComponent(t)

	var callback bldmq.MessageCallback
	mockMQ.EXPECT().Subscribe(mock.MatchedBy(func(p bldmq.SubscribeParams) bool {
		callback = p.Callback
		return true
	})).Return(nil)

	consumer := NewUpstreamSyncConsumer(mockMQ, mockSync)
	require.NoError(t, consumer.Start())
	require.NotNil(t, callback)

	// First: a stop event at t=100 for deploy 42.
	stopEvent := commontypes.DeployUpstreamSyncEvent{DeployID: 42, EventTime: 100}
	stopRaw, _ := json.Marshal(stopEvent)
	mockSync.EXPECT().DisableDeployTarget(mock.Anything, int64(42)).Return(nil)
	acker1 := &noopAcker{}
	err := callback(stopRaw, bldmq.MessageMeta{Topic: bldmq.DeployUpstreamSyncStopSubject, Acker: acker1})
	require.NoError(t, err)
	require.True(t, acker1.acked, "stop event should be ACK'd")

	// Second: a stale running event at t=50 (earlier) for the same deploy.
	// This should be discarded — SyncRunningDeploy must NOT be called.
	runningEvent := commontypes.DeployUpstreamSyncEvent{
		DeployID:  42,
		EventTime: 50,
		Deploy: &commontypes.DeployUpstreamInfo{
			DeployID:      42,
			RepoPath:      "ns/model-a",
			LegacyModelID: "ns/model-a:16",
		},
	}
	runningRaw, _ := json.Marshal(runningEvent)

	acker2 := &noopAcker{}
	err = callback(runningRaw, bldmq.MessageMeta{Topic: bldmq.DeployUpstreamSyncRunningSubject, Acker: acker2})
	require.NoError(t, err)
	require.True(t, acker2.acked, "stale event should still be ACK'd (discarded, not redelivered)")
}

func TestUpstreamSyncConsumer_AllowNewerRunningEvent(t *testing.T) {
	mockMQ := mockmq.NewMockMessageQueue(t)
	mockSync := mockupstream.NewMockAIGatewayUpstreamSyncComponent(t)

	var callback bldmq.MessageCallback
	mockMQ.EXPECT().Subscribe(mock.MatchedBy(func(p bldmq.SubscribeParams) bool {
		callback = p.Callback
		return true
	})).Return(nil)

	consumer := NewUpstreamSyncConsumer(mockMQ, mockSync)
	require.NoError(t, consumer.Start())
	require.NotNil(t, callback)

	// First: a stop event at t=100 for deploy 42.
	stopEvent := commontypes.DeployUpstreamSyncEvent{DeployID: 42, EventTime: 100}
	stopRaw, _ := json.Marshal(stopEvent)
	mockSync.EXPECT().DisableDeployTarget(mock.Anything, int64(42)).Return(nil)
	acker1 := &noopAcker{}
	_ = callback(stopRaw, bldmq.MessageMeta{Topic: bldmq.DeployUpstreamSyncStopSubject, Acker: acker1})

	// Second: a newer running event at t=200 for the same deploy.
	// This should be processed normally.
	runningEvent := commontypes.DeployUpstreamSyncEvent{
		DeployID:  42,
		EventTime: 200,
		Deploy: &commontypes.DeployUpstreamInfo{
			DeployID:      42,
			RepoPath:      "ns/model-a",
			LegacyModelID: "ns/model-a:16",
		},
	}
	runningRaw, _ := json.Marshal(runningEvent)

	mockSync.EXPECT().SyncRunningDeploy(mock.Anything, mock.MatchedBy(func(i *commontypes.DeployUpstreamInfo) bool {
		return i != nil && i.DeployID == 42
	})).Return(nil)

	acker2 := &noopAcker{}
	err := callback(runningRaw, bldmq.MessageMeta{Topic: bldmq.DeployUpstreamSyncRunningSubject, Acker: acker2})
	require.NoError(t, err)
	require.True(t, acker2.acked, "newer event should be ACK'd after processing")
}

func TestUpstreamSyncConsumer_ZeroEventTimeBackwardCompat(t *testing.T) {
	mockMQ := mockmq.NewMockMessageQueue(t)
	mockSync := mockupstream.NewMockAIGatewayUpstreamSyncComponent(t)

	var callback bldmq.MessageCallback
	mockMQ.EXPECT().Subscribe(mock.MatchedBy(func(p bldmq.SubscribeParams) bool {
		callback = p.Callback
		return true
	})).Return(nil)

	consumer := NewUpstreamSyncConsumer(mockMQ, mockSync)
	require.NoError(t, consumer.Start())
	require.NotNil(t, callback)

	// Event with EventTime=0 (old publisher without timestamp) should always
	// be processed, never treated as stale.
	event := commontypes.DeployUpstreamSyncEvent{
		DeployID:  42,
		EventTime: 0,
		Deploy: &commontypes.DeployUpstreamInfo{
			DeployID:      42,
			RepoPath:      "ns/model-a",
			LegacyModelID: "ns/model-a:16",
		},
	}
	raw, _ := json.Marshal(event)

	mockSync.EXPECT().SyncRunningDeploy(mock.Anything, mock.MatchedBy(func(i *commontypes.DeployUpstreamInfo) bool {
		return i != nil && i.DeployID == 42
	})).Return(nil)

	acker := &noopAcker{}
	err := callback(raw, bldmq.MessageMeta{Topic: bldmq.DeployUpstreamSyncRunningSubject, Acker: acker})
	require.NoError(t, err)
	require.True(t, acker.acked, "event with zero EventTime should be processed normally")
}
