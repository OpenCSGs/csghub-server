package migrations

import (
	"context"
	"fmt"
	"time"

	"github.com/uptrace/bun"
	"opencsg.com/csghub-server/common/types"
)

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

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		_, err := db.ExecContext(ctx, `
			CREATE TYPE invitation_activity_status AS ENUM ('pending', 'awarded', 'failed');
		`)
		if err != nil {
			return fmt.Errorf("failed to create enum for invitation_activity_status, error: %w", err)
		}

		if err = createTables(ctx, db, Invitation{}, InvitationActivity{}); err != nil {
			return fmt.Errorf("failed to create table invitation/invitation_activity, error: %w", err)
		}

		_, err = db.NewCreateIndex().
			Model(&InvitationActivity{}).
			Index("idx_invitation_activity_inviter_uuid_inviter_status").
			Column("inviter_uuid").
			Column("inviter_status").
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("failed to create index for invitation_activity on invite_code and inviter_status, error: %w", err)
		}

		return nil
	}, func(ctx context.Context, db *bun.DB) error {
		_, err := db.NewDropIndex().
			Model(&InvitationActivity{}).
			Index("idx_invitation_activity_inviter_uuid_inviter_status").
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("failed to drop index for invitation_activity on inviter_uuid and inviter_status, error: %w", err)
		}

		if err := dropTables(ctx, db, Invitation{}, InvitationActivity{}); err != nil {
			return fmt.Errorf("failed to drop table invitation/invitation_activity, error: %w", err)
		}

		_, err = db.ExecContext(ctx, `
			DROP TYPE invitation_activity_status;
		`)
		if err != nil {
			return fmt.Errorf("failed to drop enum for invitation_activity_status, error: %w", err)
		}
		return nil
	})
}
