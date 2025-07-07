package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/notification/handler"

	mc "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/notification/component"
)

// TestGetUnreadCount tests the GetUnreadCount handler function.
func TestGetUnreadCount(t *testing.T) {
	gin.SetMode(gin.TestMode)
	nmc := mc.NewMockNotificationComponent(t)
	nmc.EXPECT().GetUnreadCount(context.Background(), "test").Return(10, nil)
	handler := handler.NewNotifierMessageHandlerWithMock(nmc)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/notifications/unread", nil)
	ctx, _ := gin.CreateTestContext(w)
	ctx.Request = req
	httpbase.SetCurrentUserUUID(ctx, "test")
	handler.GetUnreadCount(ctx)
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, but got %d, msg %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Errorf("Expected JSON response, but got %s", w.Body.String())
	}
	if resp["data"].(float64) != 10 {
		t.Errorf("Expected unread count 10, but got %v", resp["data"])
	}
}

func TestListNotifications(t *testing.T) {
	gin.SetMode(gin.TestMode)
	nmc := mc.NewMockNotificationComponent(t)
	req := types.NotificationsRequest{
		Page:     1,
		PageSize: 10,
	}
	nmc.EXPECT().ListNotifications(context.Background(), "test", req).Return([]types.Notifications{}, 0, nil)
	nmc.EXPECT().GetUnreadCount(context.Background(), "test").Return(0, nil)
	handler := handler.NewNotifierMessageHandlerWithMock(nmc)
	w := httptest.NewRecorder()
	reqObj, _ := http.NewRequest("GET", fmt.Sprintf("/notifications?page=%d&page_size=%d", req.Page, req.PageSize), nil)
	ctx, _ := gin.CreateTestContext(w)
	ctx.Request = reqObj
	httpbase.SetCurrentUserUUID(ctx, "test")
	handler.ListNotifications(ctx)
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, but got %d, msg %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Errorf("Expected JSON response, but got %s", w.Body.String())
	}
}

func TestMarkAsRead(t *testing.T) {
	gin.SetMode(gin.TestMode)
	nmc := mc.NewMockNotificationComponent(t)
	req := types.MarkNotificationsAsReadReq{
		MarkAll: true,
	}
	nmc.EXPECT().MarkAsRead(context.Background(), "test", req).Return(nil)
	handler := handler.NewNotifierMessageHandlerWithMock(nmc)
	w := httptest.NewRecorder()
	body, _ := json.Marshal(req)
	reqObj, _ := http.NewRequest("PUT", "/notifications/count", bytes.NewBuffer(body))
	reqObj.Header.Set("Content-Type", "application/json")
	ctx, _ := gin.CreateTestContext(w)
	ctx.Request = reqObj
	httpbase.SetCurrentUserUUID(ctx, "test")
	handler.MarkAsRead(ctx)
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, but got %d, msg %s", w.Code, w.Body.String())
	}
}

func TestUpdateNotificationSetting(t *testing.T) {
	gin.SetMode(gin.TestMode)
	nmc := mc.NewMockNotificationComponent(t)
	location, _ := time.LoadLocation("Asia/Shanghai")
	req := types.UpdateNotificationReq{
		IsEmailNotificationEnabled: true,
		EmailAddress:               "test@example.com",
		IsDoNotDisturbEnabled:      true,
		DoNotDisturbStart:          "09:00",
		DoNotDisturbEnd:            "17:00",
		SubNotificationType:        []string{"system"},
		MessageTTL:                 100,
		IsSMSNotificationEnabled:   true,
		PhoneNumber:                "1234567890",
		Timezone:                   "Asia/Shanghai",
	}

	// The handler will modify the request by setting DoNotDisturbStartTime and DoNotDisturbEndTime
	// So we need to expect a request with those fields set
	expectedReq := req
	startTime, _ := time.ParseInLocation("15:04", req.DoNotDisturbStart, location)
	expectedReq.DoNotDisturbStartTime = time.Date(2000, 1, 1, startTime.Hour(), startTime.Minute(), 0, 0, time.UTC)
	endTime, _ := time.ParseInLocation("15:04", req.DoNotDisturbEnd, location)
	expectedReq.DoNotDisturbEndTime = time.Date(2000, 1, 1, endTime.Hour(), endTime.Minute(), 0, 0, time.UTC)

	nmc.EXPECT().NotificationsSetting(context.Background(), "test", expectedReq, location).Return(nil)
	handler := handler.NewNotifierMessageHandlerWithMock(nmc)
	w := httptest.NewRecorder()
	body, _ := json.Marshal(req)
	reqObj, _ := http.NewRequest("PUT", "/notifications/setting", bytes.NewBuffer(body))
	reqObj.Header.Set("Content-Type", "application/json")
	ctx, _ := gin.CreateTestContext(w)
	ctx.Request = reqObj
	httpbase.SetCurrentUserUUID(ctx, "test")
	handler.UpdateNotificationSetting(ctx)
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, but got %d, msg %s", w.Code, w.Body.String())
	}
}

func TestPollNewNotifications(t *testing.T) {
	gin.SetMode(gin.TestMode)
	nmc := mc.NewMockNotificationComponent(t)
	limit := 5
	t.Logf("Testing with limit: %d", limit)
	location, _ := time.LoadLocation("Asia/Shanghai")
	nmc.EXPECT().PollNewNotifications(context.Background(), "test", limit, location).Return(&types.NewNotifications{}, nil)
	handler := handler.NewNotifierMessageHandlerWithMock(nmc)

	router := gin.Default()
	router.Use(func(ctx *gin.Context) {
		httpbase.SetCurrentUserUUID(ctx, "test")
		ctx.Next()
	})
	router.GET("/notifications/poll/:limit", handler.PollNewNotifications)

	w := httptest.NewRecorder()
	reqObj, _ := http.NewRequest("GET", fmt.Sprintf("/notifications/poll/%d?timezone=Asia/Shanghai", limit), nil)
	ctx, _ := gin.CreateTestContext(w)
	ctx.Request = reqObj

	router.HandleContext(ctx)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, but got %d, msg %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Errorf("Expected JSON response, but got %s", w.Body.String())
	}
}

func TestGetNotificationSetting(t *testing.T) {
	gin.SetMode(gin.TestMode)
	nmc := mc.NewMockNotificationComponent(t)
	location, _ := time.LoadLocation("Asia/Shanghai")
	nmc.EXPECT().GetNotificationSetting(context.Background(), "test", location).Return(&types.NotificationSetting{}, nil)
	handler := handler.NewNotifierMessageHandlerWithMock(nmc)
	w := httptest.NewRecorder()
	reqObj, _ := http.NewRequest("GET", "/notifications/setting?timezone=Asia/Shanghai", nil)
	ctx, _ := gin.CreateTestContext(w)
	ctx.Request = reqObj
	httpbase.SetCurrentUserUUID(ctx, "test")
	handler.GetNotificationSetting(ctx)
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, but got %d, msg %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Errorf("Expected JSON response, but got %s", w.Body.String())
	}
}

func TestGetAllMessageTypes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := handler.NewNotifierMessageHandlerWithMock(mc.NewMockNotificationComponent(t))
	w := httptest.NewRecorder()
	reqObj, _ := http.NewRequest("GET", "/notifications/message-types", nil)
	ctx, _ := gin.CreateTestContext(w)
	ctx.Request = reqObj
	httpbase.SetCurrentUserUUID(ctx, "test")
	handler.GetAllMessageTypes(ctx)
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, but got %d, msg %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Errorf("Expected JSON response, but got %s", w.Body.String())
	}
}

func TestSendMessage_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockNMC := mc.NewMockNotificationComponent(t)
	handler := handler.NewNotifierMessageHandlerWithMock(mockNMC)

	reqBody := types.MessageRequest{
		Scenario:   "test_scenario",
		Parameters: "{\"status\": \"success\"}",
		Priority:   types.MessagePriorityHigh,
	}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("POST", "/notifications/message", bytes.NewBuffer(body))
	c.Request.Header.Set("Content-Type", "application/json")

	mockNMC.EXPECT().PublishMessage(c.Request.Context(), types.ScenarioMessage{
		Scenario:   "test_scenario",
		Parameters: "{\"status\": \"success\"}",
		Priority:   types.MessagePriorityHigh,
	}).Return(nil)

	handler.SendMessage(c)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, but got %d, msg %s", w.Code, w.Body.String())
	}
	mockNMC.AssertExpectations(t)
}
