package mq

import (
	"opencsg.com/csghub-server/common/config"
)

func Init(config *config.Config) (*NatsHandler, error) {
	sysMQ, err := NewNats(config)
	if err != nil {
		return nil, err
	}
	err = sysMQ.GetJetStream()
	if err != nil {
		return nil, err
	}
	return sysMQ, nil
}
