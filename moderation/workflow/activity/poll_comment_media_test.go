package activity

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/testsuite"
	mockrpc "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/rpc"
	mockdb "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/builder/rpc"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/moderation/workflow/common"
)

// withMockDeps swaps the package-level factories for mock-backed ones and
// returns a restore function.
func withMockDeps(t *testing.T, mediaStore *mockdb.MockMediaModerationStore, rpcClient *mockrpc.MockModerationSvcClient, discussionStore *mockdb.MockDiscussionStore) func() {
	t.Helper()
	origMedia, origRPC, origDiscussion, origConfig := newMediaModerationStore, newModerationRPCClient, newDiscussionStore, runtimeConfig
	newMediaModerationStore = func() database.MediaModerationStore { return mediaStore }
	newModerationRPCClient = func(*config.Config) rpc.ModerationSvcClient { return rpcClient }
	newDiscussionStore = func() database.DiscussionStore { return discussionStore }
	runtimeConfig = &config.Config{}
	return func() {
		newMediaModerationStore = origMedia
		newModerationRPCClient = origRPC
		newDiscussionStore = origDiscussion
		runtimeConfig = origConfig
	}
}

func TestPollCommentMediaModerationResultActivity_Pass(t *testing.T) {
	mediaStore := mockdb.NewMockMediaModerationStore(t)
	rpcClient := mockrpc.NewMockModerationSvcClient(t)
	discussionStore := mockdb.NewMockDiscussionStore(t)
	restore := withMockDeps(t, mediaStore, rpcClient, discussionStore)
	defer restore()

	item := common.PollCommentMediaItem{DataID: "d1", TaskID: "t1", MediaType: types.MediaTypeAudio}
	rpcClient.EXPECT().QueryMediaModerationResult(mock.Anything, types.MediaModerationRequest{
		Type: types.MediaTypeAudio, DataID: "d1", TaskID: "t1",
	}).Return(&types.MediaModerationResult{DataID: "d1", Status: common.MediaModerationStatusPass}, nil)
	mediaStore.EXPECT().UpdateResult(mock.Anything, "d1", "t1", common.MediaModerationStatusPass, "").Return(nil)

	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestActivityEnvironment()
	env.RegisterActivity(PollCommentMediaModerationResultActivity)
	var status string
	result, err := env.ExecuteActivity(PollCommentMediaModerationResultActivity, item)
	require.NoError(t, err)
	require.NoError(t, result.Get(&status))
	assert.Equal(t, common.MediaModerationStatusPass, status)
}

func TestPollCommentMediaModerationResultActivity_Pending(t *testing.T) {
	mediaStore := mockdb.NewMockMediaModerationStore(t)
	rpcClient := mockrpc.NewMockModerationSvcClient(t)
	discussionStore := mockdb.NewMockDiscussionStore(t)
	restore := withMockDeps(t, mediaStore, rpcClient, discussionStore)
	defer restore()

	item := common.PollCommentMediaItem{DataID: "d1", TaskID: "t1", MediaType: types.MediaTypeAudio}
	rpcClient.EXPECT().QueryMediaModerationResult(mock.Anything, mock.MatchedBy(func(_ types.MediaModerationRequest) bool { return true })).Return(&types.MediaModerationResult{Status: common.MediaModerationStatusPending}, nil)

	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestActivityEnvironment()
	env.RegisterActivity(PollCommentMediaModerationResultActivity)
	var status string
	result, err := env.ExecuteActivity(PollCommentMediaModerationResultActivity, item)
	require.NoError(t, err)
	require.NoError(t, result.Get(&status))
	assert.Equal(t, common.MediaModerationStatusPending, status)
}

func TestPollCommentMediaModerationResultActivity_Reject(t *testing.T) {
	mediaStore := mockdb.NewMockMediaModerationStore(t)
	rpcClient := mockrpc.NewMockModerationSvcClient(t)
	discussionStore := mockdb.NewMockDiscussionStore(t)
	restore := withMockDeps(t, mediaStore, rpcClient, discussionStore)
	defer restore()

	item := common.PollCommentMediaItem{DataID: "d1", TaskID: "t1", MediaType: types.MediaTypeVideo}
	rpcClient.EXPECT().QueryMediaModerationResult(mock.Anything, mock.MatchedBy(func(_ types.MediaModerationRequest) bool { return true })).Return(&types.MediaModerationResult{Status: common.MediaModerationStatusReject, Reason: "sensitive"}, nil)
	mediaStore.EXPECT().UpdateResult(mock.Anything, "d1", "t1", common.MediaModerationStatusReject, "sensitive").Return(nil)

	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestActivityEnvironment()
	env.RegisterActivity(PollCommentMediaModerationResultActivity)
	var status string
	result, err := env.ExecuteActivity(PollCommentMediaModerationResultActivity, item)
	require.NoError(t, err)
	require.NoError(t, result.Get(&status))
	assert.Equal(t, common.MediaModerationStatusReject, status)
}

func TestPollCommentMediaModerationResultActivity_UnknownStatusPreservesProviderValue(t *testing.T) {
	mediaStore := mockdb.NewMockMediaModerationStore(t)
	rpcClient := mockrpc.NewMockModerationSvcClient(t)
	discussionStore := mockdb.NewMockDiscussionStore(t)
	restore := withMockDeps(t, mediaStore, rpcClient, discussionStore)
	defer restore()

	item := common.PollCommentMediaItem{DataID: "d1", TaskID: "t1", MediaType: types.MediaTypeAudio}
	rpcClient.EXPECT().QueryMediaModerationResult(mock.Anything, mock.Anything).
		Return(&types.MediaModerationResult{Status: "provider_unknown"}, nil)
	mediaStore.EXPECT().UpdateResult(mock.Anything, "d1", "t1", common.MediaModerationStatusError,
		`unknown media moderation status "provider_unknown"`).Return(nil)

	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestActivityEnvironment()
	env.RegisterActivity(PollCommentMediaModerationResultActivity)
	var status string
	result, err := env.ExecuteActivity(PollCommentMediaModerationResultActivity, item)
	require.NoError(t, err)
	require.NoError(t, result.Get(&status))
	assert.Equal(t, common.MediaModerationStatusError, status)
}

func TestPollCommentMediaModerationResultActivity_RPCError(t *testing.T) {
	mediaStore := mockdb.NewMockMediaModerationStore(t)
	rpcClient := mockrpc.NewMockModerationSvcClient(t)
	discussionStore := mockdb.NewMockDiscussionStore(t)
	restore := withMockDeps(t, mediaStore, rpcClient, discussionStore)
	defer restore()

	item := common.PollCommentMediaItem{DataID: "d1", TaskID: "t1", MediaType: types.MediaTypeAudio}
	rpcClient.EXPECT().QueryMediaModerationResult(mock.Anything, mock.MatchedBy(func(_ types.MediaModerationRequest) bool { return true })).Return(nil, errors.New("provider offline"))

	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestActivityEnvironment()
	env.RegisterActivity(PollCommentMediaModerationResultActivity)
	_, err := env.ExecuteActivity(PollCommentMediaModerationResultActivity, item)
	require.Error(t, err)
}

func TestFinalizeCommentMediaModerationActivity_KeepsOnAllPass(t *testing.T) {
	mediaStore := mockdb.NewMockMediaModerationStore(t)
	rpcClient := mockrpc.NewMockModerationSvcClient(t)
	discussionStore := mockdb.NewMockDiscussionStore(t)
	restore := withMockDeps(t, mediaStore, rpcClient, discussionStore)
	defer restore()

	mediaStore.EXPECT().FindMediaByCommentIDs(mock.Anything, []int64{42}).Return([]database.CommentMediaView{
		{CommentID: 42, Status: database.MediaModerationStatusPass},
		{CommentID: 42, Status: database.MediaModerationStatusPass},
	}, nil)
	discussionStore.AssertNotCalled(t, "DeleteComment")

	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestActivityEnvironment()
	env.RegisterActivity(FinalizeCommentMediaModerationActivity)
	_, err := env.ExecuteActivity(FinalizeCommentMediaModerationActivity, int64(42))
	require.NoError(t, err)
}

func TestFinalizeCommentMediaModerationActivity_DeletesOnReject(t *testing.T) {
	mediaStore := mockdb.NewMockMediaModerationStore(t)
	rpcClient := mockrpc.NewMockModerationSvcClient(t)
	discussionStore := mockdb.NewMockDiscussionStore(t)
	restore := withMockDeps(t, mediaStore, rpcClient, discussionStore)
	defer restore()

	mediaStore.EXPECT().FindMediaByCommentIDs(mock.Anything, []int64{42}).Return([]database.CommentMediaView{
		{CommentID: 42, Status: database.MediaModerationStatusPass},
		{CommentID: 42, Status: database.MediaModerationStatusReject},
	}, nil)
	discussionStore.EXPECT().DeleteComment(mock.Anything, int64(42)).Return(nil)

	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestActivityEnvironment()
	env.RegisterActivity(FinalizeCommentMediaModerationActivity)
	_, err := env.ExecuteActivity(FinalizeCommentMediaModerationActivity, int64(42))
	require.NoError(t, err)
}

func TestFinalizeCommentMediaModerationActivity_DeletesOnPendingTimeout(t *testing.T) {
	mediaStore := mockdb.NewMockMediaModerationStore(t)
	rpcClient := mockrpc.NewMockModerationSvcClient(t)
	discussionStore := mockdb.NewMockDiscussionStore(t)
	restore := withMockDeps(t, mediaStore, rpcClient, discussionStore)
	defer restore()

	mediaStore.EXPECT().FindMediaByCommentIDs(mock.Anything, []int64{42}).Return([]database.CommentMediaView{
		{CommentID: 42, Status: database.MediaModerationStatusPending},
	}, nil)
	discussionStore.EXPECT().DeleteComment(mock.Anything, int64(42)).Return(nil)

	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestActivityEnvironment()
	env.RegisterActivity(FinalizeCommentMediaModerationActivity)
	_, err := env.ExecuteActivity(FinalizeCommentMediaModerationActivity, int64(42))
	require.NoError(t, err)
}

func TestDeleteCommentAfterFinalizeFailureActivity(t *testing.T) {
	mediaStore := mockdb.NewMockMediaModerationStore(t)
	rpcClient := mockrpc.NewMockModerationSvcClient(t)
	discussionStore := mockdb.NewMockDiscussionStore(t)
	restore := withMockDeps(t, mediaStore, rpcClient, discussionStore)
	defer restore()

	discussionStore.EXPECT().DeleteComment(mock.Anything, int64(42)).Return(nil)

	require.NoError(t, DeleteCommentAfterFinalizeFailureActivity(context.Background(), 42))
}

func TestDeleteCommentAfterFinalizeFailureActivity_AlreadyDeletedIsSuccess(t *testing.T) {
	mediaStore := mockdb.NewMockMediaModerationStore(t)
	rpcClient := mockrpc.NewMockModerationSvcClient(t)
	discussionStore := mockdb.NewMockDiscussionStore(t)
	restore := withMockDeps(t, mediaStore, rpcClient, discussionStore)
	defer restore()

	discussionStore.EXPECT().DeleteComment(mock.Anything, int64(42)).Return(sql.ErrNoRows)

	require.NoError(t, DeleteCommentAfterFinalizeFailureActivity(context.Background(), 42))
}

func TestFinalizeCommentMediaModerationActivity_KeepsWhenNoRows(t *testing.T) {
	mediaStore := mockdb.NewMockMediaModerationStore(t)
	rpcClient := mockrpc.NewMockModerationSvcClient(t)
	discussionStore := mockdb.NewMockDiscussionStore(t)
	restore := withMockDeps(t, mediaStore, rpcClient, discussionStore)
	defer restore()

	mediaStore.EXPECT().FindMediaByCommentIDs(mock.Anything, []int64{42}).Return([]database.CommentMediaView{}, nil)

	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestActivityEnvironment()
	env.RegisterActivity(FinalizeCommentMediaModerationActivity)
	_, err := env.ExecuteActivity(FinalizeCommentMediaModerationActivity, int64(42))
	require.NoError(t, err)
}

func TestSendApprovedCommentNotificationActivity(t *testing.T) {
	mediaStore := mockdb.NewMockMediaModerationStore(t)
	notificationClient := mockrpc.NewMockNotificationSvcClient(t)
	originalFactory, originalMediaFactory, originalConfig := newNotificationRPCClient, newMediaModerationStore, runtimeConfig
	newNotificationRPCClient = func(*config.Config) rpc.NotificationSvcClient { return notificationClient }
	newMediaModerationStore = func() database.MediaModerationStore { return mediaStore }
	runtimeConfig = &config.Config{}
	defer func() {
		newNotificationRPCClient = originalFactory
		newMediaModerationStore = originalMediaFactory
		runtimeConfig = originalConfig
	}()

	notification := types.CommentNotification{
		CommentID: 42, MsgUUID: "msg-1", RepoType: types.ModelRepo, RepoPath: "owner/repo",
		SenderUUID: "sender", RecipientUUID: "recipient",
	}
	mediaStore.EXPECT().FindMediaByCommentIDs(mock.Anything, []int64{42}).Return([]database.CommentMediaView{
		{CommentID: 42, Status: database.MediaModerationStatusPass},
	}, nil)
	notificationClient.EXPECT().Send(mock.Anything, mock.MatchedBy(func(req *types.MessageRequest) bool {
		return req.Scenario == types.MessageScenarioDiscussion && strings.Contains(req.Parameters, `"msg_uuid":"msg-1"`)
	})).Return(nil)

	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestActivityEnvironment()
	env.RegisterActivity(SendApprovedCommentNotificationActivity)
	_, err := env.ExecuteActivity(SendApprovedCommentNotificationActivity, notification)
	require.NoError(t, err)
}

func TestSendApprovedCommentNotificationActivity_SkipsRejectedComment(t *testing.T) {
	mediaStore := mockdb.NewMockMediaModerationStore(t)
	notificationClient := mockrpc.NewMockNotificationSvcClient(t)
	originalNotificationFactory, originalMediaFactory, originalConfig := newNotificationRPCClient, newMediaModerationStore, runtimeConfig
	newNotificationRPCClient = func(*config.Config) rpc.NotificationSvcClient { return notificationClient }
	newMediaModerationStore = func() database.MediaModerationStore { return mediaStore }
	runtimeConfig = &config.Config{}
	defer func() {
		newNotificationRPCClient = originalNotificationFactory
		newMediaModerationStore = originalMediaFactory
		runtimeConfig = originalConfig
	}()
	mediaStore.EXPECT().FindMediaByCommentIDs(mock.Anything, []int64{42}).Return([]database.CommentMediaView{
		{CommentID: 42, Status: database.MediaModerationStatusReject},
	}, nil)

	err := SendApprovedCommentNotificationActivity(context.Background(), types.CommentNotification{CommentID: 42})
	require.NoError(t, err)
	notificationClient.AssertNotCalled(t, "Send")
}
