package mailer_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/notification/mailer"
)

func TestFormatNotifyMsg(t *testing.T) {
	cfg := &config.Config{}
	cfg.Notification.MailerHost = "smtp.example.com"
	cfg.Notification.MailerPort = 587
	cfg.Notification.MailerUsername = "test@example.com"
	cfg.Notification.MailerPassword = "password"
	m := mailer.NewMailer(cfg)

	msg := types.NotificationMessage{
		MsgUUID:          "test-uuid-456",
		UserUUIDs:        []string{"user-uuid-456"},
		SenderUUID:       "sender-uuid-456",
		NotificationType: types.NotificationComment,
		Title:            "Comment Notification",
		Summary:          "Someone commented on your repository",
		Content:          "User John Doe commented: Great work on this feature!",
		ClickActionURL:   "https://example.com/comment/123",
		Priority:         2,
		CreateAt:         time.Now(),
	}

	formattedMsg, err := m.FormatNotifyMsg(msg)
	assert.NoError(t, err)
	assert.NotEmpty(t, formattedMsg)

	// Verify the formatted message contains the expected content
	assert.Contains(t, formattedMsg, msg.Title)
	assert.Contains(t, formattedMsg, msg.Summary)
	assert.Contains(t, formattedMsg, msg.Content)

	// Verify it's HTML formatted
	assert.Contains(t, formattedMsg, "<h3>")
	assert.Contains(t, formattedMsg, "</h3>")
	assert.Contains(t, formattedMsg, "<p>")
	assert.Contains(t, formattedMsg, "</p>")
}
