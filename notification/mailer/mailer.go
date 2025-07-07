package mailer

import (
	"bytes"
	"embed"
	"fmt"
	"html/template"
	"io/fs"

	"gopkg.in/gomail.v2"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
)

//go:embed templates
var Templates embed.FS

type MailerInterface interface {
	Send(req types.EmailReq) error
	FormatNotifyMsg(msg types.NotificationMessage) (string, error)
}

type mailer struct {
	dialer   *gomail.Dialer
	host     string
	port     int
	username string
	password string
}

var _ MailerInterface = (*mailer)(nil)

func NewMailer(config *config.Config) MailerInterface {
	dialer := gomail.NewDialer(
		config.Notification.MailerHost,
		config.Notification.MailerPort,
		config.Notification.MailerUsername,
		config.Notification.MailerPassword,
	)
	return &mailer{
		dialer:   dialer,
		host:     config.Notification.MailerHost,
		port:     config.Notification.MailerPort,
		username: config.Notification.MailerUsername,
		password: config.Notification.MailerPassword,
	}
}

func (m *mailer) Send(req types.EmailReq) error {
	message := gomail.NewMessage()

	message.SetHeader("From", m.username)
	message.SetHeader("To", req.To...)
	message.SetHeader("Cc", req.CC...)
	message.SetHeader("Bcc", req.BCC...)
	message.SetHeader("Subject", req.Subject)
	if req.ContentType == "" {
		req.ContentType = types.ContentTypeTextPlain
	}
	message.SetBody(string(req.ContentType), req.Body)

	for _, attachment := range req.Attachments {
		message.Attach(attachment.Path, gomail.Rename(attachment.Name))
	}

	err := m.dialer.DialAndSend(message)
	if err != nil {
		return fmt.Errorf("failed to send email: %w", err)
	}

	return nil
}

func (m *mailer) FormatNotifyMsg(msg types.NotificationMessage) (string, error) {
	tmpls, err := fs.Sub(Templates, "templates")
	if err != nil {
		return "", fmt.Errorf("failed to load templates: %v", err)
	}
	tmpl, err := template.ParseFS(tmpls, "notification_email.html")
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %v", err)
	}

	var body bytes.Buffer
	if err := tmpl.Execute(&body, msg); err != nil {
		return "", fmt.Errorf("failed to execute template: %v", err)
	}
	return body.String(), nil
}
