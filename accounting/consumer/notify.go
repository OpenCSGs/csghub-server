package consumer

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/mq"
)

type Notify struct {
	sysMQ   *mq.NatsHandler
	CH      chan types.ACC_NOTIFY
	timeOut *time.Timer
}

func NewNotify(mqh *mq.NatsHandler) *Notify {
	notify := &Notify{
		sysMQ:   mqh,
		CH:      make(chan types.ACC_NOTIFY),
		timeOut: time.NewTimer(idleDuration),
	}
	return notify
}

func (n *Notify) Run() {
	for {
		n.preNotify()
		n.notifyWithRetry()
	}
}

func (n *Notify) preNotify() {
	var err error = nil
	var i int = 0
	for {
		i++
		err = n.sysMQ.BuildNotifyStream()
		if err != nil {
			tip := fmt.Sprintf("fail to build notify stream for the %d time", i)
			slog.Error(tip, slog.Any("err", err))
			time.Sleep(2 * time.Second)
			continue
		}
		break
	}
}

func (n *Notify) notifyWithRetry() {
	for {
		err := n.sysMQ.VerifyNotifyStream()
		if err != nil {
			slog.Error("fail to verify notify stream", slog.Any("error", err))
			break
		}

		var notify types.ACC_NOTIFY = types.ACC_NOTIFY{ReasonCode: -1}
		n.timeOut.Reset(idleDuration)
		select {
		case notify = <-n.CH:
		case <-n.timeOut.C:
		}

		if notify.ReasonCode == -1 {
			continue
		}
		err = n.publishNotificationWithRetry(notify)
		if err != nil {
			break
		}
	}
}

func (n *Notify) publishNotificationWithRetry(notify types.ACC_NOTIFY) error {
	// max try 5 times
	var err error = nil
	for m := 0; m < 5; m++ {
		err = n.publishNotification(notify)
		if err == nil {
			break
		}
	}

	if err != nil {
		slog.Error("fail to pub notify with retry 5 times", slog.Any("notify", notify), slog.Any("error", err))
	}
	return err
}

func (n *Notify) publishNotification(notify types.ACC_NOTIFY) error {
	str, err := json.Marshal(notify)
	if err != nil {
		return fmt.Errorf("fail to unmarshal for notification of lack balance, %v, %w", notify, err)
	}
	err = n.sysMQ.PublishNotificationForNoBalance(str)
	if err != nil {
		return fmt.Errorf("fail to publish notification of lack balance, %v, %w", notify, err)
	}
	return nil
}
