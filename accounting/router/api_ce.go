//go:build !ee && !saas

package router

import (
	"github.com/gin-gonic/gin"
	bldmq "opencsg.com/csghub-server/builder/mq"
	"opencsg.com/csghub-server/common/config"
)

func createAdvancedRoutes(apiGroup *gin.RouterGroup, config *config.Config, mqFactory bldmq.MessageQueueFactory) error {
	return nil
}
