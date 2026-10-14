package email

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"go.temporal.io/api/enums/v1"
	"go.temporal.io/api/serviceerror"
	"go.temporal.io/sdk/client"
	"opencsg.com/csghub-server/builder/temporal"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/notification/notifychannel"
	emailworkflow "opencsg.com/csghub-server/notification/notifychannel/channel/email/workflow"
	"opencsg.com/csghub-server/notification/utils"
)

type recordingEmailService struct {
	attempts              map[string]int
	failuresBeforeSuccess map[string]int
	subjects              []string
}

func (s *recordingEmailService) Send(req types.EmailReq) error {
	s.subjects = append(s.subjects, req.Subject)
	email := req.To[0]
	s.attempts[email]++
	if s.attempts[email] <= s.failuresBeforeSuccess[email] {
		return errors.New("send failed")
	}
	return nil
}

func TestEmailChannelSendToUsers(t *testing.T) {
	t.Run("sends each recipient once on success", func(t *testing.T) {
		emailService := newRecordingEmailService(nil)
		channel := newInlineEmailChannel(emailService)

		err := channel.Send(context.Background(), newEmailNotifyRequest(
			"admin1@example.com",
			"admin2@example.com",
		))

		require.NoError(t, err)
		require.Equal(t, 1, emailService.attempts["admin1@example.com"])
		require.Equal(t, 1, emailService.attempts["admin2@example.com"])
	})

	t.Run("retries only the failed recipient", func(t *testing.T) {
		emailService := newRecordingEmailService(map[string]int{
			"admin2@example.com": 1,
		})
		channel := newInlineEmailChannel(emailService)

		err := channel.Send(context.Background(), newEmailNotifyRequest(
			"admin1@example.com",
			"admin2@example.com",
		))

		require.NoError(t, err)
		require.Equal(t, 1, emailService.attempts["admin1@example.com"])
		require.Equal(t, 2, emailService.attempts["admin2@example.com"])
	})

	t.Run("does not retry the whole channel after a partial failure", func(t *testing.T) {
		emailService := newRecordingEmailService(map[string]int{
			"admin2@example.com": individualEmailMaxAttempts,
		})
		channel := newInlineEmailChannel(emailService)

		err := channel.Send(context.Background(), newEmailNotifyRequest(
			"admin1@example.com",
			"admin2@example.com",
		))

		require.NoError(t, err)
		require.Equal(t, 1, emailService.attempts["admin1@example.com"])
		require.Equal(t, individualEmailMaxAttempts, emailService.attempts["admin2@example.com"])
	})

	t.Run("returns retryable error when every recipient fails", func(t *testing.T) {
		emailService := newRecordingEmailService(map[string]int{
			"admin1@example.com": individualEmailMaxAttempts,
			"admin2@example.com": individualEmailMaxAttempts,
		})
		channel := newInlineEmailChannel(emailService)

		err := channel.Send(context.Background(), newEmailNotifyRequest(
			"admin1@example.com",
			"admin2@example.com",
		))

		require.Error(t, err)
		require.True(t, utils.IsErrSendMsg(err))
		require.Equal(t, individualEmailMaxAttempts, emailService.attempts["admin1@example.com"])
		require.Equal(t, individualEmailMaxAttempts, emailService.attempts["admin2@example.com"])
	})
}

func TestEmailChannelSendKeepsThresholdRecipientCountInline(t *testing.T) {
	emailService := newRecordingEmailService(nil)
	channel := newThresholdEmailChannel(emailService, 5)

	err := channel.Send(context.Background(), newEmailNotifyRequest(
		"a1@example.com",
		"a2@example.com",
		"a3@example.com",
		"a4@example.com",
		"a5@example.com",
	))

	require.NoError(t, err)
	require.Equal(t, 1, emailService.attempts["a1@example.com"])
	require.Equal(t, 1, emailService.attempts["a5@example.com"])
}

func TestEmailChannelSendUsesWorkflowAboveThreshold(t *testing.T) {
	emailService := newRecordingEmailService(nil)
	workflowClient := &recordingWorkflowClient{}
	temporal.Assign(workflowClient)
	t.Cleanup(func() {
		temporal.Assign(nil)
	})

	channel := newThresholdEmailChannel(emailService, 5)
	req := newEmailNotifyRequest(
		"a1@example.com",
		"a2@example.com",
		"a3@example.com",
		"a4@example.com",
		"a5@example.com",
		"a6@example.com",
	)
	req.MsgUUID = "msg-1"

	err := channel.Send(context.Background(), req)

	require.NoError(t, err)
	require.Empty(t, emailService.attempts)
	require.Equal(t, 1, workflowClient.calls)
	require.Equal(t, emailworkflow.WorkflowBroadcastEmailQueueName, workflowClient.options.TaskQueue)
	require.Equal(t, "notification-email-msg-1", workflowClient.options.ID)
	require.Equal(t, enums.WORKFLOW_ID_REUSE_POLICY_ALLOW_DUPLICATE_FAILED_ONLY, workflowClient.options.WorkflowIDReusePolicy)
	sentReq, ok := workflowClient.args[0].(types.EmailReq)
	require.True(t, ok)
	require.Equal(t, []string{
		"a1@example.com",
		"a2@example.com",
		"a3@example.com",
		"a4@example.com",
		"a5@example.com",
		"a6@example.com",
	}, sentReq.To)
	require.Equal(t, 100, workflowClient.args[1])
}

func TestEmailChannelSendTreatsAlreadyStartedWorkflowAsSuccess(t *testing.T) {
	emailService := newRecordingEmailService(nil)
	workflowClient := &recordingWorkflowClient{
		err: &serviceerror.WorkflowExecutionAlreadyStarted{},
	}
	temporal.Assign(workflowClient)
	t.Cleanup(func() {
		temporal.Assign(nil)
	})

	channel := newThresholdEmailChannel(emailService, 5)
	req := newEmailNotifyRequest(
		"a1@example.com",
		"a2@example.com",
		"a3@example.com",
		"a4@example.com",
		"a5@example.com",
		"a6@example.com",
	)
	req.MsgUUID = "msg-1"

	err := channel.Send(context.Background(), req)

	require.NoError(t, err)
	require.Empty(t, emailService.attempts)
	require.Equal(t, 1, workflowClient.calls)
}

func TestEmailChannelSendOmitsWorkflowIDWithoutMessageUUID(t *testing.T) {
	emailService := newRecordingEmailService(nil)
	workflowClient := &recordingWorkflowClient{}
	temporal.Assign(workflowClient)
	t.Cleanup(func() {
		temporal.Assign(nil)
	})

	channel := newThresholdEmailChannel(emailService, 1)

	err := channel.Send(context.Background(), newEmailNotifyRequest("admin1@example.com", "admin2@example.com"))

	require.NoError(t, err)
	require.Empty(t, workflowClient.options.ID)
	require.Equal(t, enums.WORKFLOW_ID_REUSE_POLICY_UNSPECIFIED, workflowClient.options.WorkflowIDReusePolicy)
}

func TestEmailChannelSendTreatsNonPositiveCutoffAsDefault(t *testing.T) {
	emailService := newRecordingEmailService(nil)
	channel := newThresholdEmailChannel(emailService, 0)

	err := channel.Send(context.Background(), newEmailNotifyRequest("admin@example.com"))

	require.NoError(t, err)
	require.Equal(t, 1, emailService.attempts["admin@example.com"])
}

func newInlineEmailChannel(emailService *recordingEmailService) *EmailChannel {
	return newThresholdEmailChannel(emailService, 100)
}

func newThresholdEmailChannel(emailService *recordingEmailService, maxRecipients int) *EmailChannel {
	conf := &config.Config{}
	conf.Notification.DirectEmailMaxRecipients = maxRecipients
	conf.Notification.BroadcastEmailPageSize = 100
	return &EmailChannel{config: conf, emailService: emailService}
}

type recordingWorkflowClient struct {
	client.Client
	options client.StartWorkflowOptions
	args    []any
	err     error
	calls   int
}

func (r *recordingWorkflowClient) ExecuteWorkflow(_ context.Context, options client.StartWorkflowOptions, _ any, args ...any) (client.WorkflowRun, error) {
	r.calls++
	r.options = options
	r.args = args
	if r.err != nil {
		return nil, r.err
	}
	return recordingWorkflowRun{id: options.ID}, nil
}

type recordingWorkflowRun struct {
	client.WorkflowRun
	id string
}

func (r recordingWorkflowRun) GetID() string {
	return r.id
}

func newRecordingEmailService(failuresBeforeSuccess map[string]int) *recordingEmailService {
	if failuresBeforeSuccess == nil {
		failuresBeforeSuccess = make(map[string]int)
	}
	return &recordingEmailService{
		attempts:              make(map[string]int),
		failuresBeforeSuccess: failuresBeforeSuccess,
	}
}

func newEmailNotifyRequest(emails ...string) *notifychannel.NotifyRequest {
	receiver := &notifychannel.Receiver{IsBroadcast: false}
	receiver.AddRecipients(notifychannel.RecipientKeyUserEmails, emails)
	return &notifychannel.NotifyRequest{
		Message: types.EmailReq{
			Source: types.EmailSourceUser,
		},
		Receiver: receiver,
		FormattedData: &types.TemplateOutput{
			Title:   "Resource application",
			Content: "A resource was requested",
		},
	}
}
