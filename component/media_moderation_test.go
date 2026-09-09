package component

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	mockrpc "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/rpc"
	mockdatabase "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
)

func mediaRefForTest() types.MediaRef {
	return types.MediaRef{
		URL:       "https://bucket.example/audio/a.mp3",
		Bucket:    "bucket",
		ObjectKey: "audio/a.mp3",
		Type:      types.MediaTypeAudio,
	}
}

func TestMediaModerationComponent_Check_AllPass(t *testing.T) {
	ref := mediaRefForTest()
	key, err := ref.ResourceKey()
	require.NoError(t, err)
	store := mockdatabase.NewMockMediaModerationStore(t)
	rpcClient := mockrpc.NewMockModerationSvcClient(t)
	c := NewMediaModerationComponent(store, rpcClient)

	store.EXPECT().FindByResourceKeys(mock.Anything, []string{key}).Return([]database.MediaModeration{
		{ResourceKey: key, DataID: "d1", MediaType: ref.Type, Status: database.MediaModerationStatusPass},
	}, nil)

	decision, err := c.Check(context.Background(), []types.MediaRef{ref})
	require.NoError(t, err)
	assert.Equal(t, types.MediaModerationDecisionPass, decision.Decision)
	assert.Empty(t, decision.Items)
}

func TestMediaModerationComponent_Check_Reject(t *testing.T) {
	ref := mediaRefForTest()
	key, _ := ref.ResourceKey()
	store := mockdatabase.NewMockMediaModerationStore(t)
	rpcClient := mockrpc.NewMockModerationSvcClient(t)
	c := NewMediaModerationComponent(store, rpcClient)

	store.EXPECT().FindByResourceKeys(mock.Anything, []string{key}).Return([]database.MediaModeration{
		{ResourceKey: key, DataID: "d1", MediaType: ref.Type, Status: database.MediaModerationStatusReject},
	}, nil)

	decision, err := c.Check(context.Background(), []types.MediaRef{ref})
	require.NoError(t, err)
	assert.Equal(t, types.MediaModerationDecisionReject, decision.Decision)
}

func TestMediaModerationComponent_Check_RejectsWholeBatchBeforeSubmittingNewResource(t *testing.T) {
	newRef := mediaRefForTest()
	rejectedRef := types.MediaRef{
		URL:       "https://bucket.example/video/rejected.mp4",
		Bucket:    "bucket",
		ObjectKey: "video/rejected.mp4",
		Type:      types.MediaTypeVideo,
	}
	newKey, err := newRef.ResourceKey()
	require.NoError(t, err)
	rejectedKey, err := rejectedRef.ResourceKey()
	require.NoError(t, err)
	store := mockdatabase.NewMockMediaModerationStore(t)
	rpcClient := mockrpc.NewMockModerationSvcClient(t)
	c := NewMediaModerationComponent(store, rpcClient)

	store.EXPECT().FindByResourceKeys(mock.Anything, []string{newKey, rejectedKey}).Return([]database.MediaModeration{
		{ResourceKey: rejectedKey, DataID: "rejected", MediaType: rejectedRef.Type, Status: database.MediaModerationStatusReject},
	}, nil)

	decision, err := c.Check(context.Background(), []types.MediaRef{newRef, rejectedRef})
	require.NoError(t, err)
	assert.Equal(t, types.MediaModerationDecisionReject, decision.Decision)
	store.AssertNotCalled(t, "CreateOrGet")
	rpcClient.AssertNotCalled(t, "SubmitMediaModeration")
}

func TestMediaModerationComponent_Check_PendingSurfacesExistingTask(t *testing.T) {
	ref := mediaRefForTest()
	key, _ := ref.ResourceKey()
	store := mockdatabase.NewMockMediaModerationStore(t)
	rpcClient := mockrpc.NewMockModerationSvcClient(t)
	c := NewMediaModerationComponent(store, rpcClient)

	store.EXPECT().FindByResourceKeys(mock.Anything, []string{key}).Return([]database.MediaModeration{
		{ResourceKey: key, DataID: "d1", TaskID: "task-existing", MediaType: ref.Type, Status: database.MediaModerationStatusPending},
	}, nil)

	decision, err := c.Check(context.Background(), []types.MediaRef{ref})
	require.NoError(t, err)
	assert.Equal(t, types.MediaModerationDecisionPending, decision.Decision)
	require.Len(t, decision.Items, 1)
	assert.Equal(t, "task-existing", decision.Items[0].TaskID)
	assert.Equal(t, "d1", decision.Items[0].DataID)
}

func TestMediaModerationComponent_Check_SubmitsNewAndPending(t *testing.T) {
	ref := mediaRefForTest()
	key, _ := ref.ResourceKey()
	store := mockdatabase.NewMockMediaModerationStore(t)
	rpcClient := mockrpc.NewMockModerationSvcClient(t)
	c := NewMediaModerationComponent(store, rpcClient)

	store.EXPECT().FindByResourceKeys(mock.Anything, []string{key}).Return(nil, nil)
	store.EXPECT().CreateOrGet(mock.Anything, mock.MatchedBy(func(m database.MediaModeration) bool {
		return m.ResourceKey == key && m.Status == database.MediaModerationStatusPending && m.LastAttemptAt != nil
	})).Return(&database.MediaModeration{ResourceKey: key, DataID: "d1", Seed: "s1", MediaType: ref.Type, Status: database.MediaModerationStatusPending}, true, nil)
	rpcClient.EXPECT().SubmitMediaModeration(mock.Anything, mock.MatchedBy(func(r types.MediaModerationRequest) bool {
		return r.DataID == "d1" && r.Seed == "s1" && r.URL == ref.URL
	})).Return(&types.MediaModerationSubmission{DataID: "d1", Seed: "s1", TaskID: "task-new"}, nil)
	store.EXPECT().MarkSubmitted(mock.Anything, "d1", "task-new", mock.Anything).Return(nil)

	decision, err := c.Check(context.Background(), []types.MediaRef{ref})
	require.NoError(t, err)
	assert.Equal(t, types.MediaModerationDecisionPending, decision.Decision)
	require.Len(t, decision.Items, 1)
	assert.Equal(t, "task-new", decision.Items[0].TaskID)
}

func TestMediaModerationComponent_CheckExisting_AllPassed(t *testing.T) {
	ref := mediaRefForTest()
	key, _ := ref.ResourceKey()
	store := mockdatabase.NewMockMediaModerationStore(t)
	rpcClient := mockrpc.NewMockModerationSvcClient(t)
	c := NewMediaModerationComponent(store, rpcClient)
	store.EXPECT().FindByResourceKeys(mock.Anything, []string{key}).Return([]database.MediaModeration{{
		ResourceKey: key, MediaType: ref.Type, Status: database.MediaModerationStatusPass,
	}}, nil)

	decision, err := c.CheckExisting(context.Background(), []types.MediaRef{ref})
	require.NoError(t, err)
	assert.Equal(t, types.MediaModerationDecisionPass, decision.Decision)
	rpcClient.AssertNotCalled(t, "SubmitMediaModeration")
}

func TestMediaModerationComponent_CheckExisting_MissingIsPendingWithoutSubmission(t *testing.T) {
	ref := mediaRefForTest()
	key, _ := ref.ResourceKey()
	store := mockdatabase.NewMockMediaModerationStore(t)
	rpcClient := mockrpc.NewMockModerationSvcClient(t)
	c := NewMediaModerationComponent(store, rpcClient)
	store.EXPECT().FindByResourceKeys(mock.Anything, []string{key}).Return(nil, nil)

	decision, err := c.CheckExisting(context.Background(), []types.MediaRef{ref})
	require.NoError(t, err)
	assert.Equal(t, types.MediaModerationDecisionPending, decision.Decision)
	rpcClient.AssertNotCalled(t, "SubmitMediaModeration")
}

func TestMediaModerationComponent_Check_RPCErrorFailsClosed(t *testing.T) {
	ref := mediaRefForTest()
	key, _ := ref.ResourceKey()
	store := mockdatabase.NewMockMediaModerationStore(t)
	rpcClient := mockrpc.NewMockModerationSvcClient(t)
	c := NewMediaModerationComponent(store, rpcClient)

	store.EXPECT().FindByResourceKeys(mock.Anything, []string{key}).Return(nil, nil)
	store.EXPECT().CreateOrGet(mock.Anything, mock.Anything).Return(&database.MediaModeration{ResourceKey: key, DataID: "d1", Seed: "s1", MediaType: ref.Type, Status: database.MediaModerationStatusPending}, true, nil)
	rpcClient.EXPECT().SubmitMediaModeration(mock.Anything, mock.Anything).Return(nil, errors.New("provider offline"))

	decision, err := c.Check(context.Background(), []types.MediaRef{ref})
	require.Error(t, err)
	assert.Equal(t, types.MediaModerationDecisionError, decision.Decision)
}

func TestMediaModerationComponent_Check_ErrorRowIsResubmitted(t *testing.T) {
	// A row previously errored must be claimed and resubmitted, not blocked
	// forever (P0-1).
	ref := mediaRefForTest()
	key, _ := ref.ResourceKey()
	store := mockdatabase.NewMockMediaModerationStore(t)
	rpcClient := mockrpc.NewMockModerationSvcClient(t)
	c := NewMediaModerationComponent(store, rpcClient)

	store.EXPECT().FindByResourceKeys(mock.Anything, []string{key}).Return([]database.MediaModeration{
		{ResourceKey: key, DataID: "d1", Seed: "s1", TaskID: "old-task", MediaType: ref.Type, Status: database.MediaModerationStatusError},
	}, nil)
	store.EXPECT().ClaimForResubmit(mock.Anything, "d1", mock.Anything, mock.Anything).Return(true, nil)
	rpcClient.EXPECT().SubmitMediaModeration(mock.Anything, mock.MatchedBy(func(r types.MediaModerationRequest) bool {
		return r.DataID == "d1" && r.Seed == "s1"
	})).Return(&types.MediaModerationSubmission{DataID: "d1", Seed: "s1", TaskID: "new-task"}, nil)
	store.EXPECT().MarkSubmitted(mock.Anything, "d1", "new-task", mock.Anything).Return(nil)

	decision, err := c.Check(context.Background(), []types.MediaRef{ref})
	require.NoError(t, err)
	assert.Equal(t, types.MediaModerationDecisionPending, decision.Decision)
	require.Len(t, decision.Items, 1)
	assert.Equal(t, "new-task", decision.Items[0].TaskID)
}

func TestMediaModerationComponent_Check_ErrorRowNotEligibleFailsClosed(t *testing.T) {
	// An errored row still within the resubmit lease must fail closed (not
	// create the comment in an unresolvable state).
	ref := mediaRefForTest()
	key, _ := ref.ResourceKey()
	store := mockdatabase.NewMockMediaModerationStore(t)
	rpcClient := mockrpc.NewMockModerationSvcClient(t)
	c := NewMediaModerationComponent(store, rpcClient)

	store.EXPECT().FindByResourceKeys(mock.Anything, []string{key}).Return([]database.MediaModeration{
		{ResourceKey: key, DataID: "d1", MediaType: ref.Type, Status: database.MediaModerationStatusError},
	}, nil)
	store.EXPECT().ClaimForResubmit(mock.Anything, "d1", mock.Anything, mock.Anything).Return(false, nil)

	decision, err := c.Check(context.Background(), []types.MediaRef{ref})
	require.Error(t, err)
	assert.Equal(t, types.MediaModerationDecisionError, decision.Decision)
}

func TestMediaModerationComponent_Check_PendingEmptyTaskIsResubmitted(t *testing.T) {
	// A pending row with an empty task_id (previous submit never confirmed,
	// P1-5) must be claimed and resubmitted, not surfaced with an empty task
	// that the workflow would poll forever.
	ref := mediaRefForTest()
	key, _ := ref.ResourceKey()
	store := mockdatabase.NewMockMediaModerationStore(t)
	rpcClient := mockrpc.NewMockModerationSvcClient(t)
	c := NewMediaModerationComponent(store, rpcClient)

	store.EXPECT().FindByResourceKeys(mock.Anything, []string{key}).Return([]database.MediaModeration{
		{ResourceKey: key, DataID: "d1", Seed: "s1", TaskID: "", MediaType: ref.Type, Status: database.MediaModerationStatusPending},
	}, nil)
	store.EXPECT().ClaimForResubmit(mock.Anything, "d1", mock.Anything, mock.Anything).Return(true, nil)
	rpcClient.EXPECT().SubmitMediaModeration(mock.Anything, mock.Anything).Return(&types.MediaModerationSubmission{DataID: "d1", Seed: "s1", TaskID: "task-1"}, nil)
	store.EXPECT().MarkSubmitted(mock.Anything, "d1", "task-1", mock.Anything).Return(nil)

	decision, err := c.Check(context.Background(), []types.MediaRef{ref})
	require.NoError(t, err)
	assert.Equal(t, types.MediaModerationDecisionPending, decision.Decision)
	require.Len(t, decision.Items, 1)
	assert.Equal(t, "task-1", decision.Items[0].TaskID)
}

func TestMediaModerationComponent_Check_PendingEmptyTaskNotClaimedFailsClosed(t *testing.T) {
	// If the empty-task pending row is held by another caller's lease, fail
	// closed: a workflow input containing an empty task ID can never recover.
	ref := mediaRefForTest()
	key, _ := ref.ResourceKey()
	store := mockdatabase.NewMockMediaModerationStore(t)
	rpcClient := mockrpc.NewMockModerationSvcClient(t)
	c := NewMediaModerationComponent(store, rpcClient)

	store.EXPECT().FindByResourceKeys(mock.Anything, []string{key}).Return([]database.MediaModeration{
		{ResourceKey: key, DataID: "d1", TaskID: "", MediaType: ref.Type, Status: database.MediaModerationStatusPending},
	}, nil)
	store.EXPECT().ClaimForResubmit(mock.Anything, "d1", mock.Anything, mock.Anything).Return(false, nil)

	decision, err := c.Check(context.Background(), []types.MediaRef{ref})
	require.Error(t, err)
	assert.Equal(t, types.MediaModerationDecisionError, decision.Decision)
	assert.Empty(t, decision.Items)
}

func TestMediaModerationComponent_Check_CreateRaceWithEmptyTaskFailsClosed(t *testing.T) {
	ref := mediaRefForTest()
	key, _ := ref.ResourceKey()
	store := mockdatabase.NewMockMediaModerationStore(t)
	rpcClient := mockrpc.NewMockModerationSvcClient(t)
	c := NewMediaModerationComponent(store, rpcClient)

	store.EXPECT().FindByResourceKeys(mock.Anything, []string{key}).Return(nil, nil)
	store.EXPECT().CreateOrGet(mock.Anything, mock.Anything).Return(&database.MediaModeration{
		ResourceKey: key, DataID: "d1", Seed: "s1", TaskID: "",
		MediaType: ref.Type, Status: database.MediaModerationStatusPending,
	}, false, nil)
	store.EXPECT().ClaimForResubmit(mock.Anything, "d1", mock.Anything, mock.Anything).Return(false, nil)

	decision, err := c.Check(context.Background(), []types.MediaRef{ref})
	require.Error(t, err)
	assert.Equal(t, types.MediaModerationDecisionError, decision.Decision)
	assert.Empty(t, decision.Items)
}

func TestMediaModerationComponent_LinkCommentMedia(t *testing.T) {
	store := mockdatabase.NewMockMediaModerationStore(t)
	rpcClient := mockrpc.NewMockModerationSvcClient(t)
	c := NewMediaModerationComponent(store, rpcClient)

	items := []types.CommentMediaItem{{DataID: "d1", TaskID: "t1", MediaType: types.MediaTypeAudio}}
	store.EXPECT().LinkCommentMedia(mock.Anything, int64(42), items).Return(nil)

	require.NoError(t, c.LinkCommentMedia(context.Background(), 42, items))
}

func TestMediaModerationComponent_LinkCommentMediaEmptyNoop(t *testing.T) {
	c := NewMediaModerationComponent(mockdatabase.NewMockMediaModerationStore(t), mockrpc.NewMockModerationSvcClient(t))
	require.NoError(t, c.LinkCommentMedia(context.Background(), 42, nil))
}
