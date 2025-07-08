package database_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/tests"
)

func TestListNotifications(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ns := database.NewNotificationStoreWithDB(db)

	notification := &database.NotificationMessage{
		MsgUUID:          "test_uuid",
		SenderUUID:       "test_sender",
		NotificationType: "test_type",
		Title:            "test_title",
		Summary:          "test_summary",
		Content:          "test_content",
		ActionURL:        "test_url",
		Priority:         1,
	}
	err := ns.CreateNotificationMessage(context.TODO(), notification)
	require.Nil(t, err)
	_, err = ns.CreateUserMessages(context.TODO(), notification.MsgUUID, []string{"test_user"})
	require.Nil(t, err)

	params := database.ListNotificationsParams{
		UserUUID:         "test_user",
		NotificationType: "test_type",
		TitleKeyword:     "test_title",
		Page:             1,
		PageSize:         10,
		UnreadOnly:       false,
	}

	notifications, total, err := ns.ListNotificationMessages(context.TODO(), params)
	require.Nil(t, err)
	require.Equal(t, 1, total)
	require.Equal(t, 1, len(notifications))
}

func TestCreateNotificationMessageForUsers(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ns := database.NewNotificationStoreWithDB(db)

	notification := &database.NotificationMessage{
		MsgUUID:          "test_uuid_1",
		SenderUUID:       "test_sender",
		NotificationType: "test_type",
		Title:            "test_title",
		Summary:          "test_summary",
		Content:          "test_content",
		ActionURL:        "test_url",
		Priority:         1,
	}

	err := ns.CreateNotificationMessageForUsers(context.TODO(), notification, []string{"test_user_1", "test_user_2"})
	require.Nil(t, err)

	notificationMessages, total, err := ns.ListNotificationMessages(context.TODO(), database.ListNotificationsParams{
		UserUUID:         "test_user_1",
		NotificationType: "test_type",
		TitleKeyword:     "test_title",
		Page:             1,
		PageSize:         10,
		UnreadOnly:       false,
	})
	require.Nil(t, err)
	require.Equal(t, 1, total)
	require.Equal(t, 1, len(notificationMessages))

	notificationMessages, total, err = ns.ListNotificationMessages(context.TODO(), database.ListNotificationsParams{
		UserUUID:         "test_user_2",
		NotificationType: "test_type",
		TitleKeyword:     "test_title",
		Page:             1,
		PageSize:         10,
		UnreadOnly:       false,
	})
	require.Nil(t, err)
	require.Equal(t, 1, total)
	require.Equal(t, 1, len(notificationMessages))
}

func TestGetSetting(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ns := database.NewNotificationStoreWithDB(db)

	setting := &database.NotificationSetting{
		UserUUID:                   "test_user",
		SubNotificationType:        []string{"test_type"},
		IsEmailNotificationEnabled: true,
		EmailAddress:               "test@example.com",
		MessageTTL:                 24 * time.Hour,
		IsSMSNotificationEnabled:   false,
		PhoneNumber:                "",
		IsDoNotDisturbEnabled:      false,
	}

	err := ns.CreateSetting(context.TODO(), setting)
	require.Nil(t, err)

	result, err := ns.GetSetting(context.TODO(), "test_user")
	require.Nil(t, err)
	require.Equal(t, setting.UserUUID, result.UserUUID)
}

func TestUpdateSetting(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ns := database.NewNotificationStoreWithDB(db)

	setting := &database.NotificationSetting{
		UserUUID:                   "test_user",
		SubNotificationType:        []string{"test_type"},
		IsEmailNotificationEnabled: true,
		EmailAddress:               "test@example.com",
		MessageTTL:                 24 * time.Hour,
		IsSMSNotificationEnabled:   false,
		PhoneNumber:                "",
		IsDoNotDisturbEnabled:      false,
	}

	err := ns.CreateSetting(context.TODO(), setting)
	require.Nil(t, err)

	setting.IsEmailNotificationEnabled = false
	err = ns.UpdateSetting(context.TODO(), setting)
	require.Nil(t, err)

	result, err := ns.GetSetting(context.TODO(), "test_user")
	require.Nil(t, err)
	require.Equal(t, false, result.IsEmailNotificationEnabled)
}

func TestCreateSetting(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ns := database.NewNotificationStoreWithDB(db)

	setting := &database.NotificationSetting{
		UserUUID:                   "test_user",
		SubNotificationType:        []string{"test_type"},
		IsEmailNotificationEnabled: true,
		EmailAddress:               "test@example.com",
		MessageTTL:                 24 * time.Hour,
		IsSMSNotificationEnabled:   false,
		PhoneNumber:                "",
		IsDoNotDisturbEnabled:      false,
	}

	err := ns.CreateSetting(context.TODO(), setting)
	require.Nil(t, err)
}

func TestGetUnreadCount(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ns := database.NewNotificationStoreWithDB(db)

	notification := &database.NotificationMessage{
		MsgUUID:          "test_uuid",
		SenderUUID:       "test_sender",
		NotificationType: "test_type",
		Title:            "test_title",
		Summary:          "test_summary",
		Content:          "test_content",
		ActionURL:        "test_url",
		Priority:         1,
	}

	err := ns.CreateNotificationMessageForUsers(context.TODO(), notification, []string{"test_user_1"})
	require.Nil(t, err)

	count, err := ns.GetUnreadCount(context.TODO(), "test_user_1")
	require.Nil(t, err)
	require.Equal(t, int64(1), count)
}

func TestMarkAsRead(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ns := database.NewNotificationStoreWithDB(db)

	notification := &database.NotificationMessage{
		MsgUUID:          "test_uuid",
		SenderUUID:       "test_sender",
		NotificationType: "test_type",
		Title:            "test_title",
		Summary:          "test_summary",
		Content:          "test_content",
		ActionURL:        "test_url",
		Priority:         1,
	}

	err := ns.CreateNotificationMessageForUsers(context.TODO(), notification, []string{"test_user_3"})
	require.Nil(t, err)

	notificationMessages, total, err := ns.ListNotificationMessages(context.TODO(), database.ListNotificationsParams{
		UserUUID:         "test_user_3",
		NotificationType: "test_type",
		TitleKeyword:     "test_title",
		Page:             1,
		PageSize:         10,
		UnreadOnly:       true,
	})
	require.Nil(t, err)
	require.Equal(t, 1, total)
	require.Equal(t, 1, len(notificationMessages))

	err = ns.MarkAsRead(context.TODO(), "test_user_3", []int64{notificationMessages[0].ID})
	require.Nil(t, err)

	count, err := ns.GetUnreadCount(context.TODO(), "test_user_3")
	require.Nil(t, err)
	require.Equal(t, int64(0), count)
}

func TestMarkAllAsRead(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ns := database.NewNotificationStoreWithDB(db)

	notification := &database.NotificationMessage{
		MsgUUID:          "test_uuid",
		SenderUUID:       "test_sender",
		NotificationType: "test_type",
		Title:            "test_title",
		Summary:          "test_summary",
		Content:          "test_content",
		ActionURL:        "test_url",
		Priority:         1,
	}

	err := ns.CreateNotificationMessageForUsers(context.TODO(), notification, []string{"test_user_1"})
	require.Nil(t, err)

	notification = &database.NotificationMessage{
		MsgUUID:          "test_uuid_1",
		SenderUUID:       "test_sender",
		NotificationType: "test_type",
		Title:            "test_title",
		Summary:          "test_summary",
		Content:          "test_content",
		ActionURL:        "test_url",
		Priority:         1,
	}

	err = ns.CreateNotificationMessageForUsers(context.TODO(), notification, []string{"test_user_1"})
	require.Nil(t, err)

	err = ns.MarkAllAsRead(context.TODO(), "test_user_1")
	require.Nil(t, err)

	count, err := ns.GetUnreadCount(context.TODO(), "test_user_1")
	require.Nil(t, err)
	require.Equal(t, int64(0), count)
}
