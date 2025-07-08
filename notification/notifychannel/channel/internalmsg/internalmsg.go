package internalmsg

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"go.temporal.io/sdk/client"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	notifychannel "opencsg.com/csghub-server/notification/notifychannel"
	internalmsgworkflow "opencsg.com/csghub-server/notification/notifychannel/channel/internalmsg/workflow"
	"opencsg.com/csghub-server/notification/workflow"
)

type InternalMessageChannel struct {
	conf    *config.Config
	storage database.NotificationStore
}

func NewChannel(conf *config.Config, storage database.NotificationStore) notifychannel.Notifier {
	return &InternalMessageChannel{
		conf:    conf,
		storage: storage,
	}
}

var _ notifychannel.Notifier = (*InternalMessageChannel)(nil)

func (s *InternalMessageChannel) IsFormatRequired() bool {
	return false
}

func (s *InternalMessageChannel) Send(ctx context.Context, req *notifychannel.NotifyRequest) error {
	if err := req.Receiver.Validate(); err != nil {
		return fmt.Errorf("invalid receiver: %w", err)
	}

	// Try to extract message data from different possible types
	var msg types.NotificationMessage

	// First try to cast as NotificationMessage
	if notificationMsg, ok := req.MessageData.(types.NotificationMessage); ok {
		msg = notificationMsg
	} else {
		// If not a NotificationMessage, try to extract from map or other types
		// that might contain title/subject and content/message
		title, content := extractMessageFromData(req.MessageData)
		msg.Title = title
		msg.Content = content
	}

	// Validate that we have the required fields
	if msg.Title == "" && msg.Summary == "" {
		return fmt.Errorf("message must contain either title or summary")
	}

	if msg.Content == "" {
		return fmt.Errorf("message must contain content")
	}

	// Set default notification type if not provided
	if !msg.NotificationType.IsValid() {
		msg.NotificationType = types.NotificationSystem
	}

	if msg.MsgUUID == "" {
		msg.MsgUUID = uuid.New().String()
	}

	if msg.CreateAt.IsZero() {
		msg.CreateAt = time.Now()
	}

	siteInternalMessage := &database.NotificationMessage{
		MsgUUID:          msg.MsgUUID,
		GroupID:          "",
		NotificationType: msg.NotificationType.String(),
		SenderUUID:       msg.SenderUUID,
		Summary:          msg.Summary,
		Title:            msg.Title,
		Content:          msg.Content,
		ActionURL:        msg.ClickActionURL,
		Priority:         msg.Priority,
	}

	// dispatch site internal message
	if !req.Receiver.IsBroadcast {
		userUUIDs := req.Receiver.GetUserUUIDs()
		if err := s.sendInternalMessageToUsers(ctx, siteInternalMessage, userUUIDs); err != nil {
			return fmt.Errorf("failed to send site internal message to users: %w", err)
		}
		slog.Info("send site internal message to users successfully", slog.Any("message id", siteInternalMessage.MsgUUID))
	} else {
		if err := s.broadcastInternalMessage(ctx, siteInternalMessage); err != nil {
			return fmt.Errorf("failed to broadcast site internal message: %w", err)
		}
		slog.Info("broadcast site internal message to all users successfully", slog.Any("message id", siteInternalMessage.MsgUUID))
	}

	return nil
}

func (s *InternalMessageChannel) broadcastInternalMessage(ctx context.Context, message *database.NotificationMessage) error {
	slog.Info("broadcast site internal message to all users", slog.Any("message id", message.MsgUUID))

	existed, err := s.storage.IsNotificationMessageExists(ctx, message.MsgUUID)
	if err != nil {
		return fmt.Errorf("failed to check if notification message exists: %w", err)
	}
	if existed {
		slog.Info("site internal message already exists, skipped", slog.Any("message id", message.MsgUUID))
		return nil
	}

	if err := s.storage.CreateNotificationMessage(ctx, message); err != nil {
		return fmt.Errorf("failed to create site internal message: %w", err)
	}
	slog.Info("create site internal message successfully", slog.Any("message id", message.MsgUUID))

	workflowClient := workflow.GetWorkflowClient()
	if workflowClient == nil {
		return fmt.Errorf("workflow client is nil")
	}
	workflowOptions := client.StartWorkflowOptions{
		TaskQueue: internalmsgworkflow.WorkflowBroadcastInternalMessageQueueName,
	}
	we, err := workflowClient.ExecuteWorkflow(context.Background(), workflowOptions, internalmsgworkflow.BroadcastInternalMessageWorkflow, message.MsgUUID, s.conf.Notification.BroadcastUserPageSize)
	if err != nil {
		return fmt.Errorf("failed to start broadcast message workflow: %w", err)
	}
	slog.Info("start broadcast message workflow", slog.Any("workflow id", we.GetID()), slog.Any("message id", message.MsgUUID))
	return nil
}

func (s *InternalMessageChannel) sendInternalMessageToUsers(ctx context.Context, message *database.NotificationMessage, users []string) error {
	if err := s.storage.CreateNotificationMessageForUsers(ctx, message, users); err != nil {
		return fmt.Errorf("failed to send site internal message for users: %w", err)
	}
	slog.Info("send site internal message for users successfully", slog.Any("message id", message.MsgUUID))
	return nil
}

// extractMessageFromData attempts to extract title and content from various input types
func extractMessageFromData(data any) (title, content string) {
	// Try to extract from map[string]interface{}
	if dataMap, ok := data.(map[string]interface{}); ok {
		// Try different field names for title
		if titleVal, exists := dataMap["title"]; exists {
			if titleStr, ok := titleVal.(string); ok {
				title = titleStr
			}
		}
		if title == "" {
			if subject, exists := dataMap["subject"]; exists {
				if subjectStr, ok := subject.(string); ok {
					title = subjectStr
				}
			}
		}

		// Try different field names for content
		if contentVal, exists := dataMap["content"]; exists {
			if contentStr, ok := contentVal.(string); ok {
				content = contentStr
			}
		}

		if content == "" {
			if summary, exists := dataMap["summary"]; exists {
				if summaryStr, ok := summary.(string); ok {
					content = summaryStr
				}
			}
		}
	}

	return title, content
}
