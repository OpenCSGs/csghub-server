package consumer

import (
	"fmt"
	"log/slog"
	"time"

	"opencsg.com/csghub-server/mq"
)

type FeeDlq struct {
	sysMQ   *mq.NatsHandler
	CH      chan []byte
	timeOut *time.Timer
}

func NewFeeDlq(mqh *mq.NatsHandler) *FeeDlq {
	dlq := &FeeDlq{
		sysMQ:   mqh,
		CH:      make(chan []byte),
		timeOut: time.NewTimer(idleDuration),
	}
	return dlq
}

func (d *FeeDlq) Run() {
	for {
		d.preDLQ()
		d.moveWithRetry()
	}
}

func (d *FeeDlq) preDLQ() {
	var err error = nil
	var i int = 0
	for {
		i++
		err = d.sysMQ.BuildDLQStream()
		if err != nil {
			tip := fmt.Sprintf("fail to build DLQ stream in accounting for the %d time", i)
			slog.Error(tip, slog.Any("error", err))
			time.Sleep(2 * time.Second)
			continue
		}
		break
	}
}

func (d *FeeDlq) moveWithRetry() {
	for {
		err := d.sysMQ.VerifyDLQStream()
		if err != nil {
			slog.Error("fail to verify DLQ stream", slog.Any("error", err))
			break
		}

		var data []byte = []byte("")
		d.timeOut.Reset(idleDuration)
		select {
		case data = <-d.CH:
		case <-d.timeOut.C:
		}

		if len(data) < 1 {
			continue
		}
		err = d.publishToDLQWithRetry(data)
		if err != nil {
			break
		}
	}
}

func (d *FeeDlq) publishToDLQWithRetry(data []byte) error {
	// max try 10 times
	var err error = nil
	for m := 0; m < 10; m++ {
		err = d.sysMQ.PublishFeeDataToDLQ(data)
		if err == nil {
			break
		}
	}
	if err != nil {
		slog.Error("fail to move DLQ with retry 10 times", slog.Any("data", string(data)), slog.Any("error", err))
	}
	return err
}
