package email

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"go.temporal.io/sdk/client"
	"opencsg.com/csghub-server/builder/temporal"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	notifychannel "opencsg.com/csghub-server/notification/notifychannel"
	emailclient "opencsg.com/csghub-server/notification/notifychannel/channel/email/client"
	emailworkflow "opencsg.com/csghub-server/notification/notifychannel/channel/email/workflow"
	"opencsg.com/csghub-server/notification/utils"
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
	if req.Message != nil {
		if extractedEmailReq, ok := req.Message.(types.EmailReq); ok {
			emailReq = extractedEmailReq
		} else {
			slog.Warn("invalid message format, using default email settings", "message", req.Message, "type", fmt.Sprintf("%T", req.Message))
		}
	}

	if req.FormattedData != nil {
		emailReq.Subject = req.FormattedData.Title
		emailReq.Body = req.FormattedData.Content
	}

	if emailReq.ContentType == "" {
		emailReq.ContentType = types.ContentTypeTextHTML
	}

	if emailReq.Source == "" {
		emailReq.Source = types.EmailSourceUser
	}

	if req.Receiver.IsBroadcast {
		return s.broadcastEmail(ctx, emailReq)
	} else {
		emailReq.To = req.Receiver.GetUserEmails()
		return s.sendEmailToUsers(emailReq)
	}
}

func (s *EmailChannel) sendEmailToUsers(msg types.EmailReq) error {
	if len(msg.To) == 0 {
		slog.Error("no emails to send", "email", msg)
		return nil
	}

	var failedEmails []string
	var successCount int

	for i, email := range msg.To {
		individualReq := msg
		individualReq.To = []string{email}
		if err := s.emailService.Send(individualReq); err != nil {
			slog.Error("failed to send email to user", "error", err, "index", i, "totalEmails", len(msg.To))
			failedEmails = append(failedEmails, email)
			continue
		}
		successCount++
		slog.Debug("send email to user successfully", "email", email, "index", i, "totalEmails", len(msg.To))
	}

	if successCount == 0 && len(failedEmails) > 0 {
		return utils.NewErrSendMsg(errors.New("failed to send email"), fmt.Sprintf("failed emails: %v", failedEmails))
	}

	if len(failedEmails) > 0 {
		slog.Warn("some emails failed to send", "failedCount", len(failedEmails), "successCount", successCount, "failedEmails", failedEmails)
	}

	slog.Info("send email to users successfully", "successCount", successCount, "failedEmails", failedEmails)
	return nil
}

func (s *EmailChannel) broadcastEmail(ctx context.Context, msg types.EmailReq) error {
	slog.Info("broadcast email to all users", "subject", msg.Subject)

	workflowClient := temporal.GetClient()
	workflowOptions := client.StartWorkflowOptions{
		TaskQueue: emailworkflow.WorkflowBroadcastEmailQueueName,
	}
	we, err := workflowClient.ExecuteWorkflow(ctx, workflowOptions, emailworkflow.BroadcastEmailWorkflow, msg, s.config.Notification.BroadcastEmailPageSize)
	if err != nil {
		return utils.NewErrSendMsg(err, "failed to start broadcast email workflow")
	}
	slog.Info("start broadcast email workflow", slog.Any("workflow id", we.GetID()))
	return nil
}
