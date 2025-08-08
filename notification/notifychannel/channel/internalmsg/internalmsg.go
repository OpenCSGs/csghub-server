package internalmsg

import (
	"context"
	"fmt"
	"log/slog"
	"reflect"
	"time"

	"github.com/google/uuid"
	"go.temporal.io/sdk/client"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	notifychannel "opencsg.com/csghub-server/notification/notifychannel"
	internalmsgworkflow "opencsg.com/csghub-server/notification/notifychannel/channel/internalmsg/workflow"
	"opencsg.com/csghub-server/notification/utils"
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

	var msg types.NotificationMessage
	if req.Message != nil {
		if extractedMsg, ok := req.Message.(types.NotificationMessage); ok {
			msg = extractedMsg
		} else {
			slog.Warn("invalid message format, will try to extract message", "message", req.Message, "type", fmt.Sprintf("%T", req.Message))
			title, content := extractMessageFromData(req.Message)
			msg.Title = title
			msg.Content = content
		}
	}

	if err := validateMessageContent(msg); err != nil {
		return err
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
		Template:         msg.Template,
		Payload:          msg.Payload,
		ActionURL:        msg.ClickActionURL,
		Priority:         msg.Priority,
	}

	if !req.Receiver.IsBroadcast {
		userUUIDs := req.Receiver.GetUserUUIDs()
		if err := s.sendInternalMessageToUsers(ctx, siteInternalMessage, userUUIDs); err != nil {
			return utils.NewErrSendMsg(err, fmt.Sprintf("failed to send site internal message %s to users: %v", siteInternalMessage.MsgUUID, userUUIDs))
		}
	} else {
		if err := s.broadcastInternalMessage(ctx, siteInternalMessage); err != nil {
			return utils.NewErrSendMsg(err, fmt.Sprintf("failed to broadcast site internal message %s", siteInternalMessage.MsgUUID))
		}
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
	slog.Info("start broadcast message workflow successfully", slog.Any("workflow id", we.GetID()), slog.Any("message id", message.MsgUUID))
	return nil
}

func (s *InternalMessageChannel) sendInternalMessageToUsers(ctx context.Context, message *database.NotificationMessage, users []string) error {
	if err := s.storage.CreateNotificationMessageForUsers(ctx, message, users); err != nil {
		return fmt.Errorf("failed to send site internal message for users: %w", err)
	}
	slog.Info("send site internal message for users successfully", slog.Any("message id", message.MsgUUID))
	return nil
}

func extractMessageFromData(data any) (title, content string) {
	v := reflect.ValueOf(data)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	if v.Kind() == reflect.Struct {
		t := v.Type()
		for i := 0; i < v.NumField(); i++ {
			field := v.Field(i)
			fieldName := t.Field(i).Name

			if (fieldName == "Title" || fieldName == "Subject") && title == "" {
				if field.Kind() == reflect.String {
					title = field.String()
				}
			}

			if (fieldName == "Content" || fieldName == "Summary") && content == "" {
				if field.Kind() == reflect.String {
					content = field.String()
				}
			}
		}

		if title != "" && content != "" {
			return title, content
		}
	}

	if dataMap, ok := data.(map[string]any); ok {
		if title == "" {
			if titleVal, exists := dataMap["title"]; exists {
				if titleStr, ok := titleVal.(string); ok {
					title = titleStr
				}
			}
		}
		if title == "" {
			if subject, exists := dataMap["subject"]; exists {
				if subjectStr, ok := subject.(string); ok {
					title = subjectStr
				}
			}
		}

		if content == "" {
			if contentVal, exists := dataMap["content"]; exists {
				if contentStr, ok := contentVal.(string); ok {
					content = contentStr
				}
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

func validateMessageContent(msg types.NotificationMessage) error {
	if msg.Template != "" && len(msg.Payload) > 0 {
		return nil
	}

	// When no template and payload, require direct message content
	if msg.Title == "" {
		return fmt.Errorf("message must contain title")
	}

	if msg.Content == "" {
		return fmt.Errorf("message must contain content")
	}

	return nil
}
