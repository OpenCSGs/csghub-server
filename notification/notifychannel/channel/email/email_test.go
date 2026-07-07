package email

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/notification/notifychannel"
	"opencsg.com/csghub-server/notification/utils"
)

type recordingEmailService struct {
	attempts              map[string]int
	failuresBeforeSuccess map[string]int
}

func (s *recordingEmailService) Send(req types.EmailReq) error {
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
		channel := &EmailChannel{emailService: emailService}

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
		channel := &EmailChannel{emailService: emailService}

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
		channel := &EmailChannel{emailService: emailService}

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
		channel := &EmailChannel{emailService: emailService}

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
