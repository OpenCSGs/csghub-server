//go:build !ee && !saas

package router

import (
	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/mq"
)

func createAdvancedRoutes(apiGroup *gin.RouterGroup, config *config.Config, mqHandler mq.MessageQueue) error {
	return nil
}
