package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/uptrace/bun"
)

const DefaultMessageTTL = 7 * 24 * time.Hour
const MaxErrorMsgLength = 255

type ListNotificationsParams struct {
	UserUUID         string
	NotificationType string
	TitleKeyword     string
	Page             int
	PageSize         int
	UnreadOnly       bool
}

type NotificationStore interface {
	GetSetting(ctx context.Context, uid string) (*NotificationSetting, error)
	GetSettingByUserUUIDs(ctx context.Context, userUUID []string) ([]*NotificationSetting, error)
	GetAllSettings(ctx context.Context) ([]*NotificationSetting, error)
	GetEmails(ctx context.Context, per, page int) ([]string, int, error)
	UpdateSetting(ctx context.Context, setting *NotificationSetting) error
	CreateSetting(ctx context.Context, setting *NotificationSetting) error

	GetUnreadCount(ctx context.Context, uid string) (int64, error)
	MarkAsRead(ctx context.Context, uid string, ids []int64) error
	MarkAsUnread(ctx context.Context, uid string, ids []int64) error
	MarkAllAsRead(ctx context.Context, uid string) error
	MarkAllAsUnread(ctx context.Context, uid string) error
	DeleteNotifications(ctx context.Context, uid string, ids []int64) error
	DeleteAllNotifications(ctx context.Context, uid string) error
	CreateNotificationMessageForUsers(ctx context.Context, msg *NotificationMessage, userUUIDs []string) error
	IsNotificationMessageExists(ctx context.Context, msgUUID string) (bool, error)
	CreateNotificationMessage(ctx context.Context, msg *NotificationMessage) error
	CreateUserMessages(ctx context.Context, msgUUID string, userUUIDs []string) ([]NotificationUserMessageErrorLog, error)
	CreateUserMessageErrorLog(ctx context.Context, msgUUID string, userUUID string, errorMsg string) error
	ListNotificationMessages(ctx context.Context, params ListNotificationsParams) ([]NotificationUserMessageView, int, error)
	GetUnNotifiedMessages(ctx context.Context, userUUID string, limit int) ([]NotificationUserMessageView, int, error)
	MarkAsNotified(ctx context.Context, userUUID string, msgUUIDs []string) error
}

func NewNotificationStore() NotificationStore {
	return &NotificationStoreImpl{
		db: defaultDB,
	}
}

func NewNotificationStoreWithDB(db *DB) NotificationStore {
	return &NotificationStoreImpl{
		db: db,
	}
}

type NotificationStoreImpl struct {
	db *DB
}

var _ NotificationStore = (*NotificationStoreImpl)(nil)

// GetSetting
func (s *NotificationStoreImpl) GetSetting(ctx context.Context, uid string) (*NotificationSetting, error) {
	var setting NotificationSetting
	err := s.db.Operator.Core.NewSelect().
		Model(&setting).
		Where("user_uuid = ?", uid).
		Scan(ctx)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &setting, nil
}

// GetSettingByUserUUIDs
func (s *NotificationStoreImpl) GetSettingByUserUUIDs(ctx context.Context, userUUID []string) ([]*NotificationSetting, error) {
	var settings []*NotificationSetting
	err := s.db.Operator.Core.NewSelect().
		Model(&settings).
		Where("user_uuid IN (?)", bun.In(userUUID)).
		Scan(ctx)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return settings, nil
}

// GetAllSettings
func (s *NotificationStoreImpl) GetAllSettings(ctx context.Context) ([]*NotificationSetting, error) {
	var settings []*NotificationSetting
	err := s.db.Operator.Core.NewSelect().
		Model(&settings).
		Scan(ctx)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
	}
	return settings, nil
}

// UpdateSetting
func (s *NotificationStoreImpl) UpdateSetting(ctx context.Context, setting *NotificationSetting) error {
	res, err := s.db.Operator.Core.NewUpdate().
		Model(setting).
		WherePK().
		Exec(ctx)
	if err != nil {
		return err
	}

	if err = assertAffectedOneRow(res, err); err != nil {
		return fmt.Errorf("update notification setting failed,error:%w", err)
	}
	return nil
}

// CreateSetting
func (s *NotificationStoreImpl) CreateSetting(ctx context.Context, setting *NotificationSetting) error {
	res, err := s.db.Operator.Core.NewInsert().
		Model(setting).
		Exec(ctx)
	if err != nil {
		return err
	}

	if err = assertAffectedOneRow(res, err); err != nil {
		return fmt.Errorf("create notification setting failed,error:%w", err)
	}
	return nil
}

func (s *NotificationStoreImpl) GetUnreadCount(ctx context.Context, uid string) (int64, error) {
	var count int
	count, err := s.db.Operator.Core.NewSelect().
		Model(&NotificationUserMessage{}).
		Where("user_uuid = ?", uid).
		Where("read_at IS NULL OR read_at = '0001-01-01 00:00:00'").
		Where("expire_at > ?", time.Now()).
		Count(ctx)
	if err != nil {
		return 0, err
	}
	return int64(count), nil
}

func (s *NotificationStoreImpl) MarkAsRead(ctx context.Context, uid string, ids []int64) error {
	_, err := s.db.Operator.Core.NewUpdate().
		Model(&NotificationUserMessage{}).
		Where("user_uuid =?", uid).
		Where("id IN (?)", bun.In(ids)).
		Set("read_at = ?", time.Now()).
		Exec(ctx)
	if err != nil {
		return err
	}
	return nil
}

func (s *NotificationStoreImpl) MarkAsUnread(ctx context.Context, uid string, ids []int64) error {
	_, err := s.db.Operator.Core.NewUpdate().
		Model(&NotificationUserMessage{}).
		Where("user_uuid =?", uid).
		Where("id IN (?)", bun.In(ids)).
		Set("read_at = ?", nil).
		Exec(ctx)
	if err != nil {
		return err
	}
	return nil
}

func (s *NotificationStoreImpl) MarkAllAsRead(ctx context.Context, uid string) error {
	_, err := s.db.Operator.Core.NewUpdate().
		Model(&NotificationUserMessage{}).
		Where("user_uuid =?", uid).
		Where("read_at IS NULL OR read_at = '0001-01-01 00:00:00'").
		Where("expire_at > ?", time.Now()).
		Set("read_at = ?", time.Now()).
		Exec(ctx)
	if err != nil {
		return err
	}
	return nil
}

func (s *NotificationStoreImpl) MarkAllAsUnread(ctx context.Context, uid string) error {
	_, err := s.db.Operator.Core.NewUpdate().
		Model(&NotificationUserMessage{}).
		Where("user_uuid =?", uid).
		Where("read_at IS NOT NULL AND read_at != '0001-01-01 00:00:00'").
		Where("expire_at > ?", time.Now()).
		Set("read_at = ?", nil).
		Exec(ctx)
	if err != nil {
		return err
	}
	return nil
}

func (s *NotificationStoreImpl) DeleteNotifications(ctx context.Context, uid string, ids []int64) error {
	_, err := s.db.Operator.Core.NewDelete().
		Model(&NotificationUserMessage{}).
		Where("user_uuid =?", uid).
		Where("id IN (?)", bun.In(ids)).
		Exec(ctx)
	if err != nil {
		return err
	}
	return nil
}

func (s *NotificationStoreImpl) DeleteAllNotifications(ctx context.Context, uid string) error {
	_, err := s.db.Operator.Core.NewDelete().
		Model(&NotificationUserMessage{}).
		Where("user_uuid =?", uid).
		Where("expire_at > ?", time.Now()).
		Exec(ctx)
	if err != nil {
		return err
	}
	return nil
}

func (s *NotificationStoreImpl) ListNotificationMessages(ctx context.Context, params ListNotificationsParams) ([]NotificationUserMessageView, int, error) {
	offset := (params.Page - 1) * params.PageSize

	query := s.db.Operator.Core.NewSelect().
		Model(&NotificationUserMessageView{}).
		Where("user_uuid = ?", params.UserUUID).
		Where("expire_at > ?", time.Now())

	if params.NotificationType != "" {
		query = query.Where("notification_type =?", params.NotificationType)
	}
	if params.TitleKeyword != "" {
		query = query.Where("title ILIKE ?", "%"+params.TitleKeyword+"%")
	}
	if params.UnreadOnly {
		query = query.Where("read_at IS NULL OR read_at = '0001-01-01 00:00:00'")
	}

	total, err := query.Count(ctx)
	if err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return nil, 0, nil
	}

	var notificationViews []NotificationUserMessageView

	query = query.Order("priority DESC", "created_at DESC").
		Offset(offset).
		Limit(params.PageSize)

	if err := query.Scan(ctx, &notificationViews); err != nil {
		return nil, 0, err
	}

	return notificationViews, total, nil
}

func (s *NotificationStoreImpl) CreateNotificationMessageForUsers(ctx context.Context, msg *NotificationMessage, userUUIDs []string) error {
	if msg == nil || len(userUUIDs) == 0 {
		return fmt.Errorf("invalid input: msg or userUUIDs is empty")
	}

	tx, err := s.db.Operator.Core.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin tx: %w", err)
	}

	_, err = tx.NewInsert().
		Model(msg).
		On("CONFLICT (msg_uuid) DO NOTHING").
		Exec(ctx)
	if err != nil {
		err = tx.Rollback()
		if err != nil {
			return fmt.Errorf("failed to rollback tx: %w", err)
		}
		return fmt.Errorf("failed to insert notification message: %w", err)
	}

	for _, userUUID := range userUUIDs {
		userSetting, err := s.GetSetting(ctx, userUUID)
		if err != nil {
			err = tx.Rollback()
			if err != nil {
				return fmt.Errorf("failed to rollback tx: %w", err)
			}
			return fmt.Errorf("failed to get user setting: %w", err)
		}
		expireAt := time.Now().Add(DefaultMessageTTL)
		if userSetting != nil && userSetting.MessageTTL > 0 {
			expireAt = time.Now().Add(userSetting.MessageTTL)
		}
		_, err = tx.NewInsert().
			Model(&NotificationUserMessage{
				MsgUUID:    msg.MsgUUID,
				UserUUID:   userUUID,
				IsNotified: false,
				ExpireAt:   expireAt,
			}).
			On("CONFLICT (msg_uuid, user_uuid) DO NOTHING").
			Exec(ctx)
		if err != nil {
			err = tx.Rollback()
			if err != nil {
				return fmt.Errorf("failed to rollback tx: %w", err)
			}
			return fmt.Errorf("failed to insert notification user message: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		err = tx.Rollback()
		if err != nil {
			return fmt.Errorf("failed to rollback tx: %w", err)
		}
		return fmt.Errorf("failed to commit tx: %w", err)
	}

	return nil
}

func (s *NotificationStoreImpl) IsNotificationMessageExists(ctx context.Context, msgUUID string) (bool, error) {
	count, err := s.db.Operator.Core.NewSelect().
		Model(&NotificationMessage{}).
		Where("msg_uuid = ?", msgUUID).
		Count(ctx)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (s *NotificationStoreImpl) CreateNotificationMessage(ctx context.Context, msg *NotificationMessage) error {
	res, err := s.db.Operator.Core.NewInsert().
		Model(msg).
		Exec(ctx)
	if err != nil {
		return err
	}

	if err = assertAffectedOneRow(res, err); err != nil {
		return fmt.Errorf("create notification message failed, error: %w", err)
	}
	return nil
}

func truncateErrorMsg(err error, maxLength int) string {
	if err == nil {
		return ""
	}
	msg := err.Error()
	if len(msg) <= maxLength {
		return msg
	}
	return msg[:maxLength-3] + "..."
}

func (s *NotificationStoreImpl) CreateUserMessages(ctx context.Context, msgUUID string, userUUIDs []string) ([]NotificationUserMessageErrorLog, error) {
	errorLogs := make([]NotificationUserMessageErrorLog, 0, len(userUUIDs))

	for _, userUUID := range userUUIDs {
		userSetting, err := s.GetSetting(ctx, userUUID)
		if err != nil {
			return nil, err
		}
		expireAt := time.Now().Add(DefaultMessageTTL)
		if userSetting != nil {
			expireAt = time.Now().Add(userSetting.MessageTTL)
		}
		_, err = s.db.Operator.Core.NewInsert().
			Model(&NotificationUserMessage{
				MsgUUID:  msgUUID,
				UserUUID: userUUID,
				ExpireAt: expireAt,
			}).
			On("CONFLICT (msg_uuid, user_uuid) DO NOTHING").
			Exec(ctx)
		if err != nil {
			errorLogs = append(errorLogs, NotificationUserMessageErrorLog{
				MsgUUID:  msgUUID,
				UserUUID: userUUID,
				ErrorMsg: truncateErrorMsg(err, MaxErrorMsgLength),
			})
		}
	}

	return errorLogs, nil
}

func (s *NotificationStoreImpl) CreateUserMessageErrorLog(ctx context.Context, msgUUID string, userUUID string, errorMsg string) error {
	res, err := s.db.Operator.Core.NewInsert().
		Model(&NotificationUserMessageErrorLog{
			MsgUUID:  msgUUID,
			UserUUID: userUUID,
			ErrorMsg: errorMsg,
		}).
		Exec(ctx)
	if err != nil {
		return err
	}

	if err = assertAffectedOneRow(res, err); err != nil {
		return fmt.Errorf("create user message error log failed, error: %w", err)
	}
	return nil
}

func (s *NotificationStoreImpl) GetUnNotifiedMessages(ctx context.Context, userUUID string, limit int) ([]NotificationUserMessageView, int, error) {
	query := s.db.Operator.Core.NewSelect().
		Model(&NotificationUserMessageView{}).
		Where("user_uuid = ?", userUUID).
		Where("is_notified = ?", false).
		Order("priority DESC", "created_at DESC").
		Where("expire_at > ?", time.Now()).
		Limit(limit)

	total, err := query.Count(ctx)
	if err != nil {
		return nil, 0, err
	}

	var notificationViews []NotificationUserMessageView
	if err := query.Scan(ctx, &notificationViews); err != nil {
		return nil, 0, err
	}

	return notificationViews, total, nil
}

func (s *NotificationStoreImpl) MarkAsNotified(ctx context.Context, userUUID string, msgUUIDs []string) error {
	_, err := s.db.Operator.Core.NewUpdate().
		Model(&NotificationUserMessage{}).
		Where("user_uuid = ?", userUUID).
		Where("msg_uuid IN (?)", bun.In(msgUUIDs)).
		Set("is_notified = ?", true).
		Exec(ctx)
	if err != nil {
		return err
	}
	return nil
}

func (s *NotificationStoreImpl) GetEmails(ctx context.Context, per, page int) ([]string, int, error) {
	query := s.db.Operator.Core.NewSelect().
		Model(&NotificationSetting{}).
		Column("email_address").
		Where("email_address IS NOT NULL AND email_address != ''")
	count, err := query.Count(ctx)
	if err != nil {
		return nil, 0, err
	}
	query = query.Order("user_uuid ASC").Limit(per).Offset((page - 1) * per)
	var emails []string
	err = query.Scan(ctx, &emails)
	if err != nil {
		return nil, 0, err
	}
	return emails, count, nil
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
	DoNotDisturbStartTime      time.Time     `bun:"column:do_not_disturb_start_time,type:time"`
	DoNotDisturbEndTime        time.Time     `bun:"column:do_not_disturb_end_time,type:time"`
	IsDoNotDisturbEnabled      bool          `bun:"column:is_do_not_disturb_enabled,notnull,default:false"`
	times
}

type NotificationMessage struct {
	ID               int64          `bun:"column:id,pk,autoincrement"`
	MsgUUID          string         `bun:"column:msg_uuid,notnull,unique"`
	GroupID          string         `bun:"column:group_id"`
	NotificationType string         `bun:"column:notification_type,notnull"`
	SenderUUID       string         `bun:"column:sender_uuid"`
	Summary          string         `bun:"column:summary,notnull"`
	Title            string         `bun:"column:title,notnull"`
	Content          string         `bun:"column:content,notnull"`
	Template         string         `bun:"column:template"`
	Payload          map[string]any `bun:"column:payload,type:jsonb"`
	ActionURL        string         `bun:"column:action_url"`
	Priority         int            `bun:"column:priority,notnull,default:0"`
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
	ID               int64          `bun:"column:id"`
	MsgUUID          string         `bun:"column:msg_uuid"`
	NotificationType string         `bun:"column:notification_type"`
	SenderUUID       string         `bun:"column:sender_uuid"`
	Summary          string         `bun:"column:summary"`
	Title            string         `bun:"column:title"`
	Content          string         `bun:"column:content"`
	Template         string         `bun:"column:template"`
	Payload          map[string]any `bun:"column:payload,type:jsonb"`
	ActionURL        string         `bun:"column:action_url"`
	Priority         int            `bun:"column:priority"`
	UserUUID         string         `bun:"column:user_uuid"`
	ReadAt           time.Time      `bun:"column:read_at"`
	IsNotified       bool           `bun:"column:is_notified"`
	ExpireAt         time.Time      `bun:"column:expire_at,type:timestamp"`
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
