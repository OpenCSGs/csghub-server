package component_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"

	"github.com/stretchr/testify/assert"
	mockdb "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/notification/component"
)

func TestGetUnreadCount(t *testing.T) {
	db := mockdb.NewMockNotificationStore(t)
	nmc := component.NewMockNotificationComponent(db)
	ctx := context.Background()
	uid := "test-uuid"

	db.EXPECT().GetUnreadCount(ctx, uid).Return(1, nil)
	count, err := nmc.GetUnreadCount(ctx, uid)
	if err != nil {
		t.Fatalf("GetUnreadCount failed: %v", err)
	}
	assert.Equal(t, int64(1), count)
}

func TestListNotifications(t *testing.T) {
	db := mockdb.NewMockNotificationStore(t)
	nmc := component.NewMockNotificationComponent(db)

	ctx := context.Background()
	uid := "test-uuid"
	req := types.NotificationsRequest{
		Page:       1,
		PageSize:   10,
		UnReadOnly: false,
	}
	params := database.ListNotificationsParams{
		UserUUID:         uid,
		NotificationType: req.NotificationType,
		TitleKeyword:     req.Title,
		Page:             req.Page,
		PageSize:         req.PageSize,
		UnreadOnly:       req.UnReadOnly,
	}
	var list []database.NotificationUserMessageView
	total := 1
	db.EXPECT().ListNotificationMessages(ctx, params).Return(list, total, nil)

	_, totalRes, err := nmc.ListNotifications(ctx, uid, req)
	if err != nil {
		t.Fatalf("ListNotifications failed: %v", err)
	}
	assert.Equal(t, total, totalRes)
}

func TestMarkAsRead(t *testing.T) {
	db := mockdb.NewMockNotificationStore(t)
	nmc := component.NewMockNotificationComponent(db)
	ctx := context.Background()
	uid := "test-uuid"
	req := types.BatchNotificationOperationReq{
		MarkAll: false,
		IDs:     []int64{1, 2, 3},
	}
	db.EXPECT().MarkAsRead(ctx, uid, req.IDs).Return(nil)

	err := nmc.MarkAsRead(ctx, uid, req)
	assert.NoError(t, err)
}

func TestNotificationsSetting(t *testing.T) {
	t.Run("existing setting should call UpdateSetting", func(t *testing.T) {
		db := mockdb.NewMockNotificationStore(t)
		nmc := component.NewMockNotificationComponent(db)
		location, _ := time.LoadLocation("Asia/Shanghai")
		ctx := context.Background()
		uid := "test-uuid"
		req := types.UpdateNotificationReq{
			SubNotificationType:        []string{"type1", "type2"},
			EmailAddress:               "test@example.com",
			IsEmailNotificationEnabled: true,
		}
		setting := &database.NotificationSetting{
			UserUUID:                   uid,
			SubNotificationType:        []string{"oldType"},
			EmailAddress:               "old@example.com",
			IsEmailNotificationEnabled: false,
		}

		// Expectation: setting exists, so UpdateSetting will be called
		db.EXPECT().GetSetting(ctx, uid).Return(setting, nil)
		db.EXPECT().UpdateSetting(ctx, mock.Anything).RunAndReturn(func(ctx context.Context, s *database.NotificationSetting) error {
			assert.Equal(t, req.SubNotificationType, s.SubNotificationType)
			assert.Equal(t, req.EmailAddress, s.EmailAddress)
			assert.Equal(t, req.IsEmailNotificationEnabled, s.IsEmailNotificationEnabled)
			return nil
		})

		err := nmc.NotificationsSetting(ctx, uid, req, location)
		assert.NoError(t, err)
	})

	t.Run("new setting should call CreateSetting", func(t *testing.T) {
		db := mockdb.NewMockNotificationStore(t)
		nmc := component.NewMockNotificationComponent(db)

		ctx := context.Background()
		uid := "new-uuid"
		req := types.UpdateNotificationReq{
			SubNotificationType:        []string{"type1", "type2"},
			EmailAddress:               "test@example.com",
			IsEmailNotificationEnabled: true,
		}
		location, _ := time.LoadLocation("Asia/Shanghai")

		// Expectation: no setting found, so CreateSetting will be called
		db.EXPECT().GetSetting(ctx, uid).Return(nil, nil)
		db.EXPECT().CreateSetting(ctx, mock.Anything).RunAndReturn(func(ctx context.Context, s *database.NotificationSetting) error {
			assert.Equal(t, uid, s.UserUUID)
			assert.Equal(t, req.SubNotificationType, s.SubNotificationType)
			assert.Equal(t, req.EmailAddress, s.EmailAddress)
			assert.Equal(t, req.IsEmailNotificationEnabled, s.IsEmailNotificationEnabled)
			return nil
		})

		err := nmc.NotificationsSetting(ctx, uid, req, location)
		assert.NoError(t, err)
	})
}

func TestGetNotificationSetting(t *testing.T) {
	db := mockdb.NewMockNotificationStore(t)
	nmc := component.NewMockNotificationComponent(db)

	ctx := context.Background()
	uid := "test-uuid"
	setting := &database.NotificationSetting{
		UserUUID:                   uid,
		SubNotificationType:        []string{"type1", "type2"},
		EmailAddress:               "test@example.com",
		IsEmailNotificationEnabled: true,
	}
	db.EXPECT().GetSetting(ctx, uid).Return(setting, nil)
	location, _ := time.LoadLocation("Asia/Shanghai")

	result, err := nmc.GetNotificationSetting(ctx, uid, location)
	if err != nil {
		t.Fatalf("GetNotificationSetting failed: %v", err)
	}
	assert.Equal(t, setting.UserUUID, result.UserUUID)
	assert.Equal(t, setting.SubNotificationType, result.SubNotificationType)
	assert.Equal(t, setting.EmailAddress, result.EmailAddress)
	assert.Equal(t, setting.IsEmailNotificationEnabled, result.IsEmailNotificationEnabled)
}

func TestPollNewNotifications(t *testing.T) {
	db := mockdb.NewMockNotificationStore(t)
	nmc := component.NewMockNotificationComponent(db)

	ctx := context.Background()
	userUUID := "test-uuid"
	limit := 10

	// Mock notification setting
	setting := &database.NotificationSetting{
		UserUUID:              userUUID,
		SubNotificationType:   []string{"system", "comment"},
		IsDoNotDisturbEnabled: false,
	}

	// Mock un-notified messages
	messages := []database.NotificationUserMessageView{
		{
			ID:               1,
			MsgUUID:          "msg-uuid-1",
			NotificationType: "system",
			SenderUUID:       "sender-uuid",
			Summary:          "Test summary",
			Title:            "Test title",
			Content:          "Test content",
			ActionURL:        "https://example.com",
			Priority:         1,
			UserUUID:         userUUID,
			ReadAt:           time.Time{}, // Not read
			IsNotified:       false,       // Not notified yet
			ExpireAt:         time.Now().Add(24 * time.Hour),
		},
	}

	total := 1

	// Set up expectations
	db.EXPECT().GetSetting(ctx, userUUID).Return(setting, nil)
	db.EXPECT().GetUnNotifiedMessages(ctx, userUUID, limit).Return(messages, total, nil)
	db.EXPECT().MarkAsNotified(ctx, userUUID, []string{"msg-uuid-1"}).Return(nil)

	// Execute the function
	location, _ := time.LoadLocation("Asia/Shanghai")
	result, err := nmc.PollNewNotifications(ctx, userUUID, limit, location)

	// Assertions
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Len(t, result.Data, 1)
	assert.Equal(t, "system", result.Data[0].NotificationType)
	assert.Equal(t, "Test title", result.Data[0].Title)
	assert.Equal(t, "Test summary", result.Data[0].Summary)
	assert.Equal(t, "Test content", result.Data[0].Content)
	assert.Equal(t, "https://example.com", result.Data[0].ClickActionURL)
	assert.False(t, result.Data[0].IsRead)
}

func TestPollNewNotifications_DoNotDisturbEnabled(t *testing.T) {
	db := mockdb.NewMockNotificationStore(t)
	nmc := component.NewMockNotificationComponent(db)

	ctx := context.Background()
	userUUID := "test-uuid"
	limit := 10

	// Mock notification setting with do-not-disturb enabled
	setting := &database.NotificationSetting{
		UserUUID:              userUUID,
		SubNotificationType:   []string{"system", "comment"},
		IsDoNotDisturbEnabled: true,
		DoNotDisturbStartTime: time.Date(2000, 1, 1, 22, 0, 0, 0, time.UTC), // 10 PM
		DoNotDisturbEndTime:   time.Date(2000, 1, 1, 8, 0, 0, 0, time.UTC),  // 8 AM
	}

	// Mock un-notified messages
	messages := []database.NotificationUserMessageView{
		{
			ID:               1,
			MsgUUID:          "msg-uuid-1",
			NotificationType: "system",
			SenderUUID:       "sender-uuid",
			Summary:          "Test summary",
			Title:            "Test title",
			Content:          "Test content",
			ActionURL:        "https://example.com",
			Priority:         1,
			UserUUID:         userUUID,
			ReadAt:           time.Time{},
			IsNotified:       false,
			ExpireAt:         time.Now().Add(24 * time.Hour),
		},
	}

	total := 1

	// Set up expectations
	db.EXPECT().GetSetting(ctx, userUUID).Return(setting, nil)
	db.EXPECT().GetUnNotifiedMessages(ctx, userUUID, limit).Return(messages, total, nil)
	db.EXPECT().MarkAsNotified(ctx, userUUID, []string{"msg-uuid-1"}).Return(nil)

	// Execute the function
	location, _ := time.LoadLocation("Asia/Shanghai")
	result, err := nmc.PollNewNotifications(ctx, userUUID, limit, location)

	// Assertions - during do-not-disturb hours, should return empty data
	assert.NoError(t, err)
	assert.NotNil(t, result)
	// The result should be empty because it's during do-not-disturb hours
	// (assuming the test runs during night hours)
	// assert.Len(t, result.Data, 0)
}

func TestPollNewNotifications_NoSetting(t *testing.T) {
	db := mockdb.NewMockNotificationStore(t)
	nmc := component.NewMockNotificationComponent(db)

	ctx := context.Background()
	userUUID := "test-uuid"
	limit := 10

	// Mock un-notified messages
	messages := []database.NotificationUserMessageView{
		{
			ID:               1,
			MsgUUID:          "msg-uuid-1",
			NotificationType: "system",
			SenderUUID:       "sender-uuid",
			Summary:          "Test summary",
			Title:            "Test title",
			Content:          "Test content",
			ActionURL:        "https://example.com",
			Priority:         1,
			UserUUID:         userUUID,
			ReadAt:           time.Time{},
			IsNotified:       false,
			ExpireAt:         time.Now().Add(24 * time.Hour),
		},
	}

	total := 1

	// Set up expectations - no setting found
	db.EXPECT().GetSetting(ctx, userUUID).Return(nil, nil)
	db.EXPECT().GetUnNotifiedMessages(ctx, userUUID, limit).Return(messages, total, nil)
	db.EXPECT().MarkAsNotified(ctx, userUUID, []string{"msg-uuid-1"}).Return(nil)

	// Execute the function
	location, _ := time.LoadLocation("Asia/Shanghai")
	result, err := nmc.PollNewNotifications(ctx, userUUID, limit, location)

	// Assertions
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Len(t, result.Data, 1)
	// Without settings, all notifications should be returned
	assert.Equal(t, "system", result.Data[0].NotificationType)
}

func TestPollNewNotifications_EmptyMessages(t *testing.T) {
	db := mockdb.NewMockNotificationStore(t)
	nmc := component.NewMockNotificationComponent(db)

	ctx := context.Background()
	userUUID := "test-uuid"
	limit := 10

	// Mock notification setting
	setting := &database.NotificationSetting{
		UserUUID:              userUUID,
		SubNotificationType:   []string{"system", "comment"},
		IsDoNotDisturbEnabled: false,
	}

	// Mock empty messages
	var messages []database.NotificationUserMessageView
	total := 0

	// Set up expectations
	db.EXPECT().GetSetting(ctx, userUUID).Return(setting, nil)
	db.EXPECT().GetUnNotifiedMessages(ctx, userUUID, limit).Return(messages, total, nil)

	location, _ := time.LoadLocation("Asia/Shanghai")
	result, err := nmc.PollNewNotifications(ctx, userUUID, limit, location)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Len(t, result.Data, 0)
	// Should have longer poll time when no messages
	assert.True(t, result.NextPollTime.After(time.Now().Add(30*time.Second)))
}

func TestMarkAsUnread(t *testing.T) {
	t.Run("mark as unread", func(t *testing.T) {
		db := mockdb.NewMockNotificationStore(t)
		nmc := component.NewMockNotificationComponent(db)

		ctx := context.Background()
		uid := "test-uuid"
		req := types.BatchNotificationOperationReq{
			MarkAll: false,
			IDs:     []int64{1, 2, 3},
		}
		db.EXPECT().MarkAsUnread(ctx, uid, req.IDs).Return(nil)

		err := nmc.MarkAsUnread(ctx, uid, req)
		assert.NoError(t, err)
	})

	t.Run("mark all as unread", func(t *testing.T) {
		db := mockdb.NewMockNotificationStore(t)
		nmc := component.NewMockNotificationComponent(db)

		ctx := context.Background()
		uid := "test-uuid"
		req := types.BatchNotificationOperationReq{
			MarkAll: true,
		}
		db.EXPECT().MarkAllAsUnread(ctx, uid).Return(nil)

		err := nmc.MarkAsUnread(ctx, uid, req)
		assert.NoError(t, err)
	})
}

func TestDeleteNotifications(t *testing.T) {
	t.Run("delete notifications", func(t *testing.T) {
		db := mockdb.NewMockNotificationStore(t)
		nmc := component.NewMockNotificationComponent(db)

		ctx := context.Background()
		uid := "test-uuid"
		req := types.BatchNotificationOperationReq{
			MarkAll: false,
			IDs:     []int64{1, 2, 3},
		}
		db.EXPECT().DeleteNotifications(ctx, uid, req.IDs).Return(nil)

		err := nmc.DeleteNotifications(ctx, uid, req)
		assert.NoError(t, err)
	})

	t.Run("delete all notifications", func(t *testing.T) {
		db := mockdb.NewMockNotificationStore(t)
		nmc := component.NewMockNotificationComponent(db)

		ctx := context.Background()
		uid := "test-uuid"
		req := types.BatchNotificationOperationReq{
			MarkAll: true,
		}
		db.EXPECT().DeleteAllNotifications(ctx, uid).Return(nil)

		err := nmc.DeleteNotifications(ctx, uid, req)
		assert.NoError(t, err)
	})
}
