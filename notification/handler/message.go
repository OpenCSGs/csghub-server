package handler

import (
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/notification/component"
)

// NotifierMessageHandler handles notifications related requests
type NotificationHandler struct {
	nmc component.NotificationComponent
}

func NewNotificationHandler(conf *config.Config) (*NotificationHandler, error) {
	nmc, err := component.NewNotificationComponent(conf)
	if err != nil {
		slog.Error("Failed to create notification component", slog.Any("error", err))
		return nil, err
	}

	return &NotificationHandler{
		nmc: nmc,
	}, nil
}

// GetUnreadCount Get the count of unread notifications
// @Security ApiKey
// @Summary Get unread notifications count
// @Description Retrieve the number of unread notifications
// @Tags Notifications
// @Produce json
// @Success 200  {object}  types.Response{data=int} "OK"
// @Failure 400  {object}  types.APIBadRequest "Bad request"
// @Failure 401  {object}  types.APIUnauthorized "Unauthorized"
// @Failure 500  {object}  types.APIInternalServerError "Internal server error"
// @Router /notifications/count [get]
func (h *NotificationHandler) GetUnreadCount(c *gin.Context) {
	uuid := httpbase.GetCurrentUserUUID(c)
	if uuid == "" {
		httpbase.UnauthorizedError(c, fmt.Errorf("user_uuid is required"))
		return
	}
	unreadCount, err := h.nmc.GetUnreadCount(c.Request.Context(), uuid)
	if err != nil {
		slog.Error("Failed to get unread count", slog.Any("user_uuid", uuid), slog.Any("error", err))
		httpbase.ServerError(c, err)
		return
	}
	httpbase.OK(c, unreadCount)
}

// ListNotifications List all notifications
// @Security ApiKey
// @Summary List notifications
// @Description List all notifications with pagination and filtering options
// @Tags Notifications
// @Param page query int false "Page number" default(1)
// @Param page_size query int false "Number of items per page" default(10)
// @Param notification_type query string false "Type of notification"
// @Param unread_only query bool false "Only return unread notifications" default(false)
// @Param title query string false "Notification title"
// @Produce json
// @Success 200  {object}  types.Response{data=types.NotificationsResp} "OK"
// @Failure 400  {object}  types.APIBadRequest "Bad request"
// @Failure 401  {object}  types.APIUnauthorized "Unauthorized"
// @Failure 500  {object}  types.APIInternalServerError "Internal server error"
// @Router /notifications [get]
func (h *NotificationHandler) ListNotifications(c *gin.Context) {
	var req types.NotificationsRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequestWithExt(c, errorx.ReqBodyFormat(err, nil))
		return
	}

	if req.Page <= 0 {
		slog.Error("Bad request format, page must be greater than 0", slog.Any("request", req))
		httpbase.BadRequestWithExt(c, errorx.ReqBodyFormat(errors.New("page must be greater than 0"), nil))
		return
	}
	if req.PageSize <= 0 {
		slog.Error("Bad request format, page_size must be greater than 0", slog.Any("request", req))
		httpbase.BadRequestWithExt(c, errorx.ReqBodyFormat(errors.New("page_size must be greater than 0"), nil))
		return
	}

	uuid := httpbase.GetCurrentUserUUID(c)
	if uuid == "" {
		httpbase.UnauthorizedError(c, fmt.Errorf("user_uuid is required"))
		return
	}
	unread, err := h.nmc.GetUnreadCount(c.Request.Context(), uuid)
	if err != nil {
		slog.Error("Failed to get unread count", slog.Any("user_uuid", uuid), slog.Any("error", err))
		httpbase.ServerError(c, err)
		return
	}

	messages, total, err := h.nmc.ListNotifications(c.Request.Context(), uuid, req)
	if err != nil {
		slog.Error("Failed to list notifications", slog.Any("user_uuid", uuid), slog.Any("request", req), slog.Any("error", err))
		httpbase.ServerError(c, err)
		return
	}

	result := types.NotificationsResp{
		Messages:    messages,
		TotalCount:  int64(total),
		UnreadCount: unread,
	}

	httpbase.OK(c, result)
}

// MarkAsRead Mark notifications as read
// @Security ApiKey
// @Summary Mark notifications as read
// @Description Mark specified notifications or all notifications as read
// @Tags Notifications
// @Accept json
// @Param request body types.BatchNotificationOperationReq true "Mark as read request"
// @Produce json
// @Success 200  {object}  types.Response{data=nil} "OK"
// @Failure 400  {object}  types.APIBadRequest "Bad request"
// @Failure 401  {object}  types.APIUnauthorized "Unauthorized"
// @Failure 500  {object}  types.APIInternalServerError "Internal server error"
// @Router /notifications/read [put]
func (h *NotificationHandler) MarkAsRead(c *gin.Context) {
	var req types.BatchNotificationOperationReq
	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequestWithExt(c, errorx.ReqBodyFormat(err, nil))
		return
	}

	if !req.MarkAll && len(req.IDs) == 0 {
		slog.Error("Bad request format, ids or mark_all is required", slog.Any("request", req))
		ext := errorx.Ctx().Set("body", "ids or mark_all")
		httpbase.BadRequestWithExt(c, errorx.ReqBodyFormat(errors.New("ids or mark_all is required"), ext))
		return
	}

	err := h.nmc.MarkAsRead(c.Request.Context(), httpbase.GetCurrentUserUUID(c), req)
	if err != nil {
		slog.Error("Failed to mark as read", slog.Any("user_uuid", httpbase.GetCurrentUserUUID(c)), slog.Any("request", req), slog.Any("error", err))
		httpbase.ServerError(c, err)
		return
	}

	httpbase.OK(c, nil)
}

// MarkAsUnread Mark notifications as unread
// @Security ApiKey
// @Summary Mark notifications as unread
// @Description Mark specified notifications or all notifications as unread
// @Tags Notifications
// @Accept json
// @Param request body types.BatchNotificationOperationReq true "Mark as unread request"
// @Produce json
// @Success 200  {object}  types.Response{data=nil} "OK"
// @Failure 400  {object}  types.APIBadRequest "Bad request"
// @Failure 401  {object}  types.APIUnauthorized "Unauthorized"
// @Failure 500  {object}  types.APIInternalServerError "Internal server error"
// @Router /notifications/unread [put]
func (h *NotificationHandler) MarkAsUnread(c *gin.Context) {
	var req types.BatchNotificationOperationReq
	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequestWithExt(c, errorx.ReqBodyFormat(err, nil))
		return
	}

	if !req.MarkAll && len(req.IDs) == 0 {
		slog.Error("Bad request format, ids or mark_all is required", slog.Any("request", req))
		ext := errorx.Ctx().Set("body", "ids or mark_all")
		httpbase.BadRequestWithExt(c, errorx.ReqBodyFormat(errors.New("ids or mark_all is required"), ext))
		return
	}

	err := h.nmc.MarkAsUnread(c.Request.Context(), httpbase.GetCurrentUserUUID(c), req)
	if err != nil {
		slog.Error("Failed to mark as unread", slog.Any("user_uuid", httpbase.GetCurrentUserUUID(c)), slog.Any("request", req), slog.Any("error", err))
		httpbase.ServerError(c, err)
		return
	}

	httpbase.OK(c, nil)
}

// delete notification
// @Security ApiKey
// @Summary Delete notification
// @Description Delete a notification
// @Tags Notifications
// @Accept json
// @Param request body types.BatchNotificationOperationReq true "Delete notifications request"
// @Produce json
// @Success 200  {object}  types.Response{data=nil} "OK"
// @Failure 400  {object}  types.APIBadRequest "Bad request"
// @Failure 401  {object}  types.APIUnauthorized "Unauthorized"
// @Failure 500  {object}  types.APIInternalServerError "Internal server error"
// @Router /notifications [delete]
func (h *NotificationHandler) DeleteNotifications(c *gin.Context) {
	var req types.BatchNotificationOperationReq
	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequestWithExt(c, errorx.ReqBodyFormat(err, nil))
		return
	}

	if !req.MarkAll && len(req.IDs) == 0 {
		slog.Error("Bad request format, ids or mark_all is required", slog.Any("request", req))
		ext := errorx.Ctx().Set("body", "ids or mark_all")
		httpbase.BadRequestWithExt(c, errorx.ReqBodyFormat(errors.New("ids or mark_all is required"), ext))
		return
	}

	err := h.nmc.DeleteNotifications(c.Request.Context(), httpbase.GetCurrentUserUUID(c), req)
	if err != nil {
		slog.Error("Failed to delete notification", slog.Any("user_uuid", httpbase.GetCurrentUserUUID(c)), slog.Any("request", req), slog.Any("error", err))
		httpbase.ServerError(c, err)
		return
	}

	httpbase.OK(c, nil)
}

// UpdateSubscription Update notification settings
// @Security ApiKey
// @Summary Update subscription settings
// @Description Update the user's notification settings
// @Tags Notifications
// @Accept json
// @Param request body types.UpdateNotificationReq true "Update subscription request"
// @Produce json
// @Success 200  {object}  types.Response{data=nil} "OK"
// @Failure 400  {object}  types.APIBadRequest "Bad request"
// @Failure 401  {object}  types.APIUnauthorized "Unauthorized"
// @Failure 500  {object}  types.APIInternalServerError "Internal server error"
// @Router /notifications/setting [put]
func (h *NotificationHandler) UpdateNotificationSetting(c *gin.Context) {
	var req types.UpdateNotificationReq
	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequestWithExt(c, errorx.ReqBodyFormat(err, nil))
		return
	}
	if req.IsEmailNotificationEnabled && req.EmailAddress == "" {
		slog.Error("Bad request format, email_address is required when is_email_notification_enabled is true", slog.Any("request", req))
		httpbase.BadRequestWithExt(c, errorx.ReqBodyFormat(errors.New("email_address is required when is_email_notification_enabled is true"), nil))
		return
	}

	if req.IsSMSNotificationEnabled && req.PhoneNumber == "" {
		slog.Error("Bad request format, phone_number is required when is_sms_notification_enabled is true", slog.Any("request", req))
		httpbase.BadRequestWithExt(c, errorx.ReqBodyFormat(errors.New("phone_number is required when is_sms_notification_enabled is true"), nil))
		return
	}

	location, err := time.LoadLocation(req.Timezone)
	if err != nil {
		slog.Error("Bad request format, timezone is invalid", slog.Any("request", req))
		httpbase.BadRequestWithExt(c, errorx.ReqBodyFormat(errors.New("timezone is invalid"), nil))
		return
	}

	startTime, err := time.ParseInLocation("15:04", req.DoNotDisturbStart, location)
	if err != nil {
		slog.Error("Bad request format, do_not_disturb_start is invalid, expected format HH:MM", slog.Any("request", req))
		httpbase.BadRequestWithExt(c, errorx.ReqBodyFormat(errors.New("do_not_disturb_start is invalid, expected format HH:MM"), nil))
		return
	}
	req.DoNotDisturbStartTime = time.Date(2000, 1, 1, startTime.Hour(), startTime.Minute(), 0, 0, time.UTC)

	endTime, err := time.ParseInLocation("15:04", req.DoNotDisturbEnd, location)
	if err != nil {
		slog.Error("Bad request format, do_not_disturb_end is invalid, expected format HH:MM", slog.Any("request", req))
		httpbase.BadRequestWithExt(c, errorx.ReqBodyFormat(errors.New("do_not_disturb_end is invalid, expected format HH:MM"), nil))
		return
	}
	req.DoNotDisturbEndTime = time.Date(2000, 1, 1, endTime.Hour(), endTime.Minute(), 0, 0, time.UTC)

	if len(req.SubNotificationType) > 0 {
		for _, t := range req.SubNotificationType {
			if !types.NotificationType(t).IsValid() {
				slog.Error("Bad request format, invalid notification_type", slog.Any("request", req))
				httpbase.BadRequestWithExt(c, errorx.ReqBodyFormat(errors.New("invalid notification_type"), nil))
				return
			}
		}
	}
	if req.MessageTTL < 0 {
		slog.Error("Bad request format, message_ttl must be greater 0", slog.Any("request", req))
		httpbase.BadRequestWithExt(c, errorx.ReqBodyFormat(errors.New("message_ttl must be greater 0"), nil))
		return
	}

	if err := h.nmc.NotificationsSetting(c.Request.Context(), httpbase.GetCurrentUserUUID(c), req, location); err != nil {
		slog.Error("Failed to update notification setting", slog.Any("user_uuid", httpbase.GetCurrentUserUUID(c)), slog.Any("request", req), slog.Any("error", err))
		httpbase.ServerError(c, err)
		return
	}
	httpbase.OK(c, nil)
}

// PollNewNotifications godoc
// @Security ApiKey
// @Summary Poll new notifications
// @Description Poll new notifications with a specified limit
// @Tags Notifications
// @Produce json
// @Param limit path int true "The maximum number of new notifications to poll"
// @Param timezone query string false "Timezone" default(Asia/Shanghai)
// @Success 200  {object}  types.Response{data=types.NewNotifications} "OK"
// @Failure 400  {object}  types.APIBadRequest "Bad request"
// @Failure 401  {object}  types.APIUnauthorized "Unauthorized"
// @Failure 500  {object}  types.APIInternalServerError "Internal server error"
// @Router /notifications/poll/{limit} [get]
func (h *NotificationHandler) PollNewNotifications(c *gin.Context) {
	uuid := httpbase.GetCurrentUserUUID(c)
	if uuid == "" {
		httpbase.UnauthorizedError(c, fmt.Errorf("user_uuid is required"))
		return
	}

	limit, err := strconv.ParseInt(c.Param("limit"), 10, 64)
	if err != nil || limit <= 0 {
		slog.Error("Bad request format, limit must be a positive integer", slog.Any("limit", limit), slog.Any("error", err))
		httpbase.BadRequestWithExt(c, errorx.ReqBodyFormat(errors.New("limit must be a positive integer"), nil))
		return
	}

	timezone := c.Query("timezone")
	if timezone == "" {
		slog.Error("Bad request format, timezone is required", slog.Any("timezone", timezone))
		httpbase.BadRequestWithExt(c, errorx.ReqBodyFormat(errors.New("timezone is required"), nil))
		return
	}
	location, err := time.LoadLocation(timezone)
	if err != nil {
		slog.Error("Bad request format, timezone is invalid", slog.Any("timezone", timezone), slog.Any("error", err))
		httpbase.BadRequestWithExt(c, errorx.ReqBodyFormat(errors.New("timezone is invalid"), nil))
		return
	}

	notifications, err := h.nmc.PollNewNotifications(c.Request.Context(), uuid, int(limit), location)
	if err != nil {
		slog.Error("Failed to poll new notifications", slog.Any("user_uuid", uuid), slog.Any("limit", limit), slog.Any("error", err))
		httpbase.ServerError(c, err)
		return
	}
	httpbase.OK(c, notifications)
}

// GetNotificationSetting user
// @Security ApiKey
// @Summary Get notification settings
// @Description get settings or default settings
// @Tags Notifications
// @Param timezone query string false "Timezone" default(Asia/Shanghai)
// @Produce json
// @Success 200  {object}  types.Response{data=types.NotificationSetting} "OK"
// @Failure 400  {object}  types.APIBadRequest "Bad request"
// @Failure 401  {object}  types.APIUnauthorized "Unauthorized"
// @Failure 500  {object}  types.APIInternalServerError "Internal server error"
// @Router /notifications/setting [get]
func (h *NotificationHandler) GetNotificationSetting(c *gin.Context) {
	uuid := httpbase.GetCurrentUserUUID(c)
	if uuid == "" {
		httpbase.UnauthorizedError(c, fmt.Errorf("user_uuid is required"))
		return
	}

	timezone := c.Query("timezone")
	if timezone == "" {
		slog.Error("Bad request format, timezone is required", slog.Any("timezone", timezone))
		httpbase.BadRequestWithExt(c, errorx.ReqBodyFormat(errors.New("timezone is required"), nil))
		return
	}
	location, err := time.LoadLocation(timezone)
	if err != nil {
		slog.Error("Bad request format, timezone is invalid", slog.Any("timezone", timezone), slog.Any("error", err))
		httpbase.BadRequestWithExt(c, errorx.ReqBodyFormat(errors.New("timezone is invalid"), nil))
		return
	}

	notificationSetting, err := h.nmc.GetNotificationSetting(c.Request.Context(), uuid, location)
	if err != nil {
		slog.Error("Failed to get notification setting", slog.Any("user_uuid", uuid), slog.Any("error", err))
		httpbase.ServerError(c, err)
		return
	}

	httpbase.OK(c, notificationSetting)
}

// GetAllMessageTypes get all message types
// @Security ApiKey
// @Summary Get all message types []string
// @Description Get all available message types
// @Tags Notifications
// @Produce json
// @Success 200  {object}  types.Response{data=[]string} "OK"
// @Failure 400  {object}  types.APIBadRequest "Bad request"
// @Failure 401  {object}  types.APIUnauthorized "Unauthorized"
// @Failure 500  {object}  types.APIInternalServerError "Internal server error"
// @Router /notifications/message-types [get]
func (h *NotificationHandler) GetAllMessageTypes(c *gin.Context) {
	httpbase.OK(c, types.NotificationTypeAll())
}

// SendMessage send a message
// @Security ApiKey
// @Summary Send a message
// @Description Send a message
// @Tags Notifications
// @Accept json
// @Param request body types.MessageRequest true "post a message request"
// @Produce json
// @Success 200  {object}  types.Response{data=nil} "OK"
// @Failure 400  {object}  types.APIBadRequest "Bad request"
// @Failure 401  {object}  types.APIUnauthorized "Unauthorized"
// @Failure 500  {object}  types.APIInternalServerError "Internal server error"
// @Router /notifications [post]
func (h *NotificationHandler) SendMessage(c *gin.Context) {
	var req types.MessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequestWithExt(c, errorx.ReqBodyFormat(err, nil))
		return
	}

	scenarioMessage := types.ScenarioMessage{
		Scenario:   req.Scenario,
		Parameters: req.Parameters,
		Priority:   req.Priority,
	}

	if err := h.nmc.PublishMessage(c.Request.Context(), scenarioMessage); err != nil {
		slog.Error("Failed to send message", slog.Any("request", req), slog.Any("error", err))
		httpbase.ServerError(c, err)
		return
	}

	httpbase.OK(c, nil)
}
