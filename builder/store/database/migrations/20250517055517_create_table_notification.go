package migrations

import (
	"context"
	"fmt"
	"time"

	"github.com/uptrace/bun"
)

type Notification struct {
	ID               int64     `bun:"column:id,pk,autoincrement"`
	UserUUID         string    `bun:"column:user_uuid,notnull"`
	SenderUUID       string    `bun:"column:sender_uuid,nullzero"`
	NotificationType string    `bun:"column:notification_type,notnull"`
	Title            string    `bun:"column:title,notnull"`
	Summary          string    `bun:"column:summary,nullzero"`
	Content          string    `bun:"column:content,notnull"`
	IsRead           bool      `bun:"column:is_read,notnull,default:false"`
	ActionURL        string    `bun:"column:action_url"`
	TaskID           int64     `bun:"column:task_id,nullzero"`
	Priority         int       `bun:"column:priority,notnull,default:0"`
	ExpireAt         time.Time `bun:"column:expire_at,type:timestamp"`
	times
}

type NotificationSetting struct {
	UserUUID                   string        `bun:"column:user_uuid,pk,notnull"`
	SubNotificationType        []string      `bun:"column:sub_notification_type"`
	IsEmailNotificationEnabled bool          `bun:"column:is_email_notification_enabled,notnull,default:false"`
	EmailAddress               string        `bun:"column:email_address"`
	MessageTTL                 time.Duration `bun:"column:message_ttl,default:604800"`
	IsSMSNotificationEnabled   bool          `bun:"column:is_sms_notification_enabled,notnull,default:false"`
	PhoneNumber                string        `bun:"column:phone_number"`
	DoNotDisturbStart          time.Time     `bun:"column:do_not_disturb_start,type:timestamp"`
	DoNotDisturbEnd            time.Time     `bun:"column:do_not_disturb_end,type:timestamp"`
	IsDoNotDisturbEnabled      bool          `bun:"column:is_do_not_disturb_enabled,notnull,default:false"`
	times
}

type NotificationTask struct {
	ID               int64    `bun:"column:id,pk,autoincrement"`
	MsgUUID          string   `bun:"column:msg_uuid,notnull,unique"`
	UserUUID         []string `bun:"column:user_uuid"`
	IsAllUsers       bool     `bun:"column:is_all_users,notnull,default:false"`
	NotificationType string   `bun:"column:notification_type,notnull"`
	SenderUUID       string   `bun:"column:sender_uuid"`
	Title            string   `bun:"column:title,notnull"`
	Summary          string   `bun:"column:summary,notnull"`
	Content          string   `bun:"column:content,notnull"`
	ActionURL        string   `bun:"column:action_url"`
	Priority         int      `bun:"column:priority,notnull,default:0"`
	Status           string   `bun:"column:status,notnull,default:'pending'"`
	times
}

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		if err := createTables(ctx, db, &Notification{}, &NotificationSetting{}, &NotificationTask{}); err != nil {
			return fmt.Errorf("create table notification, notification_setting, notification_task fail: %w", err)
		}

		queries := []string{
			"CREATE INDEX notifications_idx_useruuid_isread ON notifications (user_uuid, is_read);",
			"CREATE INDEX notifications_idx_useruuid_notificationtype ON notifications (user_uuid, notification_type);",
			"CREATE INDEX notifications_idx_useruuid_taskid ON notifications (user_uuid, task_id);",
			"CREATE INDEX notification_settings_idx_useruuid ON notification_settings (user_uuid);",
		}

		for _, query := range queries {
			if _, err := db.ExecContext(ctx, query); err != nil {
				return fmt.Errorf("create index fail: %w", err)
			}
		}

		return nil
	}, func(ctx context.Context, db *bun.DB) error {
		dropIndexQueries := []string{
			"DROP INDEX IF EXISTS notifications_idx_useruuid_isread;",
			"DROP INDEX IF EXISTS notifications_idx_useruuid_notificationtype;",
			"DROP INDEX IF EXISTS notifications_idx_useruuid_taskid;",
			"DROP INDEX IF EXISTS notification_settings_idx_useruuid;",
		}

		for _, query := range dropIndexQueries {
			if _, err := db.ExecContext(ctx, query); err != nil {
				return fmt.Errorf("drop index fail: %w", err)
			}
		}

		if err := dropTables(ctx, db, &Notification{}, &NotificationSetting{}, &NotificationTask{}); err != nil {
			return fmt.Errorf("drop table notification, notification_setting, notification_task fail: %w", err)
		}

		return nil
	})
}
