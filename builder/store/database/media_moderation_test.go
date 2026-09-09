package database_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/tests"
	"opencsg.com/csghub-server/common/types"
)

func TestMediaModerationStore(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()
	store := database.NewMediaModerationStoreWithDB(db)
	leaseAt := time.Now().UTC().Truncate(time.Microsecond)

	t.Run("create or get is idempotent by resource key", func(t *testing.T) {
		m := database.MediaModeration{
			ResourceKey:   "rk-1",
			DataID:        "data-1",
			Seed:          "seed-1",
			MediaType:     types.MediaTypeAudio,
			Status:        database.MediaModerationStatusPending,
			LastAttemptAt: &leaseAt,
		}
		created, isNew, err := store.CreateOrGet(ctx, m)
		require.NoError(t, err)
		require.True(t, isNew)
		require.NotZero(t, created.ID)

		dup := m
		dup.DataID = "must-not-replace"
		existing, isNew, err := store.CreateOrGet(ctx, dup)
		require.NoError(t, err)
		require.False(t, isNew)
		require.Equal(t, created.ID, existing.ID)
		require.Equal(t, "data-1", existing.DataID)
	})

	t.Run("find by resource keys and data id", func(t *testing.T) {
		_, _, err := store.CreateOrGet(ctx, database.MediaModeration{
			ResourceKey: "rk-2", DataID: "data-2", Seed: "seed-2",
			MediaType: types.MediaTypeVideo, Status: database.MediaModerationStatusPending,
		})
		require.NoError(t, err)
		rows, err := store.FindByResourceKeys(ctx, []string{"rk-1", "rk-2"})
		require.NoError(t, err)
		require.Len(t, rows, 2)
		byDataID, err := store.FindByDataID(ctx, "data-2")
		require.NoError(t, err)
		require.Equal(t, "rk-2", byDataID.ResourceKey)
	})

	t.Run("mark submitted persists task id", func(t *testing.T) {
		require.NoError(t, store.MarkSubmitted(ctx, "data-1", "task-1", leaseAt))
		row, err := store.FindByDataID(ctx, "data-1")
		require.NoError(t, err)
		require.Equal(t, "task-1", row.TaskID)
		require.NotNil(t, row.SubmittedAt)
	})

	t.Run("mark submitted requires lease ownership", func(t *testing.T) {
		wrongLease := leaseAt.Add(time.Second)
		err := store.MarkSubmitted(ctx, "data-1", "must-not-overwrite", wrongLease)
		require.Error(t, err)
		row, findErr := store.FindByDataID(ctx, "data-1")
		require.NoError(t, findErr)
		require.Equal(t, "task-1", row.TaskID)
	})

	t.Run("update result transitions pending to terminal", func(t *testing.T) {
		require.NoError(t, store.UpdateResult(ctx, "data-2", "", database.MediaModerationStatusPass, ""))
		row, err := store.FindByDataID(ctx, "data-2")
		require.NoError(t, err)
		require.Equal(t, database.MediaModerationStatusPass, row.Status)
	})

	t.Run("update result rejects non-terminal status", func(t *testing.T) {
		err := store.UpdateResult(ctx, "data-2", "", database.MediaModerationStatusPending, "")
		require.Error(t, err)
	})

	t.Run("stale task result cannot update resubmitted resource", func(t *testing.T) {
		_, _, err := store.CreateOrGet(ctx, database.MediaModeration{
			ResourceKey: "rk-resubmitted", DataID: "data-resubmitted", Seed: "seed-resubmitted",
			TaskID: "new-task", MediaType: types.MediaTypeVideo, Status: database.MediaModerationStatusPending,
		})
		require.NoError(t, err)
		require.NoError(t, store.UpdateResult(ctx, "data-resubmitted", "old-task", database.MediaModerationStatusPass, ""))
		row, err := store.FindByDataID(ctx, "data-resubmitted")
		require.NoError(t, err)
		require.Equal(t, database.MediaModerationStatusPending, row.Status)
		require.Equal(t, "new-task", row.TaskID)
	})

	t.Run("link comment media is idempotent", func(t *testing.T) {
		items := []types.CommentMediaItem{
			{DataID: "data-1", TaskID: "task-1", MediaType: types.MediaTypeAudio},
			{DataID: "data-2", TaskID: "", MediaType: types.MediaTypeVideo},
		}
		require.NoError(t, store.LinkCommentMedia(ctx, 100, items))
		require.NoError(t, store.LinkCommentMedia(ctx, 100, items)) // no-op
		views, err := store.FindMediaByCommentIDs(ctx, []int64{100})
		require.NoError(t, err)
		require.Len(t, views, 2)
	})

	t.Run("two comments share one pending data id independently", func(t *testing.T) {
		_, _, err := store.CreateOrGet(ctx, database.MediaModeration{
			ResourceKey: "rk-shared", DataID: "data-shared", Seed: "seed-shared",
			MediaType: types.MediaTypeAudio, Status: database.MediaModerationStatusPending, LastAttemptAt: &leaseAt,
		})
		require.NoError(t, err)
		require.NoError(t, store.MarkSubmitted(ctx, "data-shared", "task-shared", leaseAt))
		shared := types.CommentMediaItem{DataID: "data-shared", TaskID: "task-shared", MediaType: types.MediaTypeAudio}
		require.NoError(t, store.LinkCommentMedia(ctx, 200, []types.CommentMediaItem{shared}))
		require.NoError(t, store.LinkCommentMedia(ctx, 201, []types.CommentMediaItem{shared}))
		views, err := store.FindMediaByCommentIDs(ctx, []int64{200, 201, 999})
		require.NoError(t, err)
		require.Len(t, views, 2)
		require.ElementsMatch(t, []int64{200, 201}, []int64{views[0].CommentID, views[1].CommentID})
		for _, v := range views {
			require.Equal(t, "data-shared", v.DataID)
			require.Equal(t, "task-shared", v.TaskID)
			require.Equal(t, database.MediaModerationStatusPending, v.Status)
		}
	})

	t.Run("find media by empty comment ids returns empty", func(t *testing.T) {
		views, err := store.FindMediaByCommentIDs(ctx, nil)
		require.NoError(t, err)
		require.Empty(t, views)
	})

	t.Run("claim for resubmit resets error row", func(t *testing.T) {
		// A row in 'error' status is eligible for resubmission after the lease.
		_, _, err := store.CreateOrGet(ctx, database.MediaModeration{
			ResourceKey: "rk-err", DataID: "data-err", Seed: "seed-err",
			MediaType: types.MediaTypeAudio, Status: database.MediaModerationStatusError,
		})
		require.NoError(t, err)
		// UpdateResult only transitions pending/error→terminal, so set error via UpdateResult.
		require.NoError(t, store.UpdateResult(ctx, "data-err", "", database.MediaModerationStatusError, "boom"))
		errRow, _ := store.FindByDataID(ctx, "data-err")
		require.Equal(t, database.MediaModerationStatusError, errRow.Status)

		// Claim succeeds when last_attempt_at is older than the lease.
		claimAt := time.Now().UTC().Truncate(time.Microsecond)
		claimed, err := store.ClaimForResubmit(ctx, "data-err", claimAt.Add(time.Second), claimAt)
		require.NoError(t, err)
		require.True(t, claimed)
		after, _ := store.FindByDataID(ctx, "data-err")
		require.Equal(t, database.MediaModerationStatusPending, after.Status)
		require.Empty(t, after.TaskID)
		require.WithinDuration(t, claimAt, *after.LastAttemptAt, time.Microsecond)

		// A second claim fails because last_attempt_at was just refreshed to
		// now; staleBefore must be in the past for the lease to hold.
		claimed2, err := store.ClaimForResubmit(ctx, "data-err", claimAt.Add(-time.Second), claimAt.Add(time.Second))
		require.NoError(t, err)
		require.False(t, claimed2)
	})

	t.Run("claim for resubmit does not touch submitted pending", func(t *testing.T) {
		// A pending row with a task_id is already submitted; not eligible.
		claimed, err := store.ClaimForResubmit(ctx, "data-1", time.Now().Add(-time.Hour), time.Now().UTC())
		require.NoError(t, err)
		require.False(t, claimed)
		row, _ := store.FindByDataID(ctx, "data-1")
		require.Equal(t, "task-1", row.TaskID)
	})

	t.Run("orphan comment_media row returns empty status (fail-closed)", func(t *testing.T) {
		// Link a comment to a data_id that has no media_moderations row.
		// LEFT JOIN must still return the row with an empty status so the
		// visibility logic treats it as under-moderation (hidden), not public.
		require.NoError(t, store.LinkCommentMedia(ctx, 7777, []types.CommentMediaItem{
			{DataID: "data-orphan", TaskID: "", MediaType: types.MediaTypeAudio},
		}))
		views, err := store.FindMediaByCommentIDs(ctx, []int64{7777})
		require.NoError(t, err)
		require.Len(t, views, 1)
		require.Equal(t, int64(7777), views[0].CommentID)
		require.Equal(t, "data-orphan", views[0].DataID)
		require.Empty(t, views[0].Status, "orphan row must have empty status so it is treated as under-moderation")
	})
}
