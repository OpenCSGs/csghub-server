package database_test

import (
	"context"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/tests"
	"opencsg.com/csghub-server/common/types"
	"testing"
	"time"
)

func TestInvitationStorePendingCursor(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.Background()
	store := database.NewInvitationStoreWithDB(db)
	inviter := uuid.NewString()
	require.NoError(t, store.CreateInvitation(ctx, inviter, "cursor-test"))
	for range 2 {
		require.NoError(t, store.CreateInvitationActivity(ctx, types.CreateInvitationActivityReq{InviterUUID: inviter, InviteCode: "cursor-test", InviteeUUID: uuid.NewString(), InviteeName: "invitee", RegisterAt: time.Now()}))
	}

	first, total, err := store.ListInvitationActivities(ctx, types.InvitationActivityFilter{
		Page: 1, Per: 1, InviterStatus: types.InvitationActivityStatusPending, OrderByID: true,
	})
	require.NoError(t, err)
	require.Equal(t, 2, total)
	require.Len(t, first, 1)
	_, err = db.Core.NewUpdate().Model((*database.InvitationActivity)(nil)).
		Set("inviter_status = ?", types.InvitationActivityStatusAwarded).
		Where("id = ?", first[0].ID).Exec(ctx)
	require.NoError(t, err)
	second, total, err := store.ListInvitationActivities(ctx, types.InvitationActivityFilter{
		Page: 1, Per: 1, InviterStatus: types.InvitationActivityStatusPending,
		BeforeID: first[0].ID, OrderByID: true,
	})
	require.NoError(t, err)
	require.Equal(t, 1, total)
	require.Len(t, second, 1)
	require.Less(t, second[0].ID, first[0].ID)
}
