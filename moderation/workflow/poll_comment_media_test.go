package workflow

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/testsuite"
	"go.temporal.io/sdk/workflow"
	mocktemporal "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/temporal"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/moderation/workflow/activity"
	"opencsg.com/csghub-server/moderation/workflow/common"
)

func TestStartCommentMediaModerationWorkflow_ReservesFinalizeAndDeleteBudget(t *testing.T) {
	temporalClient := mocktemporal.NewMockClient(t)
	timing := common.PollCommentMediaOptions{WorkflowTimeout: 30 * time.Minute}
	req := common.PollCommentMediaReq{CommentID: 42, Items: []common.PollCommentMediaItem{{
		DataID: "d1", TaskID: "t1", MediaType: types.MediaTypeAudio,
	}}}
	temporalClient.EXPECT().ExecuteWorkflow(
		mock.Anything,
		mock.MatchedBy(func(options client.StartWorkflowOptions) bool {
			return options.WorkflowExecutionTimeout >= timing.WorkflowTimeout+2*finalizeScheduleTimeout
		}),
		common.PollCommentMediaWorkflowName,
		req,
		timing,
	).Return(nil, nil)

	require.NoError(t, StartCommentMediaModerationWorkflow(context.Background(), temporalClient, req, timing))
}

func newPollTestEnv(t *testing.T) (*testsuite.TestWorkflowEnvironment, common.PollCommentMediaOptions) {
	t.Helper()
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()
	env.RegisterActivity(activity.PollCommentMediaModerationResultActivity)
	env.RegisterActivity(activity.FinalizeCommentMediaModerationActivity)
	env.RegisterActivity(activity.DeleteCommentAfterFinalizeFailureActivity)
	opts := common.PollCommentMediaOptions{PollInterval: time.Second, WorkflowTimeout: 30 * time.Second}
	env.RegisterWorkflowWithOptions(PollCommentMediaModerationWorkflow, workflow.RegisterOptions{
		Name: common.PollCommentMediaWorkflowName,
	})
	return env, opts
}

func TestPollCommentMediaWorkflow_AllPassKeepsComment(t *testing.T) {
	env, opts := newPollTestEnv(t)
	item := common.PollCommentMediaItem{DataID: "d1", TaskID: "t1", MediaType: types.MediaTypeAudio}
	req := common.PollCommentMediaReq{CommentID: 42, Items: []common.PollCommentMediaItem{item}}

	// First poll returns pending, second returns pass.
	env.OnActivity(activity.PollCommentMediaModerationResultActivity, mock.Anything, item).
		Return(common.MediaModerationStatusPending, nil).Times(1)
	env.OnActivity(activity.PollCommentMediaModerationResultActivity, mock.Anything, mock.Anything).
		Return(common.MediaModerationStatusPass, nil)
	// Finalize: all pass → keep (no delete).
	env.OnActivity(activity.FinalizeCommentMediaModerationActivity, mock.Anything, int64(42)).
		Return(nil)

	env.ExecuteWorkflow(common.PollCommentMediaWorkflowName, req, opts)
	require.NoError(t, env.GetWorkflowError())
}

func TestPollCommentMediaWorkflow_AllPassSendsDeferredNotification(t *testing.T) {
	env, opts := newPollTestEnv(t)
	item := common.PollCommentMediaItem{DataID: "d1", TaskID: "t1", MediaType: types.MediaTypeAudio}
	notification := &types.CommentNotification{
		CommentID: 42, MsgUUID: "msg-1", RepoType: types.ModelRepo, RepoPath: "owner/repo",
		SenderUUID: "sender", RecipientUUID: "recipient",
	}
	req := common.PollCommentMediaReq{CommentID: 42, Items: []common.PollCommentMediaItem{item}, Notification: notification}

	env.OnActivity(activity.PollCommentMediaModerationResultActivity, mock.Anything, item).
		Return(common.MediaModerationStatusPass, nil)
	env.OnActivity(activity.FinalizeCommentMediaModerationActivity, mock.Anything, int64(42)).
		Return(nil)
	env.OnActivity(activity.SendApprovedCommentNotificationActivity, mock.Anything, *notification).
		Return(nil).Once()

	env.ExecuteWorkflow(common.PollCommentMediaWorkflowName, req, opts)
	require.NoError(t, env.GetWorkflowError())
}

func TestPollCommentMediaWorkflow_RejectDeletesComment(t *testing.T) {
	env, opts := newPollTestEnv(t)
	item := common.PollCommentMediaItem{DataID: "d1", TaskID: "t1", MediaType: types.MediaTypeAudio}
	req := common.PollCommentMediaReq{CommentID: 42, Items: []common.PollCommentMediaItem{item}}

	env.OnActivity(activity.PollCommentMediaModerationResultActivity, mock.Anything, item).
		Return(common.MediaModerationStatusReject, nil)
	env.OnActivity(activity.FinalizeCommentMediaModerationActivity, mock.Anything, int64(42)).
		Return(nil)

	env.ExecuteWorkflow(common.PollCommentMediaWorkflowName, req, opts)
	require.NoError(t, env.GetWorkflowError())
}

func TestPollCommentMediaWorkflow_FinalizeRetriesTransientFailure(t *testing.T) {
	env, opts := newPollTestEnv(t)
	item := common.PollCommentMediaItem{DataID: "d1", TaskID: "t1", MediaType: types.MediaTypeAudio}
	req := common.PollCommentMediaReq{CommentID: 42, Items: []common.PollCommentMediaItem{item}}

	env.OnActivity(activity.PollCommentMediaModerationResultActivity, mock.Anything, item).
		Return(common.MediaModerationStatusReject, nil)
	env.OnActivity(activity.FinalizeCommentMediaModerationActivity, mock.Anything, int64(42)).
		Return(errors.New("temporary database error")).Once()
	env.OnActivity(activity.FinalizeCommentMediaModerationActivity, mock.Anything, int64(42)).
		Return(nil).Once()

	env.ExecuteWorkflow(common.PollCommentMediaWorkflowName, req, opts)
	require.NoError(t, env.GetWorkflowError())
}

func TestPollCommentMediaWorkflow_FinalizeFailureDeletesComment(t *testing.T) {
	env, opts := newPollTestEnv(t)
	item := common.PollCommentMediaItem{DataID: "d1", TaskID: "t1", MediaType: types.MediaTypeAudio}
	req := common.PollCommentMediaReq{CommentID: 42, Items: []common.PollCommentMediaItem{item}}

	env.OnActivity(activity.PollCommentMediaModerationResultActivity, mock.Anything, item).
		Return(common.MediaModerationStatusReject, nil)
	env.OnActivity(activity.FinalizeCommentMediaModerationActivity, mock.Anything, int64(42)).
		Return(errors.New("database read failed")).Times(3)
	env.OnActivity(activity.DeleteCommentAfterFinalizeFailureActivity, mock.Anything, int64(42)).
		Return(nil).Once()

	env.ExecuteWorkflow(common.PollCommentMediaWorkflowName, req, opts)
	require.NoError(t, env.GetWorkflowError())
}

func TestPollCommentMediaWorkflow_PollFailureFinalizesComment(t *testing.T) {
	env, opts := newPollTestEnv(t)
	item := common.PollCommentMediaItem{DataID: "d1", TaskID: "t1", MediaType: types.MediaTypeAudio}
	req := common.PollCommentMediaReq{CommentID: 42, Items: []common.PollCommentMediaItem{item}}

	env.OnActivity(activity.PollCommentMediaModerationResultActivity, mock.Anything, item).
		Return("", errors.New("provider unavailable"))
	env.OnActivity(activity.FinalizeCommentMediaModerationActivity, mock.Anything, int64(42)).
		Return(nil).Once()

	env.ExecuteWorkflow(common.PollCommentMediaWorkflowName, req, opts)
	require.NoError(t, env.GetWorkflowError())
}

func TestPollCommentMediaWorkflow_NoItemsIsNoop(t *testing.T) {
	env, opts := newPollTestEnv(t)
	req := common.PollCommentMediaReq{CommentID: 42, Items: nil}
	env.ExecuteWorkflow(common.PollCommentMediaWorkflowName, req, opts)
	require.NoError(t, env.GetWorkflowError())
}

func TestPollCommentMediaWorkflow_TimeoutDeletesComment(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()
	env.RegisterActivity(activity.PollCommentMediaModerationResultActivity)
	env.RegisterActivity(activity.FinalizeCommentMediaModerationActivity)
	opts := common.PollCommentMediaOptions{PollInterval: time.Second, WorkflowTimeout: 2 * time.Second}
	env.RegisterWorkflowWithOptions(PollCommentMediaModerationWorkflow, workflow.RegisterOptions{
		Name: common.PollCommentMediaWorkflowName,
	})

	item := common.PollCommentMediaItem{DataID: "d1", TaskID: "t1", MediaType: types.MediaTypeAudio}
	req := common.PollCommentMediaReq{CommentID: 99, Items: []common.PollCommentMediaItem{item}}

	// Poll always returns pending (never resolves).
	env.OnActivity(activity.PollCommentMediaModerationResultActivity, mock.Anything, mock.Anything).
		Return(common.MediaModerationStatusPending, nil)
	// Finalize must run and delete the comment (pending at finalize → delete).
	var finalizedAt time.Time
	env.OnActivity(activity.FinalizeCommentMediaModerationActivity, mock.Anything, mock.Anything).
		Return(nil).Run(func(mock.Arguments) { finalizedAt = env.Now() })

	startedAt := env.Now()
	// Safety net for the old implementation, whose configured timeout is unused.
	// A correct workflow finalizes at 2s, before this cancellation at 3s.
	env.RegisterDelayedCallback(func() { env.CancelWorkflow() }, 3*time.Second)

	env.ExecuteWorkflow(common.PollCommentMediaWorkflowName, req, opts)
	require.False(t, finalizedAt.IsZero(), "finalize activity must run at the business deadline")
	require.LessOrEqual(t, finalizedAt.Sub(startedAt), 2*time.Second,
		"finalize must run before Temporal's outer execution timeout")
}
