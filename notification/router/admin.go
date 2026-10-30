package router

import (
	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/notification/handler"
)

// adminNotificationHandler documents independently authorized notification operations.
type adminNotificationHandler struct{ target gin.HandlerFunc }

// createAdminRoutes authorizes the same marketing permission as the API server.
func createAdminRoutes(group *gin.RouterGroup, platformAdmin func(...types.AdminPermission) gin.HandlerFunc, messages *handler.NotificationHandler) {
	group.POST("/admin/marketing/email_sending/notifications", platformAdmin(types.AdminMarketingEmailSending), (&adminNotificationHandler{target: messages.SendMessage}).Send)
}

// Send publishes an administrator notification after local module authorization.
// @Security ApiKey
// @Summary Send an administrator email notification
// @Description Requires marketing/email_sending permission, independently checked by this service.
// @Tags Admin marketing email_sending
// @Accept json
// @Produce json
// @Param body body types.MessageRequest true "Notification scenario and parameters"
// @Success 200 {object} types.Response{}
// @Failure 400 {object} types.APIBadRequest
// @Failure 401 {object} types.APIUnauthorized
// @Failure 403 {object} types.APIForbidden
// @Failure 500 {object} types.APIInternalServerError
// @Router /admin/marketing/email_sending/notifications [post]
func (h *adminNotificationHandler) Send(ctx *gin.Context) { h.target(ctx) }

// createAdminCommonNotificationRoutes independently checks membership without widening personal notification scope.
func createAdminCommonNotificationRoutes(group *gin.RouterGroup, platformAdmin func(...types.AdminPermission) gin.HandlerFunc, messages *handler.NotificationHandler) {
	common := group.Group("/admin/common/notifications", platformAdmin(types.AdminPermissionAny))
	common.GET("/message-types", (&adminNotificationHandler{target: messages.GetAllMessageTypes}).CommonNotificationTypes)
	common.GET("/setting", (&adminNotificationHandler{target: messages.GetNotificationSetting}).CommonNotificationSetting)
	common.PUT("/setting", (&adminNotificationHandler{target: messages.UpdateNotificationSetting}).CommonUpdateNotificationSetting)
	common.GET("/count", (&adminNotificationHandler{target: messages.GetUnreadCount}).CommonNotificationCount)
	common.GET("/poll/:limit", (&adminNotificationHandler{target: messages.PollNewNotifications}).CommonNotificationPoll)
	common.GET("", (&adminNotificationHandler{target: messages.ListNotifications}).CommonNotifications)
	common.PUT("/read", (&adminNotificationHandler{target: messages.MarkAsRead}).CommonReadNotifications)
	common.DELETE("", (&adminNotificationHandler{target: messages.DeleteNotifications}).CommonDeleteNotifications)
}

// CommonNotificationTypes independently authorized entry preserves the existing personal notification handler.
// @Security ApiKey
// @Summary List notification types for the administrator navigation bar
// @Description Requires administrator membership independently checked by the notification service; the original current-user scope and response are preserved.
// @Tags Admin Common
// @Produce json
// @Success 200 {object} types.Response{data=[]string}
// @Failure 400 {object} types.APIBadRequest
// @Failure 401 {object} types.APIUnauthorized
// @Failure 403 {object} types.APIForbidden
// @Failure 500 {object} types.APIInternalServerError
// @Router /admin/common/notifications/message-types [get]
func (h *adminNotificationHandler) CommonNotificationTypes(ctx *gin.Context) { h.target(ctx) }

// CommonNotificationSetting independently authorized entry preserves the existing personal notification handler.
// @Security ApiKey
// @Summary Get the current administrator user's notification settings
// @Description Requires administrator membership independently checked by the notification service; the original current-user scope and response are preserved.
// @Tags Admin Common
// @Produce json
// @Param timezone query string false "Timezone" default(Asia/Shanghai)
// @Success 200 {object} types.Response{data=types.NotificationSetting}
// @Failure 400 {object} types.APIBadRequest
// @Failure 401 {object} types.APIUnauthorized
// @Failure 403 {object} types.APIForbidden
// @Failure 500 {object} types.APIInternalServerError
// @Router /admin/common/notifications/setting [get]
func (h *adminNotificationHandler) CommonNotificationSetting(ctx *gin.Context) { h.target(ctx) }

// CommonUpdateNotificationSetting independently authorized entry preserves the existing personal notification handler.
// @Security ApiKey
// @Summary Update the current administrator user's notification settings
// @Description Requires administrator membership independently checked by the notification service; the original current-user scope and response are preserved.
// @Tags Admin Common
// @Accept json
// @Produce json
// @Param body body types.UpdateNotificationReq true "Notification settings"
// @Success 200 {object} types.Response{data=nil}
// @Failure 400 {object} types.APIBadRequest
// @Failure 401 {object} types.APIUnauthorized
// @Failure 403 {object} types.APIForbidden
// @Failure 500 {object} types.APIInternalServerError
// @Router /admin/common/notifications/setting [put]
func (h *adminNotificationHandler) CommonUpdateNotificationSetting(ctx *gin.Context) { h.target(ctx) }

// CommonNotificationCount independently authorized entry preserves the existing personal notification handler.
// @Security ApiKey
// @Summary Count the current administrator user's unread notifications
// @Description Requires administrator membership independently checked by the notification service; the original current-user scope and response are preserved.
// @Tags Admin Common
// @Produce json
// @Success 200 {object} types.Response{data=int}
// @Failure 400 {object} types.APIBadRequest
// @Failure 401 {object} types.APIUnauthorized
// @Failure 403 {object} types.APIForbidden
// @Failure 500 {object} types.APIInternalServerError
// @Router /admin/common/notifications/count [get]
func (h *adminNotificationHandler) CommonNotificationCount(ctx *gin.Context) { h.target(ctx) }

// CommonNotificationPoll independently authorized entry preserves the existing personal notification handler.
// @Security ApiKey
// @Summary Poll the current administrator user's new notifications
// @Description Requires administrator membership independently checked by the notification service; the original current-user scope and response are preserved.
// @Tags Admin Common
// @Produce json
// @Param limit path int true "Maximum number of notifications"
// @Param timezone query string false "Timezone" default(Asia/Shanghai)
// @Success 200 {object} types.Response{data=types.NewNotifications}
// @Failure 400 {object} types.APIBadRequest
// @Failure 401 {object} types.APIUnauthorized
// @Failure 403 {object} types.APIForbidden
// @Failure 500 {object} types.APIInternalServerError
// @Router /admin/common/notifications/poll/{limit} [get]
func (h *adminNotificationHandler) CommonNotificationPoll(ctx *gin.Context) { h.target(ctx) }

// CommonNotifications independently authorized entry preserves the existing personal notification handler.
// @Security ApiKey
// @Summary List the current administrator user's notifications
// @Description Requires administrator membership independently checked by the notification service; the original current-user scope and response are preserved.
// @Tags Admin Common
// @Produce json
// @Param page query int false "Page number" default(1)
// @Param page_size query int false "Page size" default(10)
// @Param notification_type query string false "Notification type"
// @Param unread_only query bool false "Only unread notifications" default(false)
// @Param title query string false "Notification title"
// @Success 200 {object} types.Response{data=types.NotificationsResp}
// @Failure 400 {object} types.APIBadRequest
// @Failure 401 {object} types.APIUnauthorized
// @Failure 403 {object} types.APIForbidden
// @Failure 500 {object} types.APIInternalServerError
// @Router /admin/common/notifications [get]
func (h *adminNotificationHandler) CommonNotifications(ctx *gin.Context) { h.target(ctx) }

// CommonReadNotifications independently authorized entry preserves the existing personal notification handler.
// @Security ApiKey
// @Summary Mark the current administrator user's notifications as read
// @Description Requires administrator membership independently checked by the notification service; the original current-user scope and response are preserved.
// @Tags Admin Common
// @Accept json
// @Produce json
// @Param body body types.BatchNotificationOperationReq true "Notifications to mark as read"
// @Success 200 {object} types.Response{data=nil}
// @Failure 400 {object} types.APIBadRequest
// @Failure 401 {object} types.APIUnauthorized
// @Failure 403 {object} types.APIForbidden
// @Failure 500 {object} types.APIInternalServerError
// @Router /admin/common/notifications/read [put]
func (h *adminNotificationHandler) CommonReadNotifications(ctx *gin.Context) { h.target(ctx) }

// CommonDeleteNotifications independently authorized entry preserves the existing personal notification handler.
// @Security ApiKey
// @Summary Delete the current administrator user's notifications
// @Description Requires administrator membership independently checked by the notification service; the original current-user scope and response are preserved.
// @Tags Admin Common
// @Accept json
// @Produce json
// @Param body body types.BatchNotificationOperationReq true "Notifications to delete"
// @Success 200 {object} types.Response{data=nil}
// @Failure 400 {object} types.APIBadRequest
// @Failure 401 {object} types.APIUnauthorized
// @Failure 403 {object} types.APIForbidden
// @Failure 500 {object} types.APIInternalServerError
// @Router /admin/common/notifications [delete]
func (h *adminNotificationHandler) CommonDeleteNotifications(ctx *gin.Context) { h.target(ctx) }
