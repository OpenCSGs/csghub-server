package client

import (
	"fmt"

	"gopkg.in/gomail.v2"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
)

type emailService struct {
	dialer   *gomail.Dialer
	host     string
	port     int
	username string
	password string
}

var _ EmailService = (*emailService)(nil)

func NewEmailService(config *config.Config) EmailService {
	dialer := gomail.NewDialer(
		config.Notification.MailerHost,
		config.Notification.MailerPort,
		config.Notification.MailerUsername,
		config.Notification.MailerPassword,
	)
	return &emailService{
		dialer:   dialer,
		host:     config.Notification.MailerHost,
		port:     config.Notification.MailerPort,
		username: config.Notification.MailerUsername,
		password: config.Notification.MailerPassword,
	}
}

func (m *emailService) Send(req types.EmailReq) error {
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
