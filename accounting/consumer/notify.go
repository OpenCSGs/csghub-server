package consumer

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/nats-io/nats.go"
	"opencsg.com/csghub-server/common/types"
)

var (
	notifyStreamName string = "accountingNotifyStream"
)

type Notify struct {
	cfg                    *nats.StreamConfig
	notifyNoBalanceSubject string
	CH                     chan types.ACC_NOTIFY
	timeOut                *time.Timer
}

func NewNotify(nnbs string) *Notify {
	notify := &Notify{
		cfg: &nats.StreamConfig{
			Name:         notifyStreamName,
			Subjects:     []string{nnbs},
			MaxConsumers: -1,
			MaxMsgs:      -1,
			MaxBytes:     -1,
		},
		notifyNoBalanceSubject: nnbs,
		CH:                     make(chan types.ACC_NOTIFY),
		timeOut:                time.NewTimer(idleDuration),
	}
	return notify
}

func (n *Notify) Run(nc *nats.Conn) {
	for {
		if nc == nil || nc.IsClosed() {
			break
		}
		js, err := nc.JetStream()
		if err != nil {
			slog.Error("fail to get notify jetstream", slog.Any("err", err))
			continue
		}

		_, err = js.AddStream(n.cfg)
		if err != nil {
			// slog.Warn("fail to add notify nats stream", slog.Any("streamName", notifyStreamName), slog.Any("err", err))
			_, err = js.UpdateStream(n.cfg)
			if err != nil {
				slog.Warn("fail to update notify nats stream", slog.Any("streamName", notifyStreamName), slog.Any("err", err))
				continue
			}
		}
		n.notifyWithRetry(nc, js)
	}
}

func (n *Notify) notifyWithRetry(nc *nats.Conn, js nats.JetStreamContext) {
	for {
		var notify types.ACC_NOTIFY = types.ACC_NOTIFY{ReasonCode: -1}
		n.timeOut.Reset(idleDuration)
		select {
		case notify = <-n.CH:
		case <-n.timeOut.C:
		}
		if nc == nil || nc.IsClosed() {
			break
		}
		err := n.checkNotifyStream(js)
		if err != nil {
			slog.Error("fail to check notify stream", slog.Any("error", err))
			break
		}
		if notify.ReasonCode == -1 {
			continue
		}
		err = n.retryPublishNotification(js, notify)
		if err != nil {
			break
		}
	}
}

func (n *Notify) checkNotifyStream(js nats.JetStreamContext) error {
	info, err := js.StreamInfo(dlqStreamName)
	if err != nil {
		return fmt.Errorf("fail to get stream %s info", notifyStreamName)
	}
	if info == nil {
		return fmt.Errorf("stream %s lost", notifyStreamName)
	}
	return nil
}

func (n *Notify) retryPublishNotification(js nats.JetStreamContext, notify types.ACC_NOTIFY) error {
	// max try 10 times
	var err error = nil
	for m := 0; m < 10; m++ {
		err = n.publishNotification(js, notify)
		if err != nil {
			slog.Error("fail to pub notify", slog.Any("subject", n.notifyNoBalanceSubject), slog.Any("notify", notify), slog.Any("error", err))
			continue
		} else {
			break
		}
	}
	return err
}

func (n *Notify) publishNotification(js nats.JetStreamContext, notify types.ACC_NOTIFY) error {
	str, err := json.Marshal(notify)
	if err != nil {
		return fmt.Errorf("fail to unmarshal for notify, %v, %w", notify, err)
	}
	_, err = js.Publish(n.notifyNoBalanceSubject, str)
	if err != nil {
		return fmt.Errorf("fail to publish notification, %s, %v, %w", n.notifyNoBalanceSubject, notify, err)
	}
	return nil
}
