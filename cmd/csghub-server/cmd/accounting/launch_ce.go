//go:build !ee && !saas

package accounting

import (
	bldmq "opencsg.com/csghub-server/builder/mq"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/mq"
)

func createAdvancedConsumer(cfg *config.Config, mqHandler mq.MessageQueue, mqFactory bldmq.MessageQueueFactory) error {
	return nil
}
