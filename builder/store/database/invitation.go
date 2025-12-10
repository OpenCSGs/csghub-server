package database

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/uptrace/bun"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
)

type InvitationStore interface {
	CreateInvitation(ctx context.Context, userUUID string, inviteCode string) error
	GetInvitationByUserUUID(ctx context.Context, userUUID string) (*Invitation, error)
	GetInvitationByInviteCode(ctx context.Context, inviteCode string) (*Invitation, error)
	GetInvitationActivityByID(ctx context.Context, activityID int64) (*InvitationActivity, error)
	ListInvitationActivities(ctx context.Context, req types.InvitationActivityFilter) ([]InvitationActivity, int, error)
	GetInvitationActivityByInviteeUUID(ctx context.Context, inviteeUUID string) (*InvitationActivity, error)
	CreateInvitationActivity(ctx context.Context, req types.CreateInvitationActivityReq) error
	UpdateInviteeStatus(ctx context.Context, inviteeUUID string, status types.InvitationActivityStatus) error
	AwardCreditToInviter(ctx context.Context, activityID int64) error
	MarkInviterCreditAsFailed(ctx context.Context, activityID int64) error
}

type InvitationStoreImpl struct {
	db *DB
}

func NewInvitationStore() InvitationStore {
	return &InvitationStoreImpl{db: defaultDB}
}

func NewInvitationStoreWithDB(db *DB) InvitationStore {
	return &InvitationStoreImpl{db: db}
}

type Invitation struct {
	ID            int64   `bun:",pk,autoincrement" json:"id"`
	InviteCode    string  `bun:",notnull,unique" json:"invite_code"`
	UserUUID      string  `bun:",notnull,unique" json:"user_uuid"`
	Invites       int64   `bun:",default:0" json:"invites"`
	TotalCredit   float64 `bun:",default:0" json:"total_credit"`
	PendingCredit float64 `bun:",default:0" json:"pending_credit"`
	times
}

type InvitationActivity struct {
	ID                  int64                          `bun:",pk,autoincrement" json:"id"`
	InviteCode          string                         `bun:",notnull" json:"invite_code"`
	InviterUUID         string                         `bun:",notnull" json:"inviter_uuid"`
	InviterName         string                         `bun:",notnull" json:"inviter_name"`
	InviteeUUID         string                         `bun:",notnull,unique" json:"invitee_uuid"`
	InviteeName         string                         `bun:",notnull" json:"invitee_name"`
	RegisterAt          time.Time                      `bun:",notnull" json:"register_at"`
	InviterCreditAmount float64                        `bun:",notnull" json:"inviter_credit_amount"`
	InviteeCreditAmount float64                        `bun:",notnull" json:"invitee_credit_amount"`
	InviterStatus       types.InvitationActivityStatus `bun:",type:invitation_activity_status,default:'pending'" json:"inviter_status"`
	InviteeStatus       types.InvitationActivityStatus `bun:",type:invitation_activity_status,default:'pending'" json:"invitee_status"`
	AwardAt             time.Time                      `bun:"," json:"award_at"`
	times
}

func (s *InvitationStoreImpl) CreateInvitation(ctx context.Context, userUUID string, inviteCode string) error {
	res, err := s.db.Core.NewInsert().Model(&Invitation{
		UserUUID:   userUUID,
		InviteCode: inviteCode,
	}).Exec(ctx)
	return assertAffectedOneRow(res, err)
}

func (s *InvitationStoreImpl) GetInvitationByUserUUID(ctx context.Context, userUUID string) (*Invitation, error) {
	var invitation Invitation
	err := s.db.Core.NewSelect().Model(&invitation).Where("user_uuid = ?", userUUID).Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, errorx.HandleDBError(err, nil)
	}

	return &invitation, nil
}

func (s *InvitationStoreImpl) GetInvitationByInviteCode(ctx context.Context, inviteCode string) (*Invitation, error) {
	var invitation Invitation
	err := s.db.Core.NewSelect().Model(&invitation).Where("invite_code = ?", inviteCode).Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, errorx.HandleDBError(err, nil)
	}

	return &invitation, nil
}

func (s *InvitationStoreImpl) GetInvitationActivityByID(ctx context.Context, activityID int64) (*InvitationActivity, error) {
	var activity InvitationActivity
	err := s.db.Core.NewSelect().Model(&activity).Where("id = ?", activityID).Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, errorx.HandleDBError(err, nil)
	}
	return &activity, err
}

func (s *InvitationStoreImpl) ListInvitationActivities(ctx context.Context, req types.InvitationActivityFilter) ([]InvitationActivity, int, error) {
	var activities []InvitationActivity
	q := s.db.Core.NewSelect().Model(&activities).
		Order("created_at DESC")

	q = s.applyInvitationActivitiesFilters(q, req)

	total, err := q.Count(ctx)
	if err != nil {
		return nil, 0, errorx.HandleDBError(err, nil)
	}

	if err := q.Limit(req.Per).Offset((req.Page - 1) * req.Per).Scan(ctx); err != nil {
		return nil, 0, errorx.HandleDBError(err, nil)
	}
	return activities, total, nil
}

func (s *InvitationStoreImpl) applyInvitationActivitiesFilters(q *bun.SelectQuery, req types.InvitationActivityFilter) *bun.SelectQuery {
	if req.InviterUUID != "" {
		q = q.Where("inviter_uuid = ?", req.InviterUUID)
	}

	if req.InviterStatus != "" {
		q = q.Where("inviter_status = ?", req.InviterStatus)
	}

	if req.InviteeStatus != "" {
		q = q.Where("invitee_status = ?", req.InviteeStatus)
	}

	if req.StartDate != "" {
		q = q.Where("created_at >= ?", req.StartDate)
	}

	if req.EndDate != "" {
		q = q.Where("created_at <= ?", req.EndDate)
	}

	return q
}

func (s *InvitationStoreImpl) GetInvitationActivityByInviteeUUID(ctx context.Context, inviteeUUID string) (*InvitationActivity, error) {
	var activity InvitationActivity
	err := s.db.Core.NewSelect().Model(&activity).Where("invitee_uuid = ?", inviteeUUID).Limit(1).Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, errorx.HandleDBError(err, nil)
	}
	return &activity, nil
}

func (s *InvitationStoreImpl) CreateInvitationActivity(ctx context.Context, req types.CreateInvitationActivityReq) error {
	return s.db.RunInTx(ctx, func(ctx context.Context, tx Operator) error {
		var inv Invitation
		if err := tx.Core.NewSelect().Model(&inv).
			Where("user_uuid = ?", req.InviterUUID).
			For("UPDATE NOWAIT").
			Scan(ctx); err != nil {
			return err
		}

		if err := assertAffectedOneRow(tx.Core.NewInsert().Model(&InvitationActivity{
			InviterUUID:         req.InviterUUID,
			InviteCode:          req.InviteCode,
			InviteeUUID:         req.InviteeUUID,
			InviteeName:         req.InviteeName,
			RegisterAt:          req.RegisterAt,
			InviterCreditAmount: req.InviterCreditAmount,
			InviteeCreditAmount: req.InviteeCreditAmount,
			InviterStatus:       types.InvitationActivityStatusPending,
			InviteeStatus:       types.InvitationActivityStatusAwarded,
			AwardAt:             req.AwardAt,
		}).Exec(ctx)); err != nil {
			return err
		}

		if err := assertAffectedOneRow(tx.Core.NewUpdate().Model((*Invitation)(nil)).
			Where("id = ?", inv.ID).
			Set("invites = invites + 1").
			Set("pending_credit = pending_credit + ?", req.InviterCreditAmount).
			Set("updated_at = now()").
			Exec(ctx)); err != nil {
			return err
		}
		return nil
	})
}

func (s *InvitationStoreImpl) UpdateInviteeStatus(ctx context.Context, inviteeUUID string, status types.InvitationActivityStatus) error {
	return s.db.RunInTx(ctx, func(ctx context.Context, tx Operator) error {
		var activity InvitationActivity
		err := tx.Core.NewSelect().Model(&activity).
			Where("invitee_uuid = ?", inviteeUUID).
			Where("inviter_status = ?", types.InvitationActivityStatusPending).
			Scan(ctx)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil
			}
			return err
		}
		if err := assertAffectedOneRow(tx.Core.NewUpdate().Model((*InvitationActivity)(nil)).
			Where("id = ?", activity.ID).
			Set("invitee_status = ?", status).
			Set("updated_at = now()").
			Exec(ctx)); err != nil {
			return err
		}
		return nil
	})
}

func (s *InvitationStoreImpl) AwardCreditToInviter(ctx context.Context, activityID int64) error {
	return s.db.RunInTx(ctx, func(ctx context.Context, tx Operator) error {
		var activity InvitationActivity
		err := tx.Core.NewSelect().Model(&activity).
			Where("id = ?", activityID).
			Where("inviter_status = ?", types.InvitationActivityStatusPending).
			Where("award_at IS NULL or award_at = '0001-01-01 00:00:00'").
			For("UPDATE NOWAIT").
			Scan(ctx)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil
			}
			return err
		}

		now := time.Now()
		err = assertAffectedOneRow(tx.Core.NewUpdate().Model((*InvitationActivity)(nil)).
			Where("id = ?", activityID).
			Set("inviter_status = ?", types.InvitationActivityStatusAwarded).
			Set("award_at = ?", now).
			Set("updated_at = now()").
			Exec(ctx))
		if err != nil {
			return err
		}

		var inv Invitation
		if err := tx.Core.NewSelect().Model(&inv).
			Where("user_uuid = ?", activity.InviterUUID).
			For("UPDATE NOWAIT").
			Scan(ctx); err != nil {
			return err
		}

		err = assertAffectedOneRow(tx.Core.NewUpdate().Model(&inv).
			Where("id = ?", inv.ID).
			Set("pending_credit = pending_credit - ?", activity.InviterCreditAmount).
			Set("total_credit = total_credit + ?", activity.InviterCreditAmount).
			Set("updated_at = now()").
			Exec(ctx))
		if err != nil {
			return err
		}

		return nil
	})
}

func (s *InvitationStoreImpl) MarkInviterCreditAsFailed(ctx context.Context, activityID int64) error {
	return s.db.RunInTx(ctx, func(ctx context.Context, tx Operator) error {
		var activity InvitationActivity
		err := tx.Core.NewSelect().Model(&activity).
			Where("id = ?", activityID).
			Where("inviter_status = ?", types.InvitationActivityStatusAwarded).
			Where("award_at IS NOT NULL").
			For("UPDATE NOWAIT").
			Scan(ctx)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil
			}
			return err
		}
		if err := assertAffectedOneRow(tx.Core.NewUpdate().Model((*InvitationActivity)(nil)).
			Where("id = ?", activity.ID).
			Set("inviter_status = ?", types.InvitationActivityStatusFailed).
			Set("updated_at = now()").
			Exec(ctx)); err != nil {
			return err
		}

		var inv Invitation
		if err := tx.Core.NewSelect().Model(&inv).
			Where("user_uuid = ?", activity.InviterUUID).
			For("UPDATE NOWAIT").
			Scan(ctx); err != nil {
			return err
		}

		err = assertAffectedOneRow(tx.Core.NewUpdate().Model(&inv).
			Where("id = ?", inv.ID).
			Set("pending_credit = pending_credit + ?", activity.InviterCreditAmount).
			Set("total_credit = total_credit - ?", activity.InviterCreditAmount).
			Set("updated_at = now()").
			Exec(ctx))
		if err != nil {
			return err
		}
		return nil
	})
}
