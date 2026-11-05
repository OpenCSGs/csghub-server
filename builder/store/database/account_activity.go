package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/uptrace/bun"
	"opencsg.com/csghub-server/common/types"
)

type AccountActivity struct {
	ID            int64                `bun:",pk,autoincrement" json:"id"`
	ActID         int64                `bun:",notnull,unique" json:"act_id"`
	ActCode       string               `bun:",notnull,unique" json:"act_code"`
	ActName       string               `bun:",notnull" json:"act_name"`
	Status        types.ActivityStatus `bun:",notnull,default:'active'" json:"status"` // active, expired
	BeginAt       time.Time            `bun:",nullzero" json:"begin_at"`
	EndAt         time.Time            `bun:",nullzero" json:"end_at"`
	Description   string               `bun:",nullzero" json:"description"`
	VoucherValue  int64                `bun:",notnull,default:0" json:"voucher_value"`
	ValidDuration int64                `bun:",nullzero" json:"valid_duration"` // valid duration in seconds
}

// ErrActivityNotGrantable reports that the activity cannot grant a present at
// the requested moment: it does not exist, is not active, or now is outside
// its configured [begin_at, end_at] window.
var ErrActivityNotGrantable = errors.New("activity is not grantable")

// findGrantableActivity loads the activity by its business id and verifies it
// may grant a present at now. A set begin_at must be at or before now and a
// set end_at must not be before now; a zero (unset) bound is open, so an
// activity without a window stays grantable while it is active.
func findGrantableActivity(ctx context.Context, db bun.IDB, activityID int64, now time.Time) (*AccountActivity, error) {
	var activity AccountActivity
	err := db.NewSelect().Model(&activity).Where("act_id = ?", activityID).Scan(ctx)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return nil, fmt.Errorf("%w: activity %d not found", ErrActivityNotGrantable, activityID)
	case err != nil:
		return nil, fmt.Errorf("find activity %d, error:%w", activityID, err)
	}

	if activity.Status != types.ActivityStatusActive {
		return nil, fmt.Errorf("%w: activity %d is %s", ErrActivityNotGrantable, activityID, activity.Status)
	}

	if !activity.BeginAt.IsZero() && now.Before(activity.BeginAt) {
		return nil, fmt.Errorf("%w: activity %d begins at %s",
			ErrActivityNotGrantable, activityID, activity.BeginAt.Format(time.RFC3339))
	}

	if !activity.EndAt.IsZero() && activity.EndAt.Before(now) {
		return nil, fmt.Errorf("%w: activity %d ended at %s",
			ErrActivityNotGrantable, activityID, activity.EndAt.Format(time.RFC3339))
	}
	return &activity, nil
}
