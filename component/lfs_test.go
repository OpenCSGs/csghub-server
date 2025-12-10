package component

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	bldmq "opencsg.com/csghub-server/builder/mq"
	"opencsg.com/csghub-server/common/config"

	mockmq "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/mq"
	mockdb "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/store/database"
)

func TestLfsComponent_DispatchLfsXnetProcessed(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mockMQ := mockmq.NewMockMessageQueue(t)
		mockLfsStore := mockdb.NewMockLfsMetaObjectStore(t)
		cfg := &config.Config{}
		lfsComp := &lfsComponentImpl{
			cfg:                cfg,
			mq:                 mockMQ,
			lfsMetaObjectStore: mockLfsStore,
		}

		mockMQ.EXPECT().Subscribe(mock.MatchedBy(func(params bldmq.SubscribeParams) bool {
			return params.Group == bldmq.LfsXnetProcessedGroup &&
				len(params.Topics) == 1 &&
				params.Topics[0] == bldmq.LfsXnetProcessedSubject &&
				params.AutoACK == true
		})).Return(nil)

		err := lfsComp.DispatchLfsXnetProcessed()
		assert.NoError(t, err)
	})

	t.Run("failed to subscribe", func(t *testing.T) {
		mockMQ := mockmq.NewMockMessageQueue(t)
		mockLfsStore := mockdb.NewMockLfsMetaObjectStore(t)
		cfg := &config.Config{}
		lfsComp := &lfsComponentImpl{
			cfg:                cfg,
			mq:                 mockMQ,
			lfsMetaObjectStore: mockLfsStore,
		}

		mockMQ.EXPECT().Subscribe(mock.Anything).Return(errors.New("subscribe error"))

		err := lfsComp.DispatchLfsXnetProcessed()
		assert.Error(t, err)
		if err != nil {
			assert.Contains(t, err.Error(), "failed to subscribe lfs xnet processed event")
		}
	})
}

func TestLfsComponent_handleLfsXnetProcessedMsg(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mockMQ := mockmq.NewMockMessageQueue(t)
		mockLfsStore := mockdb.NewMockLfsMetaObjectStore(t)
		cfg := &config.Config{}
		lfsComp := &lfsComponentImpl{
			cfg:                cfg,
			mq:                 mockMQ,
			lfsMetaObjectStore: mockLfsStore,
		}

		msg := LfsXnetProcessedMessage{
			RepoID: 123,
			Oid:    "abc123456",
		}
		raw, _ := json.Marshal(msg)
		meta := bldmq.MessageMeta{Topic: bldmq.LfsXnetProcessedSubject}

		mockLfsStore.EXPECT().UpdateXnetUsed(mock.Anything, msg.RepoID, msg.Oid, true).Return(nil)

		err := lfsComp.handleLfsXnetProcessedMsg(raw, meta)
		assert.NoError(t, err)
	})

	t.Run("unmarshal error", func(t *testing.T) {
		mockMQ := mockmq.NewMockMessageQueue(t)
		mockLfsStore := mockdb.NewMockLfsMetaObjectStore(t)
		cfg := &config.Config{}
		lfsComp := &lfsComponentImpl{
			cfg:                cfg,
			mq:                 mockMQ,
			lfsMetaObjectStore: mockLfsStore,
		}

		raw := []byte("invalid json")
		meta := bldmq.MessageMeta{Topic: bldmq.LfsXnetProcessedSubject}

		err := lfsComp.handleLfsXnetProcessedMsg(raw, meta)
		assert.Error(t, err)
		if err != nil {
			assert.Contains(t, err.Error(), "failed to unmarshal lfs xnet processed message")
		}
	})

	t.Run("update store error", func(t *testing.T) {
		mockMQ := mockmq.NewMockMessageQueue(t)
		mockLfsStore := mockdb.NewMockLfsMetaObjectStore(t)
		cfg := &config.Config{}
		lfsComp := &lfsComponentImpl{
			cfg:                cfg,
			mq:                 mockMQ,
			lfsMetaObjectStore: mockLfsStore,
		}

		msg := LfsXnetProcessedMessage{
			RepoID: 123,
			Oid:    "abc123456",
		}
		raw, _ := json.Marshal(msg)
		meta := bldmq.MessageMeta{Topic: bldmq.LfsXnetProcessedSubject}

		mockLfsStore.EXPECT().UpdateXnetUsed(mock.Anything, msg.RepoID, msg.Oid, true).Return(errors.New("db error"))

		err := lfsComp.handleLfsXnetProcessedMsg(raw, meta)
		assert.Error(t, err)
		if err != nil {
			assert.Contains(t, err.Error(), "failed to update xnet_used")
		}
	})
}

func TestNewLfsComponent(t *testing.T) {
	cfg := &config.Config{}

	t.Run("success", func(t *testing.T) {
		mockMQFactory := mockmq.NewMockMessageQueueFactory(t)
		mockMQ := mockmq.NewMockMessageQueue(t)
		mockMQFactory.EXPECT().GetInstance().Return(mockMQ, nil)

		comp, err := NewLfsComponent(cfg, mockMQFactory)
		assert.NoError(t, err)
		assert.NotNil(t, comp)
	})

	t.Run("mq factory error", func(t *testing.T) {
		mockMQFactory := mockmq.NewMockMessageQueueFactory(t)
		mockMQFactory.EXPECT().GetInstance().Return(nil, errors.New("mq factory error"))

		comp, err := NewLfsComponent(cfg, mockMQFactory)
		assert.Error(t, err)
		assert.Nil(t, comp)
		assert.Contains(t, err.Error(), "failed to get mq instance")
	})
}
