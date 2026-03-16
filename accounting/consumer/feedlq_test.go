//go:build saas

package consumer

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	mockmq "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/mq"
	bldmq "opencsg.com/csghub-server/builder/mq"
)

func TestNewFeeDlq_Success(t *testing.T) {
	mockFactory := mockmq.NewMockMessageQueueFactory(t)
	mockMQ := mockmq.NewMockMessageQueue(t)

	mockFactory.EXPECT().GetInstance().Return(mockMQ, nil)

	dlq, err := NewFeeDlq(mockFactory)

	require.NoError(t, err)
	require.NotNil(t, dlq)
	require.NotNil(t, dlq.(*FeeDlqImpl).bldMQ)
}

func TestNewFeeDlq_GetInstanceError(t *testing.T) {
	mockFactory := mockmq.NewMockMessageQueueFactory(t)
	testErr := errors.New("failed to get instance")

	mockFactory.EXPECT().GetInstance().Return(nil, testErr)

	dlq, err := NewFeeDlq(mockFactory)

	require.Error(t, err)
	require.Nil(t, dlq)
	require.Contains(t, err.Error(), "failed to get message queue instance")
}

func TestFeeDlq_Run_Success(t *testing.T) {
	mockMQ := mockmq.NewMockMessageQueue(t)
	dlq := &FeeDlqImpl{
		bldMQ: mockMQ,
	}

	mockMQ.EXPECT().Subscribe(mock.MatchedBy(func(params bldmq.SubscribeParams) bool {
		return params.Group == bldmq.AccountingDlqGroup &&
			len(params.Topics) == 3 &&
			params.Topics[0] == bldmq.DLQFeeSubject &&
			params.Topics[1] == bldmq.DLQMeterSubject &&
			params.Topics[2] == bldmq.DLQRechargeSubject &&
			params.AutoACK == true &&
			params.Callback != nil
	})).Return(nil)

	dlq.Run()
}

func TestFeeDlq_Run_SubscribeError(t *testing.T) {
	mockMQ := mockmq.NewMockMessageQueue(t)
	dlq := &FeeDlqImpl{
		bldMQ: mockMQ,
	}
	testErr := errors.New("subscribe error")

	mockMQ.EXPECT().Subscribe(mock.Anything).Return(testErr)

	dlq.Run()
}

func TestFeeDlq_HandleMsgWithRetry_Success(t *testing.T) {
	mockMQ := mockmq.NewMockMessageQueue(t)
	dlq := &FeeDlqImpl{
		bldMQ: mockMQ,
	}

	raw := []byte(`{"test": "data"}`)
	meta := bldmq.MessageMeta{
		Topic: bldmq.DLQFeeSubject,
	}

	err := dlq.handleMsgWithRetry(raw, meta)

	require.NoError(t, err)
}
