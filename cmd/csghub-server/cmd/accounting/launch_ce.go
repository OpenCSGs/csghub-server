//go:build !ee && !saas

package accounting

import (
	bldmq "opencsg.com/csghub-server/builder/mq"
	"opencsg.com/csghub-server/common/config"
)

func createAdvancedConsumer(cfg *config.Config, mqFactory bldmq.MessageQueueFactory) error {
	return nil
}
