package router

import (
	"log/slog"

	"github.com/gin-contrib/pprof"
	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/api/middleware"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/notification/handler"
)

func NewNotifierRouter(conf *config.Config) (*gin.Engine, error) {
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.Log(conf))
	needAPIKey := middleware.NeedAPIKey(conf)
	debugGroup := r.Group("/debug", needAPIKey)
	pprof.RouteRegister(debugGroup, "pprof")
	r.Use(middleware.Authenticator(conf))
	notificationsGroup := r.Group("/api/v1/notifications")
	messageHandler, err := handler.NewNotificationHandler(conf)
	if err != nil {
		slog.Error("failed to create notification handler", "error", err)
		return nil, err
	}

	{
		notificationsGroup.GET("/count", messageHandler.GetUnreadCount)
		notificationsGroup.GET("", messageHandler.ListNotifications)
		notificationsGroup.POST("", messageHandler.SendMessage)
		notificationsGroup.DELETE("", messageHandler.DeleteNotifications)
		notificationsGroup.PUT("/read", messageHandler.MarkAsRead)
		notificationsGroup.PUT("/unread", messageHandler.MarkAsUnread)
		notificationsGroup.PUT("/setting", messageHandler.UpdateNotificationSetting)
		notificationsGroup.GET("/setting", messageHandler.GetNotificationSetting)
		notificationsGroup.GET("/poll/:limit", messageHandler.PollNewNotifications)
		notificationsGroup.GET("/message-types", messageHandler.GetAllMessageTypes)
	}

	return r, nil
}
