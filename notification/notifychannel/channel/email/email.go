package email

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"go.temporal.io/api/enums/v1"
	"go.temporal.io/sdk/client"
	sdktemporal "go.temporal.io/sdk/temporal"
	"opencsg.com/csghub-server/builder/temporal"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	notifychannel "opencsg.com/csghub-server/notification/notifychannel"
	emailclient "opencsg.com/csghub-server/notification/notifychannel/channel/email/client"
	emailworkflow "opencsg.com/csghub-server/notification/notifychannel/channel/email/workflow"
	"opencsg.com/csghub-server/notification/utils"
)

type EmailChannel struct {
	config        *config.Config
	emailService  emailclient.EmailService
	subjectPrefix string
}

const (
	individualEmailMaxAttempts      = 2
	defaultDirectEmailMaxRecipients = 5
)

func NewChannel(conf *config.Config, emailService emailclient.EmailService) notifychannel.Notifier {
	return &EmailChannel{
		config:        conf,
		emailService:  emailService,
		subjectPrefix: emailSubjectPrefix(conf.Frontend.URL),
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
	if s.subjectPrefix != "" {
		emailReq.Subject = s.subjectPrefix + emailReq.Subject
	}

	if emailReq.ContentType == "" {
		emailReq.ContentType = types.ContentTypeTextHTML
	}

	if emailReq.Source == "" {
		emailReq.Source = types.EmailSourceUser
	}

	if req.Receiver.IsBroadcast {
		return s.broadcastEmail(ctx, emailReq, req.MsgUUID)
	}

	emailReq.To = req.Receiver.GetUserEmails()
	if len(emailReq.To) > s.directEmailMaxRecipients() {
		return s.broadcastEmail(ctx, emailReq, req.MsgUUID)
	}
	return s.sendEmailToUsers(emailReq)
}

func (s *EmailChannel) directEmailMaxRecipients() int {
	if s.config.Notification.DirectEmailMaxRecipients <= 0 {
		return defaultDirectEmailMaxRecipients
	}
	return s.config.Notification.DirectEmailMaxRecipients
}

func (s *EmailChannel) sendEmailToUsers(msg types.EmailReq) error {
	slog.Info("sendEmailToUsers: starting", "totalEmails", len(msg.To))
	if len(msg.To) == 0 {
		slog.Error("no emails to send", "subject", msg.Subject)
		return nil
	}

	var failedEmails []string
	var successCount int

	for i, email := range msg.To {
		individualReq := msg
		individualReq.To = []string{email}

		var sendErr error
		for attempt := 1; attempt <= individualEmailMaxAttempts; attempt++ {
			slog.Debug("sendEmailToUsers: sending to individual email", "index", i, "totalEmails", len(msg.To), "attempt", attempt)
			sendErr = s.emailService.Send(individualReq)
			if sendErr == nil {
				break
			}
			slog.Warn("failed to send email to user", "error", sendErr, "index", i, "totalEmails", len(msg.To), "attempt", attempt)
		}
		if sendErr != nil {
			failedEmails = append(failedEmails, email)
			continue
		}
		successCount++
		slog.Debug("send email to user successfully", "index", i, "totalEmails", len(msg.To))
	}

	if len(failedEmails) > 0 {
		slog.Warn("some emails failed to send", "failedCount", len(failedEmails), "successCount", successCount)
		if successCount == 0 {
			return utils.NewErrSendMsg(errors.New("failed to send email"), fmt.Sprintf("failed email count: %d", len(failedEmails)))
		}
		return nil
	}

	slog.Info("send email to users successfully", "successCount", successCount, "failedCount", len(failedEmails))
	return nil
}

func (s *EmailChannel) broadcastEmail(ctx context.Context, msg types.EmailReq, msgUUID string) error {
	slog.InfoContext(ctx, "start email workflow", "subject", msg.Subject, "explicit_recipients", len(msg.To))

	workflowOptions := client.StartWorkflowOptions{
		TaskQueue: emailworkflow.WorkflowBroadcastEmailQueueName,
	}
	if msgUUID != "" {
		workflowOptions.ID = notificationEmailWorkflowID(msgUUID)
		workflowOptions.WorkflowIDReusePolicy = enums.WORKFLOW_ID_REUSE_POLICY_ALLOW_DUPLICATE_FAILED_ONLY
	}

	workflowClient := temporal.GetClient()
	we, err := workflowClient.ExecuteWorkflow(ctx, workflowOptions, emailworkflow.BroadcastEmailWorkflow, msg, s.config.Notification.BroadcastEmailPageSize)
	if sdktemporal.IsWorkflowExecutionAlreadyStartedError(err) {
		slog.InfoContext(ctx, "email workflow already started", "workflow_id", workflowOptions.ID)
		return nil
	}
	if err != nil {
		return utils.NewErrSendMsg(err, "failed to start broadcast email workflow")
	}
	slog.InfoContext(ctx, "email workflow started", slog.Any("workflow id", we.GetID()))
	return nil
}

func notificationEmailWorkflowID(msgUUID string) string {
	return "notification-email-" + msgUUID
}
