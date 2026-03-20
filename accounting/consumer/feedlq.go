//go:build saas

package consumer

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	bldmq "opencsg.com/csghub-server/builder/mq"
)

type FeeDlq interface {
	Run()
}

type FeeDlqImpl struct {
	bldMQ bldmq.MessageQueue
}

func NewFeeDlq(mqFactory bldmq.MessageQueueFactory) (FeeDlq, error) {
	mq, err := mqFactory.GetInstance()
	if err != nil {
		return nil, fmt.Errorf("failed to get message queue instance: %w", err)
	}
	dlq := &FeeDlqImpl{
		bldMQ: mq,
	}
	return dlq, nil
}

func (d *FeeDlqImpl) Run() {
	err := d.bldMQ.Subscribe(bldmq.SubscribeParams{
		Group: bldmq.AccountingDlqGroup,
		Topics: []string{
			bldmq.DLQFeeSubject,
			bldmq.DLQMeterSubject,
			bldmq.DLQRechargeSubject,
		},
		AutoACK:  true,
		Callback: d.handleMsgWithRetry,
	})
	if err != nil {
		slog.Error("failed to subscribe dlq event", slog.Any("error", err))
	}
}

func (d *FeeDlqImpl) handleMsgWithRetry(raw []byte, meta bldmq.MessageMeta) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	slog.DebugContext(ctx, "received dlq event", slog.String("subject", meta.Topic), slog.Any("raw", string(raw)))
	// TODO: Add dlq business logic here later
	return nil
}
