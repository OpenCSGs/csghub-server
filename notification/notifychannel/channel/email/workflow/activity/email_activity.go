package activity

import (
	"context"
	"fmt"

	"go.temporal.io/sdk/activity"
	"opencsg.com/csghub-server/builder/rpc"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
	emailclient "opencsg.com/csghub-server/notification/notifychannel/channel/email/client"
)

type BatchSendEmail struct {
	Page     int
	PageSize int
	EmailReq types.EmailReq
}

type BatchGetEmailResult struct {
	MorePages bool
	Emails    []string
}

type BroadcastEmailActivity struct {
	storage       database.NotificationStore
	userSvcClient rpc.UserSvcClient
	emailService  emailclient.EmailService
}

func NewBroadcastEmailActivity(storage database.NotificationStore, userSvcClient rpc.UserSvcClient, emailService emailclient.EmailService) *BroadcastEmailActivity {
	return &BroadcastEmailActivity{
		storage:       storage,
		userSvcClient: userSvcClient,
		emailService:  emailService,
	}
}

func (a *BroadcastEmailActivity) GetEmailFromNotificationSettingActivity(ctx context.Context, input BatchSendEmail) (BatchGetEmailResult, error) {
	logger := activity.GetLogger(ctx)
	logger.Debug("get email from notification setting started", "input", input)

	emails, total, err := a.storage.GetEmails(ctx, input.PageSize, input.Page)
	if err != nil {
		return BatchGetEmailResult{}, err
	}

	return BatchGetEmailResult{
		MorePages: total > input.PageSize*input.Page,
		Emails:    emails,
	}, nil
}

func (a *BroadcastEmailActivity) GetEmailFromUserActivity(ctx context.Context, input BatchSendEmail) (BatchGetEmailResult, error) {
	logger := activity.GetLogger(ctx)
	logger.Debug("get email from user started", "input", input)

	emails, total, err := a.userSvcClient.GetEmails(ctx, input.PageSize, input.Page)
	if err != nil {
		return BatchGetEmailResult{}, err
	}

	return BatchGetEmailResult{
		MorePages: total > input.PageSize*input.Page,
		Emails:    emails,
	}, nil
}

func (a *BroadcastEmailActivity) SendEmailBatchActivity(ctx context.Context, emailReq types.EmailReq, emails []string) error {
	logger := activity.GetLogger(ctx)
	logger.Info("email batch started", "subject", emailReq.Subject, "from", emailReq.From, "emailCount", len(emails))

	if len(emails) == 0 {
		logger.Info("no emails to send")
		return nil
	}

	var failedEmails []string
	var successCount int

	for i, email := range emails {
		individualReq := emailReq
		individualReq.To = []string{email}

		err := a.emailService.Send(individualReq)
		if err != nil {
			logger.Error("failed to send individual email", "email", email, "error", err, "index", i, "totalEmails", len(emails))
			failedEmails = append(failedEmails, email)
			continue
		}

		successCount++
		logger.Info("individual email sent successfully", "email", email, "index", i, "subject", individualReq.Subject)
	}

	logger.Info("email batch completed", "subject", emailReq.Subject, "from", emailReq.From, "totalEmails", len(emails), "successCount", successCount, "failedCount", len(failedEmails), "failedEmails", failedEmails)

	if successCount == 0 && len(failedEmails) > 0 {
		return fmt.Errorf("all emails failed to send (%d total): %v", len(failedEmails), failedEmails)
	}

	if len(failedEmails) > 0 {
		logger.Warn("some emails failed to send", "subject", emailReq.Subject, "from", emailReq.From, "failedCount", len(failedEmails), "successCount", successCount, "failedEmails", failedEmails)
	}

	return nil
}
