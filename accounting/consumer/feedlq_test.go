package consumer

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	mockmq "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/mq"
	"opencsg.com/csghub-server/mq"
)

func NewTestFeeDlq(mqh mq.MessageQueue) *FeeDlq {
	dlq := &FeeDlq{
		sysMQ:   mqh,
		CH:      make(chan []byte),
		timeOut: time.NewTimer(dlqIdleDuration),
	}
	return dlq
}

func TestFeeDLQ_preDLQ(t *testing.T) {
	mq := mockmq.NewMockMessageQueue(t)
	mq.EXPECT().BuildDLQStream().Return(nil)

	dlq := NewTestFeeDlq(mq)

	dlq.preDLQ()
}

func TestFeeDLQ_moveWithRetry(t *testing.T) {

	success := []byte("success")
	fail := []byte("fail")

	mq := mockmq.NewMockMessageQueue(t)
	mq.EXPECT().VerifyDLQStream().Return(nil)
	mq.EXPECT().PublishFeeDataToDLQ(success).Return(nil)
	mq.EXPECT().PublishFeeDataToDLQ(fail).Return(errors.New("fail"))

	dlq := NewTestFeeDlq(mq)

	done := make(chan bool)
	go func() {
		dlq.moveWithRetry()
		close(done)
	}()

	dlq.CH <- success
	dlq.CH <- fail
	<-done
}

func TestFeeDLQ_publishToDLQWithRetry(t *testing.T) {
	data := []byte("test")

	mq := mockmq.NewMockMessageQueue(t)
	mq.EXPECT().PublishFeeDataToDLQ(data).Return(nil)

	dlq := NewTestFeeDlq(mq)
	err := dlq.publishToDLQWithRetry(data)
	require.Nil(t, err)
}
