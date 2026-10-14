package workflow

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/testsuite"
	"opencsg.com/csghub-server/common/types"
	emailactivity "opencsg.com/csghub-server/notification/notifychannel/channel/email/workflow/activity"
)

func TestBroadcastEmailWorkflowSendsExplicitRecipientList(t *testing.T) {
	env := (&testsuite.WorkflowTestSuite{}).NewTestWorkflowEnvironment()
	var sent []types.EmailReq
	env.RegisterActivityWithOptions(func(emailReq types.EmailReq) error {
		sent = append(sent, emailReq)
		return nil
	}, activity.RegisterOptions{Name: SendExplicitEmailListActivity})

	emailReq := types.EmailReq{
		To:      []string{"a@example.com", "b@example.com", "c@example.com"},
		Subject: "resource application",
		Source:  types.EmailSourceUser,
	}
	env.ExecuteWorkflow(BroadcastEmailWorkflow, emailReq, 2)

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())
	require.Equal(t, []types.EmailReq{emailReq}, sent)
}

func TestBroadcastEmailWorkflowPagesUsersWhenToIsEmpty(t *testing.T) {
	env := (&testsuite.WorkflowTestSuite{}).NewTestWorkflowEnvironment()
	var pages []int
	var batches [][]string
	env.RegisterActivityWithOptions(func(input emailactivity.BatchSendEmail) (emailactivity.BatchGetEmailResult, error) {
		pages = append(pages, input.Page)
		if input.Page == 1 {
			return emailactivity.BatchGetEmailResult{
				Emails:    []string{"u1@example.com"},
				MorePages: true,
			}, nil
		}
		return emailactivity.BatchGetEmailResult{
			Emails:    []string{"u2@example.com"},
			MorePages: false,
		}, nil
	}, activity.RegisterOptions{Name: GetEmailFromUserActivity})
	env.RegisterActivityWithOptions(func(_ types.EmailReq, emails []string) error {
		batches = append(batches, append([]string(nil), emails...))
		return nil
	}, activity.RegisterOptions{Name: SendEmailBatchActivity})

	env.ExecuteWorkflow(BroadcastEmailWorkflow, types.EmailReq{
		Source: types.EmailSourceUser,
	}, 100)

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())
	require.Equal(t, []int{1, 2}, pages)
	require.Equal(t, [][]string{{"u1@example.com"}, {"u2@example.com"}}, batches)
}
