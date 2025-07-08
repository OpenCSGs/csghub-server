package mq

import (
	"opencsg.com/csghub-server/common/config"
)

var (
	SystemMQ MessageQueue
)

func GetOrInit(config *config.Config) (MessageQueue, error) {
	if SystemMQ != nil {
		return SystemMQ, nil
	}
	mq, err := NewNats(config)
	if err != nil {
		return nil, err
	}
	if err := mq.GetJetStream(); err != nil {
		return nil, err
	}
	SystemMQ = mq
	return SystemMQ, nil
}
