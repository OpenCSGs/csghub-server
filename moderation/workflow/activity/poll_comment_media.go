package activity

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"opencsg.com/csghub-server/builder/rpc"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/component"
	"opencsg.com/csghub-server/moderation/workflow/common"
)

// Package-level factories so tests can substitute mock stores/RPC clients
// without a live moderation service or database. Production code uses the
// defaults, which build real stores and RPC clients from config.
var (
	runtimeConfig           *config.Config
	newMediaModerationStore = func() database.MediaModerationStore {
		return database.NewMediaModerationStore()
	}
	newModerationRPCClient = func(cfg *config.Config) rpc.ModerationSvcClient {
		return rpc.NewModerationSvcHttpClient(fmt.Sprintf("%s:%d", cfg.Moderation.Host, cfg.Moderation.Port))
	}
	newDiscussionStore = func() database.DiscussionStore {
		return database.NewDiscussionStore()
	}
	newNotificationRPCClient = func(cfg *config.Config) rpc.NotificationSvcClient {
		return rpc.NewNotificationSvcHttpClient(
			fmt.Sprintf("%s:%d", cfg.Notification.Host, cfg.Notification.Port),
			rpc.AuthWithApiKey(cfg.APIToken),
		)
	}
)

// ConfigureCommentMediaActivities installs worker-local configuration used to
// construct activity dependencies. It must be called when the moderation
// worker starts; the config is never serialized into Temporal payloads.
func ConfigureCommentMediaActivities(cfg *config.Config) {
	runtimeConfig = cfg
}

func commentMediaRuntimeConfig() (*config.Config, error) {
	if runtimeConfig == nil {
		return nil, fmt.Errorf("comment media activity config is not initialized")
	}
	return runtimeConfig, nil
}

// SendApprovedCommentNotificationActivity sends a notification only after
// finalize confirms every linked media item passed. MsgUUID remains stable
// across Temporal retries so downstream delivery can deduplicate it.
func SendApprovedCommentNotificationActivity(ctx context.Context, notification types.CommentNotification) error {
	cfg, err := commentMediaRuntimeConfig()
	if err != nil {
		return err
	}
	views, err := newMediaModerationStore().FindMediaByCommentIDs(ctx, []int64{notification.CommentID})
	if err != nil {
		return fmt.Errorf("load media moderation rows before notification: %w", err)
	}
	if len(views) == 0 {
		return nil
	}
	for _, view := range views {
		if view.Status != database.MediaModerationStatusPass {
			return nil
		}
	}
	message := types.NotificationMessage{
		MsgUUID: notification.MsgUUID, UserUUIDs: []string{notification.RecipientUUID},
		SenderUUID: notification.SenderUUID, NotificationType: types.NotificationComment,
		CreateAt:       notification.CreatedAt,
		ClickActionURL: fmt.Sprintf("%s/community", component.GetRepoUrl(notification.RepoType, notification.RepoPath)),
		Template:       string(types.MessageScenarioDiscussion),
		Payload:        map[string]any{"repo_type": notification.RepoType},
	}
	parameters, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("marshal approved comment notification: %w", err)
	}
	return newNotificationRPCClient(cfg).Send(ctx, &types.MessageRequest{
		Scenario: types.MessageScenarioDiscussion, Parameters: string(parameters), Priority: types.MessagePriorityHigh,
	})
}

// PollCommentMediaModerationResultActivity queries one provider task and, when
// the result is terminal, persists it on the media_moderations row. It returns
// the terminal status ("pass"/"reject"/"error") or "pending" while the task is
// still being processed. Transport errors are returned so the workflow can
// retry the activity.
func PollCommentMediaModerationResultActivity(ctx context.Context, item common.PollCommentMediaItem) (string, error) {
	logger := slog.With("activity", "poll_comment_media")
	if item.DataID == "" || item.TaskID == "" {
		return "", fmt.Errorf("poll media moderation: data id and task id are required")
	}
	cfg, err := commentMediaRuntimeConfig()
	if err != nil {
		return "", err
	}
	client := newModerationRPCClient(cfg)
	store := newMediaModerationStore()

	result, err := client.QueryMediaModerationResult(ctx, types.MediaModerationRequest{
		Type: item.MediaType, DataID: item.DataID, TaskID: item.TaskID,
	})
	if err != nil {
		return "", fmt.Errorf("query media moderation result for task %q: %w", item.TaskID, err)
	}
	if result == nil {
		return common.MediaModerationStatusPending, nil
	}
	if result.Status == common.MediaModerationStatusPending {
		return common.MediaModerationStatusPending, nil
	}
	if result.Status != common.MediaModerationStatusPass &&
		result.Status != common.MediaModerationStatusReject &&
		result.Status != common.MediaModerationStatusError {
		unknownStatus := result.Status
		logger.Warn("poll media moderation returned an unknown status, treating as error",
			slog.String("data_id", item.DataID), slog.String("status", unknownStatus))
		result.Status = common.MediaModerationStatusError
		result.Reason = fmt.Sprintf("unknown media moderation status %q", unknownStatus)
	}
	if err := store.UpdateResult(ctx, item.DataID, item.TaskID, result.Status, result.Reason); err != nil {
		return "", fmt.Errorf("update media moderation result for data id %q: %w", item.DataID, err)
	}
	return result.Status, nil
}

// FinalizeCommentMediaModerationActivity loads every media_moderations row
// linked to the comment through comment_media and decides its fate: if all
// linked rows are terminal and every one is "pass", the comment stays (it is
// already public by the visibility rule). If any linked row is "reject" or
// "error", or any row is still non-terminal (timeout), the comment is
// hard-deleted. Scoping by the comment_media link means a shared media
// resource's status only decides the fate of comments that reference it.
func FinalizeCommentMediaModerationActivity(ctx context.Context, commentID int64) error {
	logger := slog.With("activity", "poll_comment_media")
	mediaStore := newMediaModerationStore()
	discussionStore := newDiscussionStore()

	views, err := mediaStore.FindMediaByCommentIDs(ctx, []int64{commentID})
	if err != nil {
		return fmt.Errorf("load media moderation rows for comment %d: %w", commentID, err)
	}
	deleteComment := false
	for _, view := range views {
		switch view.Status {
		case database.MediaModerationStatusPass:
			// terminal and approved
		case database.MediaModerationStatusReject, database.MediaModerationStatusError:
			deleteComment = true
		default:
			// pending or unknown at finalize time means the timeout fired
			// before the provider returned a result; fail closed by deleting.
			deleteComment = true
		}
	}
	if !deleteComment {
		logger.Info("comment media moderation passed, keeping comment", slog.Int64("comment_id", commentID))
		return nil
	}
	logger.Info("comment media moderation failed or timed out, deleting comment", slog.Int64("comment_id", commentID))
	if err := discussionStore.DeleteComment(ctx, commentID); err != nil {
		return fmt.Errorf("delete comment %d after media moderation failure: %w", commentID, err)
	}
	return nil
}

// DeleteCommentAfterFinalizeFailureActivity is the fail-closed fallback when
// finalization cannot determine the moderation result after exhausting its
// retries. It removes the comment directly so a failed workflow cannot leave
// it permanently pending and author-visible.
func DeleteCommentAfterFinalizeFailureActivity(ctx context.Context, commentID int64) error {
	if err := newDiscussionStore().DeleteComment(ctx, commentID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		return fmt.Errorf("delete comment %d after finalize failure: %w", commentID, err)
	}
	return nil
}
