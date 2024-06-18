package consumer

import (
	"log/slog"
	"time"

	"github.com/nats-io/nats.go"
)

var (
	dlqSubject    string = "accounting.dlq.fee"
	dlqStreamName string = "accountingDlqStream"
)

type Dlq struct {
	cfg     *nats.StreamConfig
	CH      chan []byte
	timeOut *time.Timer
}

func NewDlq() *Dlq {
	dlq := &Dlq{
		cfg: &nats.StreamConfig{
			Name:         dlqStreamName,
			Subjects:     []string{dlqSubject},
			MaxConsumers: -1,
			MaxMsgs:      -1,
			MaxBytes:     -1,
		},
		CH:      make(chan []byte),
		timeOut: time.NewTimer(idleDuration),
	}
	return dlq
}

func (d *Dlq) Run(nc *nats.Conn) {
	for {
		if nc == nil || nc.IsClosed() {
			break
		}
		js, err := nc.JetStream()
		if err != nil {
			slog.Error("fail to get dlq jetstream", slog.Any("err", err))
			continue
		}

		_, err = js.AddStream(d.cfg)
		if err != nil {
			// slog.Warn("fail to add dlq nats stream", slog.Any("streamName", dlqStreamName), slog.Any("err", err))
			_, err = js.UpdateStream(d.cfg)
			if err != nil {
				slog.Warn("fail to update dql nats stream", slog.Any("streamName", dlqStreamName), slog.Any("err", err))
				continue
			}
		}
		d.moveWithRetry(nc, js)
	}
}

func (d *Dlq) moveWithRetry(nc *nats.Conn, js nats.JetStreamContext) {
	for {
		var data []byte = []byte("")
		d.timeOut.Reset(idleDuration)
		select {
		case data = <-d.CH:
		case <-d.timeOut.C:
		}
		if nc == nil || nc.IsClosed() {
			break
		}
		if len(data) < 1 {
			continue
		}
		err := d.retryPublishToDlq(js, data)
		if err != nil {
			break
		}
	}
}

func (d *Dlq) retryPublishToDlq(js nats.JetStreamContext, data []byte) error {
	// max try 10 times
	var err error = nil
	for m := 0; m < 10; m++ {
		err = d.publishToDlq(js, data)
		if err != nil {
			slog.Error("fail to move DLQ", slog.Any("subject", dlqSubject), slog.Any("data", string(data)), slog.Any("error", err))
			continue
		} else {
			break
		}
	}
	return err
}

func (d *Dlq) publishToDlq(js nats.JetStreamContext, data []byte) error {
	_, err := js.Publish(dlqSubject, data)
	return err
}
