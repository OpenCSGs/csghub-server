package common

import (
	"time"

	"go.temporal.io/sdk/workflow"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
)

const (
	RepoFullCheckQueue        = "moderation_repo_full_check_queue"
	RepoFullCheckWorkflowName = "RepoFullCheckWorkflow"
)

// Queue, workflow, and activity names for the comment media moderation poll
// workflow. It is registered on a dedicated queue so it does not block repo
// full-check activities.
const (
	PollCommentMediaQueue                   = "moderation_poll_comment_media_queue"
	PollCommentMediaWorkflowName            = "PollCommentMediaModerationWorkflow"
	PollCommentMediaResultActivity          = "PollCommentMediaModerationResultActivity"
	FinalizeCommentMediaActivity            = "FinalizeCommentMediaModerationActivity"
	SendApprovedCommentNotificationActivity = "SendApprovedCommentNotificationActivity"
)

// Media moderation status values returned by the poll activity. They mirror
// the media_moderations status column so the workflow does not import the
// database package (which would create an import cycle through the activity
// layer).
const (
	MediaModerationStatusPending = "pending"
	MediaModerationStatusPass    = "pass"
	MediaModerationStatusReject  = "reject"
	MediaModerationStatusError   = "error"
)

type Repo struct {
	Namespace string
	Name      string
	RepoType  types.RepositoryType
	Branch    string
}

// PollCommentMediaItem describes one provider task whose result must be polled
// before the comment can be made public.
type PollCommentMediaItem struct {
	DataID    string          `json:"data_id"`
	TaskID    string          `json:"task_id"`
	MediaType types.MediaType `json:"media_type"`
}

// PollCommentMediaReq is the input to PollCommentMediaModerationWorkflow.
type PollCommentMediaReq struct {
	CommentID    int64                      `json:"comment_id"`
	Items        []PollCommentMediaItem     `json:"items"`
	Notification *types.CommentNotification `json:"notification,omitempty"`
}

// PollCommentMediaOptions contains only the non-sensitive timing values that
// are safe to persist in Temporal workflow history.
type PollCommentMediaOptions struct {
	PollInterval    time.Duration `json:"poll_interval"`
	WorkflowTimeout time.Duration `json:"workflow_timeout"`
}

// RepoFullCheckWorkflowFn is a type definition for the workflow function to allow IDE navigation
// while avoiding import cycles.
type RepoFullCheckWorkflowFn func(ctx workflow.Context, repo Repo, cfg *config.Config) error

// PollCommentMediaWorkflowFn is the type definition for the poll-comment-media
// workflow function, kept here to avoid import cycles between the workflow and
// activity packages.
type PollCommentMediaWorkflowFn func(ctx workflow.Context, req PollCommentMediaReq, opts PollCommentMediaOptions) error

// Pointers to the actual workflow functions to allow IDE navigation
var (
	RepoFullCheckWorkflow              RepoFullCheckWorkflowFn
	PollCommentMediaModerationWorkflow PollCommentMediaWorkflowFn
)
