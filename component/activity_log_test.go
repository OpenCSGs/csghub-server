package component

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	mockmq "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/mq"
	mockdb "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/store/database"
	bldmq "opencsg.com/csghub-server/builder/mq"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
)

func resetSingleton() {
	defaultActivityLogComponent = nil
}

func TestNewActivityLogComponent(t *testing.T) {
	t.Run("singleton returns same instance", func(t *testing.T) {
		resetSingleton()
		defer resetSingleton()

		mq := new(mockmq.MockMessageQueue)
		mqFactory := new(mockmq.MockMessageQueueFactory)
		mqFactory.EXPECT().GetInstance().Return(mq, nil)

		c1, err := NewActivityLogComponent(&config.Config{}, mqFactory)
		require.NoError(t, err)
		require.NotNil(t, c1)

		c2, err := NewActivityLogComponent(&config.Config{}, mqFactory)
		require.NoError(t, err)
		require.NotNil(t, c2)

		assert.Same(t, c1, c2)
	})

	t.Run("returns error when mq factory fails", func(t *testing.T) {
		resetSingleton()
		defer resetSingleton()

		mqFactory := new(mockmq.MockMessageQueueFactory)
		mqFactory.EXPECT().GetInstance().Return(nil, assert.AnError)

		c, err := NewActivityLogComponent(&config.Config{}, mqFactory)
		assert.Error(t, err)
		assert.Nil(t, c)
	})
}

func TestPublishActivityLog(t *testing.T) {
	resetSingleton()
	defer resetSingleton()

	mq := new(mockmq.MockMessageQueue)
	mqFactory := new(mockmq.MockMessageQueueFactory)
	mqFactory.EXPECT().GetInstance().Return(mq, nil)

	c, err := NewActivityLogComponent(&config.Config{}, mqFactory)
	require.NoError(t, err)

	log := &types.ActivityLog{
		Username:     "testuser",
		UserID:       "uuid-123",
		Action:       "upload_model_file",
		ResourceType: "models",
		ResourceName: "ns/mymodel",
		IPAddress:    "1.2.3.4",
		UserAgent:    "test-agent",
	}

	data, _ := json.Marshal(log)
	mq.EXPECT().Publish(bldmq.ActivityLogSendSubject, data).Return(nil)

	err = c.PublishActivityLog(context.Background(), log)
	assert.NoError(t, err)
}

func TestStartConsuming(t *testing.T) {
	resetSingleton()
	defer resetSingleton()

	mq := new(mockmq.MockMessageQueue)
	mqFactory := new(mockmq.MockMessageQueueFactory)
	mqFactory.EXPECT().GetInstance().Return(mq, nil)

	c, err := NewActivityLogComponent(&config.Config{}, mqFactory)
	require.NoError(t, err)

	mq.EXPECT().Subscribe(mock.MatchedBy(func(p bldmq.SubscribeParams) bool {
		return p.Group == bldmq.ActivityLogGroup &&
			len(p.Topics) == 1 && p.Topics[0] == bldmq.ActivityLogSendSubject &&
			p.AutoACK == true &&
			p.Callback != nil
	})).Return(nil)

	err = c.StartConsuming()
	assert.NoError(t, err)
}

func TestHandleActivityLogMsg(t *testing.T) {
	impl := &activityLogComponentImpl{
		logCh: make(chan *database.ActivityLog, 10),
	}

	t.Run("valid message", func(t *testing.T) {
		msg := types.ActivityLog{
			Username:      "testuser",
			UserID:        "uuid-123",
			Action:        "deploy_inference",
			ResourceType:  "models",
			ResourceID:    42,
			ResourceName:  "ns/mymodel",
			IPAddress:     "1.2.3.4",
			UserAgent:     "test-agent",
			OperationTime: time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC),
		}
		data, err := json.Marshal(msg)
		require.NoError(t, err)

		err = impl.handleActivityLogMsg(data, bldmq.MessageMeta{Topic: bldmq.ActivityLogSendSubject})
		assert.NoError(t, err)

		select {
		case dbLog := <-impl.logCh:
			assert.Equal(t, "uuid-123", dbLog.UserUUID)
			assert.Equal(t, "testuser", dbLog.Username)
			assert.Equal(t, "deploy_inference", dbLog.Action)
			assert.Equal(t, "models", dbLog.ResourceType)
			assert.Equal(t, int64(42), dbLog.ResourceID)
			assert.Equal(t, "ns/mymodel", dbLog.ResourceName)
			assert.Equal(t, "1.2.3.4", dbLog.IPAddress)
			assert.Equal(t, "test-agent", dbLog.UserAgent)
		default:
			t.Fatal("expected a log to be sent to channel")
		}
	})

	t.Run("invalid json returns error", func(t *testing.T) {
		err := impl.handleActivityLogMsg([]byte("not json"), bldmq.MessageMeta{})
		assert.Error(t, err)
	})
}

func TestDeduplicateLogs(t *testing.T) {
	impl := &activityLogComponentImpl{}

	t.Run("empty slice", func(t *testing.T) {
		result := impl.deduplicateLogs(nil)
		assert.Empty(t, result)
	})

	t.Run("no duplicates", func(t *testing.T) {
		logs := []database.ActivityLog{
			{UserUUID: "u1", Action: "a1", ResourceType: "models", ResourceName: "ns/m1", ResourceID: 1},
			{UserUUID: "u2", Action: "a2", ResourceType: "datasets", ResourceName: "ns/d1", ResourceID: 2},
		}
		result := impl.deduplicateLogs(logs)
		assert.Len(t, result, 2)
	})

	t.Run("removes duplicates by key", func(t *testing.T) {
		logs := []database.ActivityLog{
			{UserUUID: "u1", Action: "upload", ResourceType: "models", ResourceName: "ns/m1", ResourceID: 1},
			{UserUUID: "u1", Action: "upload", ResourceType: "models", ResourceName: "ns/m1", ResourceID: 1},
			{UserUUID: "u2", Action: "download", ResourceType: "models", ResourceName: "ns/m2", ResourceID: 2},
			{UserUUID: "u1", Action: "upload", ResourceType: "models", ResourceName: "ns/m1", ResourceID: 1},
		}
		result := impl.deduplicateLogs(logs)
		assert.Len(t, result, 2)
		assert.Equal(t, "u1", result[0].UserUUID)
		assert.Equal(t, "u2", result[1].UserUUID)
	})

	t.Run("different action means different key", func(t *testing.T) {
		logs := []database.ActivityLog{
			{UserUUID: "u1", Action: "upload", ResourceType: "models", ResourceName: "ns/m1", ResourceID: 1},
			{UserUUID: "u1", Action: "download", ResourceType: "models", ResourceName: "ns/m1", ResourceID: 1},
		}
		result := impl.deduplicateLogs(logs)
		assert.Len(t, result, 2)
	})

	t.Run("different resource id means different key", func(t *testing.T) {
		logs := []database.ActivityLog{
			{UserUUID: "u1", Action: "upload", ResourceType: "models", ResourceName: "ns/m1", ResourceID: 1},
			{UserUUID: "u1", Action: "upload", ResourceType: "models", ResourceName: "ns/m1", ResourceID: 2},
		}
		result := impl.deduplicateLogs(logs)
		assert.Len(t, result, 2)
	})
}

func TestFlushBuffer(t *testing.T) {
	t.Run("empty buffer does nothing", func(t *testing.T) {
		impl := &activityLogComponentImpl{
			logBuffer: make([]database.ActivityLog, 0, activityLogBatchSize),
		}
		// flushBuffer on empty buffer should be a no-op
		impl.flushBuffer()
		assert.Len(t, impl.logBuffer, 0)
	})

	t.Run("flushes and clears buffer", func(t *testing.T) {
		store := new(mockdb.MockActivityLogStore)
		store.EXPECT().BatchCreate(mock.Anything, mock.Anything).Return(nil)

		logs := []database.ActivityLog{
			{UserUUID: "u1", Action: "upload", ResourceType: "models", ResourceName: "ns/m1"},
			{UserUUID: "u2", Action: "download", ResourceType: "datasets", ResourceName: "ns/d1"},
		}

		impl := &activityLogComponentImpl{
			store:       store,
			logBuffer:   logs,
			flushTicker: time.NewTicker(time.Hour),
		}

		impl.flushBuffer()
		assert.Len(t, impl.logBuffer, 0)
	})

	t.Run("deduplicates before batch create", func(t *testing.T) {
		store := new(mockdb.MockActivityLogStore)
		store.EXPECT().BatchCreate(mock.Anything, mock.MatchedBy(func(logs []database.ActivityLog) bool {
			return len(logs) == 1
		})).Return(nil)

		logs := []database.ActivityLog{
			{UserUUID: "u1", Action: "upload", ResourceType: "models", ResourceName: "ns/m1"},
			{UserUUID: "u1", Action: "upload", ResourceType: "models", ResourceName: "ns/m1"},
		}

		impl := &activityLogComponentImpl{
			store:       store,
			logBuffer:   logs,
			flushTicker: time.NewTicker(time.Hour),
		}

		impl.flushBuffer()
		assert.Len(t, impl.logBuffer, 0)
	})
}

func TestListActivityLogs(t *testing.T) {
	resetSingleton()
	defer resetSingleton()

	store := new(mockdb.MockActivityLogStore)
	expectedLogs := []database.ActivityLog{
		{UserUUID: "u1", Action: "upload", ResourceType: "models", ResourceName: "ns/m1"},
	}
	expectedTotal := 1

	store.EXPECT().FindByTimeAfter(mock.Anything, mock.Anything, 10, 1).Return(expectedLogs, expectedTotal, nil)

	// Inject mock store into the component
	mq := new(mockmq.MockMessageQueue)
	mqFactory := new(mockmq.MockMessageQueueFactory)
	mqFactory.EXPECT().GetInstance().Return(mq, nil)

	c, err := NewActivityLogComponent(&config.Config{}, mqFactory)
	require.NoError(t, err)
	impl := c.(*activityLogComponentImpl)
	impl.store = store

	req := types.QueryActivityLogReq{
		After: time.Now(),
		Per:   10,
		Page:  1,
	}
	logs, total, err := c.ListActivityLogs(context.Background(), req)
	assert.NoError(t, err)
	assert.Equal(t, expectedLogs, logs)
	assert.Equal(t, expectedTotal, total)
}
