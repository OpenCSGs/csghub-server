package migrations

import (
	"context"
	"fmt"
	"time"

	"github.com/uptrace/bun"
)

type NotificationMessage struct {
	ID               int64  `bun:"column:id,pk,autoincrement"`
	MsgUUID          string `bun:"column:msg_uuid,notnull,unique"`
	GroupID          string `bun:"column:group_id"`
	NotificationType string `bun:"column:notification_type,notnull"`
	SenderUUID       string `bun:"column:sender_uuid"`
	Summary          string `bun:"column:summary,notnull"`
	Title            string `bun:"column:title,notnull"`
	Content          string `bun:"column:content,notnull"`
	ActionURL        string `bun:"column:action_url"`
	Priority         int    `bun:"column:priority,notnull,default:0"`
	times
}

type NotificationUserMessage struct {
	ID         int64     `bun:"column:id,pk,autoincrement"`
	MsgUUID    string    `bun:"column:msg_uuid,notnull"`
	UserUUID   string    `bun:"column:user_uuid,notnull"`
	ReadAt     time.Time `bun:"column:read_at,type:timestamp"`
	IsNotified bool      `bun:"column:is_notified,notnull,default:false"`
	ExpireAt   time.Time `bun:"column:expire_at,type:timestamp"`
	times
}

type NotificationUserMessageView struct {
	ID               int64     `bun:"column:id"`
	MsgUUID          string    `bun:"column:msg_uuid"`
	NotificationType string    `bun:"column:notification_type"`
	SenderUUID       string    `bun:"column:sender_uuid"`
	Summary          string    `bun:"column:summary"`
	Title            string    `bun:"column:title"`
	Content          string    `bun:"column:content"`
	ActionURL        string    `bun:"column:action_url"`
	Priority         int       `bun:"column:priority"`
	UserUUID         string    `bun:"column:user_uuid"`
	ReadAt           time.Time `bun:"column:read_at"`
	IsNotified       bool      `bun:"column:is_notified"`
	ExpireAt         time.Time `bun:"column:expire_at,type:timestamp"`
	times
}

type NotificationUserMessageErrorLog struct {
	ID       int64  `bun:"column:id,pk,autoincrement"`
	MsgUUID  string `bun:"column:msg_uuid,notnull"`
	UserUUID string `bun:"column:user_uuid,notnull"`
	ErrorMsg string `bun:"column:error_msg"`
	Resolved bool   `bun:"column:resolved,notnull,default:false"`
	times
}

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		if err := createTables(ctx, db, &NotificationMessage{}, &NotificationUserMessage{}, &NotificationUserMessageErrorLog{}); err != nil {
			return fmt.Errorf("create table notification_message, notification_user_message, notification_user_message_error_log fail: %w", err)
		}

		if _, err := db.ExecContext(ctx, `
			CREATE INDEX idx_notification_user_messages_user_uuid_msg_uuid ON notification_user_messages (user_uuid, msg_uuid);
		`); err != nil {
			return fmt.Errorf("create index idx_notification_user_messages_user_uuid_msg_uuid fail: %w", err)
		}

		if _, err := db.ExecContext(ctx, `
			ALTER TABLE notification_user_messages ADD CONSTRAINT notification_user_messages_msg_user_unique UNIQUE (msg_uuid, user_uuid);
		`); err != nil {
			return fmt.Errorf("add constraint notification_user_messages_msg_user_unique fail: %w", err)
		}

		if _, err := db.ExecContext(ctx, `
			CREATE OR REPLACE VIEW notification_user_message_views AS
			SELECT nmu.id, nmu.msg_uuid, nm.group_id, nm.notification_type, nm.sender_uuid,
				nm.summary, nm.title, nm.content, nm.action_url, nm.priority,
				nmu.user_uuid, nmu.read_at, nmu.is_notified, nmu.expire_at, nm.created_at, nm.updated_at
			FROM notification_user_messages nmu
			LEFT JOIN notification_messages nm ON nmu.msg_uuid = nm.msg_uuid;
		`); err != nil {
			return fmt.Errorf("create view notification_user_message_views fail: %w", err)
		}

		return nil
	}, func(ctx context.Context, db *bun.DB) error {
		if _, err := db.ExecContext(ctx, `
			DROP VIEW IF EXISTS notification_user_message_views;
		`); err != nil {
			return fmt.Errorf("drop view notification_user_message_views fail: %w", err)
		}

		if _, err := db.ExecContext(ctx, `
			ALTER TABLE notification_user_messages DROP CONSTRAINT notification_user_messages_msg_user_unique;
		`); err != nil {
			return fmt.Errorf("drop constraint notification_user_messages_msg_user_unique fail: %w", err)
		}

		if _, err := db.ExecContext(ctx, `
			DROP INDEX IF EXISTS idx_notification_user_messages_user_uuid_msg_uuid;
		`); err != nil {
			return fmt.Errorf("drop index idx_notification_user_messages_user_uuid_msg_uuid fail: %w", err)
		}

		if err := dropTables(ctx, db, &NotificationMessage{}, &NotificationUserMessage{}, &NotificationUserMessageErrorLog{}); err != nil {
			return fmt.Errorf("drop table notification_message, notification_user_message, notification_user_message_error_log fail: %w", err)
		}

		return nil
	})
}
