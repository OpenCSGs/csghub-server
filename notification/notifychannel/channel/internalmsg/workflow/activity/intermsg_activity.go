package activity

import (
	"context"
	"fmt"

	"go.temporal.io/sdk/activity"
	"opencsg.com/csghub-server/builder/rpc"
	"opencsg.com/csghub-server/builder/store/database"
)

type BatchInsertUserMessage struct {
	Page      int
	PageSize  int
	MessageId string
}

type FailedUserMessage struct {
	MsgUUID  string
	UserUUID string
	Error    string
}

type BatchInsertUserMessageResult struct {
	MorePages          bool
	FailedUserMessages []FailedUserMessage
}

type BroadcastMessageActivity struct {
	storage       database.NotificationStore
	userSvcClient rpc.UserSvcClient
}

func NewBroadcastMessageActivity(storage database.NotificationStore, userSvcClient rpc.UserSvcClient) *BroadcastMessageActivity {
	return &BroadcastMessageActivity{
		storage:       storage,
		userSvcClient: userSvcClient,
	}
}

func (a *BroadcastMessageActivity) InsertUserMessageBatchActivity(ctx context.Context, input BatchInsertUserMessage) (BatchInsertUserMessageResult, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("broadcast message started", "messageId", input.MessageId, "page", input.Page, "pageSize", input.PageSize)

	userUUIDs, total, err := a.userSvcClient.GetUserUUIDs(ctx, input.PageSize, input.Page)
	if err != nil {
		return BatchInsertUserMessageResult{}, fmt.Errorf("failed to get user uuids, error: %w", err)
	}

	errorLogs, err := a.storage.CreateUserMessages(ctx, input.MessageId, userUUIDs)
	if err != nil {
		return BatchInsertUserMessageResult{}, fmt.Errorf("failed to create user messages, error: %w", err)
	}

	var morePages bool
	if input.Page*input.PageSize < total {
		morePages = true
	}

	var failedUserMessages []FailedUserMessage
	for _, errorLog := range errorLogs {
		failedUserMessages = append(failedUserMessages, FailedUserMessage{
			MsgUUID:  errorLog.MsgUUID,
			UserUUID: errorLog.UserUUID,
			Error:    errorLog.ErrorMsg,
		})
	}

	return BatchInsertUserMessageResult{
		MorePages:          morePages,
		FailedUserMessages: failedUserMessages,
	}, nil
}

func (a *BroadcastMessageActivity) LogUserMessageFailuresActivity(ctx context.Context, failedUserMessages []FailedUserMessage) error {
	logger := activity.GetLogger(ctx)
	logger.Info("log user message failures started", "failedUserMessages", failedUserMessages)

	for _, failedUserMessage := range failedUserMessages {
		err := a.storage.CreateUserMessageErrorLog(ctx, failedUserMessage.MsgUUID, failedUserMessage.UserUUID, failedUserMessage.Error)
		if err != nil {
			return fmt.Errorf("failed to create user message error log, error: %w", err)
		}
	}

	return nil
}
