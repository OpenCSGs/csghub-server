package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
	"opencsg.com/csghub-server/common/types"
)

type accountPresentStoreImpl struct {
	db *DB
}

type AccountPresentStore interface {
	AddPresent(ctx context.Context, input AccountPresent, statement AccountStatement) error
	FindPresentByUserAndActivity(ctx context.Context, userID string, activityID int64, participantUUID string) (*AccountPresent, error)
	ListExpiredPresentsByActivityID(ctx context.Context, activityID int64) ([]AccountPresent, error)
	HasConsumptionActivity(ctx context.Context, userUUID string, startAt time.Time) (bool, error)
	MarkPresentAsUsed(ctx context.Context, eventUUID uuid.UUID) error
	CancelPresent(ctx context.Context, eventUUID uuid.UUID) error
}

func NewAccountPresentStore() AccountPresentStore {
	return &accountPresentStoreImpl{
		db: defaultDB,
	}
}

func NewAccountPresentStoreWithDB(db *DB) AccountPresentStore {
	return &accountPresentStoreImpl{
		db: db,
	}
}

type AccountPresent struct {
	ID              int64                      `bun:",pk,autoincrement" json:"id"`
	EventUUID       uuid.UUID                  `bun:"type:uuid,notnull" json:"event_uuid"`
	UserUUID        string                     `bun:",notnull" json:"user_uuid"`
	ActivityID      int64                      `bun:",notnull" json:"activity_id"`
	Value           float64                    `bun:",notnull" json:"value"`
	OpUID           string                     `bun:",notnull" json:"op_uid"`
	OpDesc          string                     `bun:",notnull" json:"op_desc"`
	ParticipantUUID string                     `bun:",notnull" json:"participant_uuid"`
	ExpireAt        time.Time                  `bun:",nullzero" json:"expire_at"`
	Status          types.AccountPresentStatus `bun:",default:0" json:"status"`
	times
}

func (ap *accountPresentStoreImpl) AddPresent(ctx context.Context, input AccountPresent, statement AccountStatement) error {
	err := ap.db.Operator.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if err := assertAffectedOneRow(tx.NewInsert().Model(&input).Exec(ctx)); err != nil {
			return fmt.Errorf("insert account present, error:%w", err)
		}

		if err := assertAffectedOneRow(tx.NewInsert().Model(&statement).Exec(ctx)); err != nil {
			return fmt.Errorf("insert account statement, error:%w", err)
		}

		var accountUser AccountUser
		if err := tx.NewSelect().Model(&accountUser).Where("user_uuid = ?", input.UserUUID).For("UPDATE NOWAIT").Scan(ctx, &accountUser); err != nil {
			return fmt.Errorf("update user account to add present, error:%w", err)
		}

		runSql := "update account_users set balance=balance + ? where user_uuid=?"
		if err := assertAffectedOneRow(tx.Exec(runSql, input.Value, input.UserUUID)); err != nil {
			return fmt.Errorf("update account balance, error:%w", err)
		}

		return nil
	})

	if err != nil {
		slog.Error("failed to add present", slog.Int64("activity_id", input.ActivityID), slog.String("user_uuid", input.UserUUID), slog.Any("error", err))
	}

	return err
}

func (ap *accountPresentStoreImpl) FindPresentByUserAndActivity(ctx context.Context, userID string, activityID int64, participantUUID string) (*AccountPresent, error) {
	present := &AccountPresent{}
	q := ap.db.Core.NewSelect().Model(present).Where("user_uuid = ? and activity_id = ?", userID, activityID)
	if participantUUID != "" {
		q = q.Where("participant_uuid = ?", participantUUID)
	}

	if err := q.Limit(1).Scan(ctx, present); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		slog.Error("failed to find account present by user id and activity id", slog.String("user_id", userID), slog.Int64("activity_id", activityID), slog.Any("error", err))
		return nil, fmt.Errorf("find account present by user id and activity id, error:%w", err)
	}
	return present, nil
}

func (ap *accountPresentStoreImpl) ListExpiredPresentsByActivityID(ctx context.Context, activityID int64) ([]AccountPresent, error) {
	var presents []AccountPresent
	err := ap.db.Core.NewSelect().Model(&presents).
		Where("activity_id = ?", activityID).
		Where("status = ?", types.AccountPresentStatusInit).
		Where("expire_at < ?", time.Now()).
		Where("expire_at is not null").
		Scan(ctx, &presents)
	if err != nil {
		slog.Error("failed to list expired presents by activity id", slog.Int64("activity_id", activityID), slog.Any("error", err))
		return nil, fmt.Errorf("list expired presents by activity id, activity_id:%d, error:%w", activityID, err)
	}
	return presents, nil
}

func (ap *accountPresentStoreImpl) HasConsumptionActivity(ctx context.Context, userUUID string, startAt time.Time) (bool, error) {
	consumptionScenes := []types.SceneType{
		types.ScenePayOrder,
		types.ScenePaySubscription,
		types.SceneModelInference,
		types.SceneSpace,
		types.SceneModelFinetune,
		types.SceneEvaluation,
		types.SceneModelServerless,
		types.SceneStarship,
		types.SceneGuiAgent,
	}
	count, err := ap.db.Core.NewSelect().Model(&AccountStatement{}).
		Where("user_uuid = ?", userUUID).
		Where("created_at >= ?", startAt).
		Where("scene in (?)", bun.In(consumptionScenes)).
		Count(ctx)
	if err != nil {
		slog.Error("failed to check present consumed", slog.String("user_uuid", userUUID), slog.Time("start_at", startAt), slog.Any("error", err))
		return false, fmt.Errorf("check present consumed, user_uuid:%s, start_at:%s, error:%w", userUUID, startAt, err)
	}
	return count > 0, nil
}

func (ap *accountPresentStoreImpl) MarkPresentAsUsed(ctx context.Context, eventUUID uuid.UUID) error {
	_, err := ap.db.Operator.Core.NewUpdate().Model(&AccountPresent{}).Set("status = ?", types.AccountPresentStatusUsed).Where("event_uuid = ?", eventUUID).Exec(ctx)
	if err != nil {
		slog.Error("failed to mark present as used", slog.String("event_uuid", eventUUID.String()), slog.Any("error", err))
		return fmt.Errorf("mark present as used, event_uuid:%s, error:%w", eventUUID.String(), err)
	}
	return nil
}

func (ap *accountPresentStoreImpl) CancelPresent(ctx context.Context, eventUUID uuid.UUID) error {
	err := ap.db.Operator.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		var present AccountPresent
		if err := tx.NewSelect().Model(&present).Where("event_uuid = ?", eventUUID).Scan(ctx, &present); err != nil {
			return fmt.Errorf("get account present, event_uuid:%s, error:%w", eventUUID.String(), err)
		}

		if err := assertAffectedOneRow(tx.NewUpdate().Model(&AccountPresent{}).Set("status = ?", types.AccountPresentStatusCanceled).Where("event_uuid = ?", eventUUID).Exec(ctx)); err != nil {
			return fmt.Errorf("update account present status, event_uuid:%s, error:%w", eventUUID.String(), err)
		}

		if err := assertAffectedOneRow(tx.NewUpdate().Model(&AccountStatement{}).Set("is_cancel = ?", true).Where("event_uuid = ?", eventUUID).Exec(ctx)); err != nil {
			return fmt.Errorf("update account statement to cancel, event_uuid:%s, error:%w", eventUUID.String(), err)
		}

		var accountUser AccountUser
		if err := tx.NewSelect().Model(&accountUser).Where("user_uuid = ?", present.UserUUID).For("UPDATE NOWAIT").Scan(ctx, &accountUser); err != nil {
			return fmt.Errorf("update user account to cancel, user_uuid:%s, error:%w", present.UserUUID, err)
		}

		runSql := "update account_users set balance=balance - ? where user_uuid=?"
		if err := assertAffectedOneRow(tx.Exec(runSql, present.Value, present.UserUUID)); err != nil {
			return fmt.Errorf("update account balance, user_uuid:%s, error:%w", present.UserUUID, err)
		}

		return nil
	})

	if err != nil {
		slog.Error("failed to cancel present", slog.String("event_uuid", eventUUID.String()), slog.Any("error", err))
	}
	return err
}
