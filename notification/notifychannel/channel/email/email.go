package email

import (
	"context"
	"fmt"
	"log/slog"

	"go.temporal.io/sdk/client"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	notifychannel "opencsg.com/csghub-server/notification/notifychannel"
	emailclient "opencsg.com/csghub-server/notification/notifychannel/channel/email/client"
	emailworkflow "opencsg.com/csghub-server/notification/notifychannel/channel/email/workflow"
	"opencsg.com/csghub-server/notification/workflow"
)

type EmailChannel struct {
	config       *config.Config
	emailService emailclient.EmailService
}

func NewChannel(conf *config.Config, emailService emailclient.EmailService) notifychannel.Notifier {
	return &EmailChannel{
		config:       conf,
		emailService: emailService,
	}
}

var _ notifychannel.Notifier = (*EmailChannel)(nil)

func (s *EmailChannel) IsFormatRequired() bool {
	return true
}

func (s *EmailChannel) Send(ctx context.Context, req *notifychannel.NotifyRequest) error {
	if err := req.Receiver.Validate(); err != nil {
		return fmt.Errorf("invalid receiver: %w", err)
	}

	var emailReq types.EmailReq
	if content, ok := req.Receiver.GetMetadata("content").(types.NotificationEmailContent); ok {
		emailReq.Subject = content.Subject
		emailReq.Attachments = content.Attachments
		emailReq.Source = content.Source
		emailReq.ContentType = content.ContentType
	}
	emailReq.Body = req.Payload
	if emailReq.ContentType == "" {
		emailReq.ContentType = types.ContentTypeTextHTML
	}
	if emailReq.Subject == "" {
		emailReq.Subject = "Notification"
	}

	if req.Receiver.IsBroadcast {
		return s.broadcastEmail(ctx, emailReq)
	} else {
		emailReq.To = req.Receiver.GetUserEmails()
		return s.sendEmailToUsers(emailReq)
	}
}

func (s *EmailChannel) sendEmailToUsers(msg types.EmailReq) error {
	toList := msg.To
	for _, to := range toList {
		msg.To = []string{to}
		if err := s.emailService.Send(msg); err != nil {
			slog.Error("failed to send email to user", "error", err)
			continue
		}
	}
	return nil
}

func (s *EmailChannel) broadcastEmail(ctx context.Context, msg types.EmailReq) error {
	slog.Info("broadcast email to all users", "subject", msg.Subject)

	workflowClient := workflow.GetWorkflowClient()
	if workflowClient == nil {
		return fmt.Errorf("workflow client is nil")
	}
	workflowOptions := client.StartWorkflowOptions{
		TaskQueue: emailworkflow.WorkflowBroadcastEmailQueueName,
	}
	we, err := workflowClient.ExecuteWorkflow(ctx, workflowOptions, emailworkflow.BroadcastEmailWorkflow, msg, s.config.Notification.BroadcastEmailPageSize)
	if err != nil {
		return fmt.Errorf("failed to start broadcast email workflow: %w", err)
	}
	slog.Info("start broadcast email workflow", slog.Any("workflow id", we.GetID()))
	return nil
}
