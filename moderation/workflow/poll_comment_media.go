package workflow

import (
	"context"
	"fmt"
	"time"

	enums "go.temporal.io/api/enums/v1"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
	csghubTemporal "opencsg.com/csghub-server/builder/temporal"
	"opencsg.com/csghub-server/moderation/workflow/activity"
	"opencsg.com/csghub-server/moderation/workflow/common"
)

const (
	defaultPollInterval     = 30 * time.Second
	defaultWorkflowTimeout  = 30 * time.Minute
	finalizeActivityTimeout = 10 * time.Second
	finalizeScheduleTimeout = 45 * time.Second
	// Covers either finalize+fallback-delete or finalize+notification, with
	// scheduling margin before the outer workflow execution timeout fires.
	workflowCleanupGrace = 2 * time.Minute
)

// pollCommentMediaWorkflow holds the poll timing used by the workflow.
type pollCommentMediaWorkflow struct {
	pollInterval    time.Duration
	workflowTimeout time.Duration
}

func newPollCommentMediaWorkflow(opts common.PollCommentMediaOptions) *pollCommentMediaWorkflow {
	pollInterval := defaultPollInterval
	if opts.PollInterval > 0 {
		pollInterval = opts.PollInterval
	}
	workflowTimeout := defaultWorkflowTimeout
	if opts.WorkflowTimeout > 0 {
		workflowTimeout = opts.WorkflowTimeout
	}
	return &pollCommentMediaWorkflow{pollInterval: pollInterval, workflowTimeout: workflowTimeout}
}

// PollCommentMediaModerationWorkflow polls every provider task attached to a
// comment until all are terminal or the workflow timeout fires. On exit it
// finalizes the comment: keep it if every media row passed, otherwise delete
// it. The workflow is the single source of truth for a comment's fate once it
// has been created with pending media.
func PollCommentMediaModerationWorkflow(ctx workflow.Context, req common.PollCommentMediaReq, opts common.PollCommentMediaOptions) error {
	wf := newPollCommentMediaWorkflow(opts)
	return wf.Execute(ctx, req)
}

func init() {
	common.PollCommentMediaModerationWorkflow = PollCommentMediaModerationWorkflow
}

func (w *pollCommentMediaWorkflow) Execute(ctx workflow.Context, req common.PollCommentMediaReq) error {
	if len(req.Items) == 0 {
		return nil
	}
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		// Generous relative to the poll interval so the activity never times
		// out on its own; the workflow's own deadline (set in StartWorkflowOptions)
		// is the real backstop.
		StartToCloseTimeout: w.pollInterval * 10,
		// Retry transient provider/RPC failures so a single blip does not
		// delete the comment. Non-retryable terminal results are returned, not
		// errored, so they do not trigger this retry policy.
		RetryPolicy: &temporal.RetryPolicy{MaximumAttempts: 5},
	})

	// pending[i] == true means item i still needs polling.
	pending := make([]bool, len(req.Items))
	for i := range req.Items {
		pending[i] = true
	}
	deadline := workflow.Now(ctx).Add(w.workflowTimeout)

	for {
		// If the workflow context was canceled (run timeout), stop polling
		// and finalize so the comment is deleted on timeout. Checking before
		// scheduling the next activity avoids the activity retry policy
		// running past the deadline and failing the workflow before finalize.
		if ctx.Err() != nil {
			goto finalize
		}
		if !workflow.Now(ctx).Before(deadline) {
			goto finalize
		}
		anyPending := false
		for i, item := range req.Items {
			if !pending[i] {
				continue
			}
			anyPending = true
			status, deadlineReached, err := w.pollUntilDeadline(ctx, item, deadline)
			if deadlineReached {
				goto finalize
			}
			if err != nil {
				// A canceled context (workflow timeout) is expected: break out
				// of the poll loop and finalize with whatever state we have so
				// the comment is deleted on timeout. Other errors abort the
				// workflow. Other errors have already exhausted the activity retry
				// policy, so fail closed through finalize instead of leaving the
				// comment pending forever.
				if temporal.IsCanceledError(err) || ctx.Err() != nil {
					goto finalize
				}
				goto finalize
			}
			if status == common.MediaModerationStatusPass {
				pending[i] = false
				continue
			}
			if status == common.MediaModerationStatusPending {
				continue
			}
			// A terminal reject/error result: stop polling and finalize now.
			pending[i] = false
			return w.finalize(ctx, req)
		}
		if !anyPending {
			break
		}
		// Wait for the poll interval before the next pass. ctx.Done() fires
		// when the workflow timeout elapses, exiting the loop.
		sleepDuration := w.pollInterval
		if remaining := deadline.Sub(workflow.Now(ctx)); remaining < sleepDuration {
			sleepDuration = remaining
		}
		if sleepDuration <= 0 {
			break
		}
		if err := workflow.Sleep(ctx, sleepDuration); err != nil {
			// ctx canceled (timeout) → finalize with whatever state we have.
			break
		}
	}
finalize:
	// Use a disconnected context for finalize so the final activity can still
	// run after the workflow timeout canceled the main ctx. Without this, the
	// finalize activity would fail on the canceled context and the comment
	// would never be deleted (stuck pending, author-only visible forever).
	finalizeCtx, cancel := workflow.NewDisconnectedContext(ctx)
	defer cancel()
	return w.finalize(finalizeCtx, req)
}

// pollUntilDeadline races one poll activity against the workflow's business
// deadline. The outer WorkflowExecutionTimeout cannot be used for this: once
// it fires, Temporal terminates the execution and no finalize activity can be
// scheduled.
func (w *pollCommentMediaWorkflow) pollUntilDeadline(
	ctx workflow.Context,
	item common.PollCommentMediaItem,
	deadline time.Time,
) (string, bool, error) {
	remaining := deadline.Sub(workflow.Now(ctx))
	if remaining <= 0 {
		return "", true, nil
	}

	activityCtx, cancelActivity := workflow.WithCancel(ctx)
	timerCtx, cancelTimer := workflow.WithCancel(ctx)
	activityFuture := workflow.ExecuteActivity(activityCtx, common.PollCommentMediaResultActivity, item)
	deadlineFuture := workflow.NewTimer(timerCtx, remaining)

	var status string
	var pollErr error
	deadlineReached := false
	selector := workflow.NewSelector(ctx)
	selector.AddFuture(activityFuture, func(f workflow.Future) {
		pollErr = f.Get(ctx, &status)
		cancelTimer()
	})
	selector.AddFuture(deadlineFuture, func(workflow.Future) {
		deadlineReached = true
		cancelActivity()
	})
	selector.Select(ctx)

	return status, deadlineReached, pollErr
}

func (w *pollCommentMediaWorkflow) finalize(ctx workflow.Context, req common.PollCommentMediaReq) error {
	// Finalize gets a bounded retry window inside workflowCleanupGrace so a
	// transient database error cannot leave the comment pending forever.
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout:    finalizeActivityTimeout,
		ScheduleToCloseTimeout: finalizeScheduleTimeout,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    time.Second,
			BackoffCoefficient: 2,
			MaximumInterval:    5 * time.Second,
			MaximumAttempts:    3,
		},
	})
	if err := workflow.ExecuteActivity(ctx, common.FinalizeCommentMediaActivity, req.CommentID).Get(ctx, nil); err != nil {
		if deleteErr := workflow.ExecuteActivity(ctx, activity.DeleteCommentAfterFinalizeFailureActivity, req.CommentID).Get(ctx, nil); deleteErr != nil {
			return fmt.Errorf("finalize comment media moderation for comment %d: %v; delete comment: %w", req.CommentID, err, deleteErr)
		}
		return nil
	}
	if req.Notification != nil {
		if err := workflow.ExecuteActivity(ctx, common.SendApprovedCommentNotificationActivity, *req.Notification).Get(ctx, nil); err != nil {
			return fmt.Errorf("notify approved comment %d: %w", req.CommentID, err)
		}
	}
	return nil
}

// StartCommentMediaModerationWorkflow starts a poll workflow for one comment.
// The workflow ID is derived from the comment id so a duplicate create retry
// reuses the existing run instead of starting a second one.
func StartCommentMediaModerationWorkflow(ctx context.Context, temporalClient csghubTemporal.Client, req common.PollCommentMediaReq, timing common.PollCommentMediaOptions) error {
	if temporalClient == nil {
		return fmt.Errorf("temporal client is not configured")
	}
	if len(req.Items) == 0 {
		return nil
	}
	timeout := defaultWorkflowTimeout
	if timing.WorkflowTimeout > 0 {
		timeout = timing.WorkflowTimeout
	}
	opts := client.StartWorkflowOptions{
		ID:                    fmt.Sprintf("comment-media-moderation-%d", req.CommentID),
		TaskQueue:             common.PollCommentMediaQueue,
		WorkflowIDReusePolicy: enums.WORKFLOW_ID_REUSE_POLICY_TERMINATE_IF_RUNNING,
		// The workflow owns the business deadline and must remain alive long
		// enough to run its disconnected finalize activity afterwards.
		WorkflowExecutionTimeout: timeout + workflowCleanupGrace,
	}
	_, err := temporalClient.ExecuteWorkflow(ctx, opts, common.PollCommentMediaWorkflowName, req, timing)
	if err != nil {
		return fmt.Errorf("start comment media moderation workflow for comment %d: %w", req.CommentID, err)
	}
	return nil
}
