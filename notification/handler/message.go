package handler

import (
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/notification/component"
)

// NotifierMessageHandler handles notifications related requests
type NotificationHandler struct {
	nmc component.NotificationComponent
	mac component.MailComponent
}

func NewNotificationHandler(conf *config.Config) (*NotificationHandler, error) {
	nmc, err := component.NewNotificationComponent(conf)
	if err != nil {
		slog.Error("Failed to create notification component", slog.Any("error", err))
		return nil, err
	}
	mac, err := component.NewMailComponent(conf)
	if err != nil {
		slog.Error("Failed to create mail component", slog.Any("error", err))
		return nil, err
	}

	return &NotificationHandler{
		nmc: nmc,
		mac: mac,
	}, nil
}

// _mock for test
func NewNotifierMessageHandlerWithMock(nmc component.NotificationComponent) *NotificationHandler {
	return &NotificationHandler{
		nmc: nmc,
	}
}

// GetUnreadCount Get the count of unread notifications
// @Summary Get unread notifications count
// @Description Retrieve the number of unread notifications
// @Tags Notifications
// @Tags         Access token
// @Param Authorization header string true "Authorization token (Bearer <token>)"
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
// @Summary List notifications
// @Description List all notifications with pagination and filtering options
// @Tags Notifications
// @Tags         Access token
// @Param page query int false "Page number" default(1)
// @Param page_size query int false "Number of items per page" default(10)
// @Param notification_type query string false "Type of notification"
// @Param unread_only query bool false "Only return unread notifications" default(false)
// @Param title query string false "Notification title"
// @Param Authorization header string true "Authorization token (Bearer <token>)"
// @Produce json
// @Success 200  {object}  types.Response{data=types.NotificationsResp} "OK"
// @Failure 400  {object}  types.APIBadRequest "Bad request"
// @Failure 401  {object}  types.APIUnauthorized "Unauthorized"
// @Failure 500  {object}  types.APIInternalServerError "Internal server error"
// @Router /notifications [get]
func (h *NotificationHandler) ListNotifications(c *gin.Context) {
	var req types.NotificationsRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		httpbase.BadRequest(c, err.Error())
		return
	}

	if req.Page <= 0 {
		httpbase.BadRequest(c, "page must be greater than 0")
		return
	}
	if req.PageSize <= 0 {
		httpbase.BadRequest(c, "page_size must be greater than 0")
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
// @Summary Mark notifications as read
// @Description Mark specified notifications or all notifications as read
// @Tags Notifications
// @Tags         Access token
// @Accept json
// @Param Authorization header string true "Authorization token (Bearer <token>)"
// @Param request body types.MarkNotificationsAsReadReq true "Mark as read request"
// @Produce json
// @Success 200  {object}  types.Response{data=nil} "OK"
// @Failure 400  {object}  types.APIBadRequest "Bad request"
// @Failure 401  {object}  types.APIUnauthorized "Unauthorized"
// @Failure 500  {object}  types.APIInternalServerError "Internal server error"
// @Router /notifications/read [put]
func (h *NotificationHandler) MarkAsRead(c *gin.Context) {
	var req types.MarkNotificationsAsReadReq
	if err := c.ShouldBindJSON(&req); err != nil {
		httpbase.BadRequest(c, err.Error())
		return
	}

	if !req.MarkAll && len(req.IDs) == 0 {
		httpbase.BadRequest(c, "Please provide message_ids or set mark_all to true")
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

// UpdateSubscription Update notification settings
// @Summary Update subscription settings
// @Description Update the user's notification settings
// @Tags Notifications
// @Tags         Access token
// @Accept json
// @Param Authorization header string true "Authorization token (Bearer <token>)"
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
		httpbase.BadRequest(c, err.Error())
		return
	}
	if req.IsEmailNotificationEnabled && req.EmailAddress == "" {
		httpbase.BadRequest(c, "email_address is required when is_email_notification_enabled is true")
		return
	}

	if req.IsSMSNotificationEnabled && req.PhoneNumber == "" {
		httpbase.BadRequest(c, "phone_number is required when is_sms_notification_enabled is true")
		return
	}

	location, err := time.LoadLocation(req.Timezone)
	if err != nil {
		httpbase.BadRequest(c, "timezone is invalid")
		return
	}

	startTime, err := time.ParseInLocation("15:04", req.DoNotDisturbStart, location)
	if err != nil {
		httpbase.BadRequest(c, "do_not_disturb_start is invalid, expected format HH:MM")
		return
	}
	req.DoNotDisturbStartTime = time.Date(2000, 1, 1, startTime.Hour(), startTime.Minute(), 0, 0, time.UTC)

	endTime, err := time.ParseInLocation("15:04", req.DoNotDisturbEnd, location)
	if err != nil {
		httpbase.BadRequest(c, "do_not_disturb_end is invalid, expected format HH:MM")
		return
	}
	req.DoNotDisturbEndTime = time.Date(2000, 1, 1, endTime.Hour(), endTime.Minute(), 0, 0, time.UTC)

	if len(req.SubNotificationType) > 0 {
		for _, t := range req.SubNotificationType {
			if !types.NotificationType(t).IsValid() {
				httpbase.BadRequest(c, "invalid notification_type")
				return
			}
		}
	}
	if req.MessageTTL < 0 {
		httpbase.BadRequest(c, "message_ttl must be greater 0")
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
// @Summary Poll new notifications
// @Description Poll new notifications with a specified limit
// @Tags Notifications
// @Tags         Access token
// @Produce json
// @Param Authorization header string true "Authorization token (Bearer <token>)"
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
		httpbase.BadRequest(c, "limit must be a positive integer")
		return
	}

	timezone := c.Query("timezone")
	if timezone == "" {
		httpbase.BadRequest(c, "timezone is required")
		return
	}
	location, err := time.LoadLocation(timezone)
	if err != nil {
		httpbase.BadRequest(c, "timezone is invalid")
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
// @Summary Get notification settings
// @Description get settings or default settings
// @Tags Notifications
// @Tags         Access token
// @Param Authorization header string true "Authorization token (Bearer <token>)"
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
		httpbase.BadRequest(c, "timezone is required")
		return
	}
	location, err := time.LoadLocation(timezone)
	if err != nil {
		httpbase.BadRequest(c, "timezone is invalid")
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
// @Summary Get all message types []string
// @Description Get all available message types
// @Tags Notifications
// @Tags         Access token
// @Produce json
// @Param Authorization header string true "Authorization token (Bearer <token>)"
// @Success 200  {object}  types.Response{data=[]string} "OK"
// @Failure 400  {object}  types.APIBadRequest "Bad request"
// @Failure 401  {object}  types.APIUnauthorized "Unauthorized"
// @Failure 500  {object}  types.APIInternalServerError "Internal server error"
// @Router /notifications/message-types [get]
func (h *NotificationHandler) GetAllMessageTypes(c *gin.Context) {
	httpbase.OK(c, types.NotificationTypeAll())
}

// CreateMessageTask send message to user
// @Summary Send a message to a user [only for admin]
// @Description Send a message to a user
// @Tags Notifications
// @Tags         Access token
// @Accept json
// @Param Authorization header string true "Authorization token (Bearer <token>)"
// @Param request body types.NotificationMessage true "Send message request"
// @Produce json
// @Success 200  {object}  types.Response{data=nil} "OK"
// @Failure 400  {object}  types.APIBadRequest "Bad request"
// @Failure 401  {object}  types.APIUnauthorized "Unauthorized"
// @Failure 500  {object}  types.APIInternalServerError "Internal server error"
// @Router /notifications/msg-task [put]
func (h *NotificationHandler) CreateMessageTask(c *gin.Context) {
	var req types.NotificationMessage
	if err := c.ShouldBindJSON(&req); err != nil {
		httpbase.BadRequest(c, err.Error())
		return
	}

	if !req.NotificationType.IsSystem() {
		httpbase.BadRequest(c, "notification_type must be system")
		return
	}

	req.MsgUUID = uuid.New().String()
	if req.CreateAt.IsZero() {
		req.CreateAt = time.Now()
	}

	if err := h.nmc.PublishNotificationMessage(c.Request.Context(), req); err != nil {
		slog.Error("Failed to publish notification message", slog.Any("request", req), slog.Any("error", err))
		httpbase.ServerError(c, err)
		return
	}

	httpbase.OK(c, nil)
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
		httpbase.BadRequest(c, err.Error())
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
