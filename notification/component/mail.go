package component

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/nats-io/nats.go/jetstream"
	"opencsg.com/csghub-server/builder/rpc"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/mq"
	"opencsg.com/csghub-server/notification/mailer"
)

type MailComponent interface{}

type mailComponentImpl struct {
	conf     *config.Config
	consumer jetstream.Consumer

	mailer        mailer.MailerInterface
	userSvcClient rpc.UserSvcClient
}

func NewMailComponent(conf *config.Config) (MailComponent, error) {
	userSvcAddr := fmt.Sprintf("%s:%d", conf.User.Host, conf.User.Port)
	userRpcClient := rpc.NewUserSvcHttpClient(userSvcAddr, rpc.AuthWithApiKey(conf.APIToken))
	nmc := &mailComponentImpl{
		conf:          conf,
		mailer:        mailer.NewMailer(conf),
		userSvcClient: userRpcClient,
	}

	n, err := mq.GetOrInit(conf)
	if err != nil {
		slog.Error("failed to init nats", slog.Any("error", err))
		return nil, err
	}

	if err = n.BuildSiteInternalMailStream(); err != nil {
		slog.Error("failed to build site internal mail stream", slog.Any("error", err))
		return nil, err
	}
	consumer, err := n.BuildSiteInternalMailConsumer()
	if err != nil {
		slog.Error("failed to build site internal mail consumer", slog.Any("error", err))
		return nil, err
	}

	nmc.consumer = consumer
	if err = nmc.processMail(); err != nil {
		slog.Error("failed to process messages", slog.Any("error", err))
		return nil, err
	}

	return nmc, nil
}

func (c *mailComponentImpl) processMail() error {
	slog.Info("start process mail")
	_, err := c.consumer.Consume(c.handleMail)
	if err != nil {
		slog.Error("failed to consume mail", slog.Any("error", err))
		return err
	}

	return nil
}

func (c *mailComponentImpl) handleMail(msg jetstream.Msg) {
	slog.Debug("handle mail", slog.Any("data", string(msg.Data())))
	defer func() {
		if err := msg.Ack(); err != nil {
			slog.Error("failed to ack mail", slog.Any("error", err))
		}
	}()
	var message types.MailMessage
	if err := json.Unmarshal(msg.Data(), &message); err != nil {
		slog.Error("failed to unmarshal mail", slog.Any("data", string(msg.Data())), slog.Any("error", err))
		return
	}

	err := c.sendMail(context.Background(), message)
	if err != nil {
		slog.Error("failed to handleMessage create mail task", slog.Any("mail", message), slog.Any("error", err))
	}
}

func (c *mailComponentImpl) sendMail(ctx context.Context, message types.MailMessage) error {
	if !message.MailType.IsValid() {
		return fmt.Errorf("invalid mail type: %s", message.Mails)
	}
	if message.MsgUUID == "" {
		return fmt.Errorf("msg_uuid is empty, message: %+v", message)
	}
	if len(message.Mails) == 0 {
		return fmt.Errorf("mails is empty, message: %+v", message)
	}

	switch message.MailType {
	case types.MailRechargeSucceed:
		if len(message.UserUUIDs) == 0 {
			return fmt.Errorf("userUUID is empty")
		}
		users, err := c.findByUUIDs(ctx, message.UserUUIDs)
		if err != nil {
			return fmt.Errorf("failed to sendMail: find users by UUIDs, %w", err)
		}
		if user, ok := users[message.UserUUIDs[0]]; ok && user != nil {
			message.Content = fmt.Sprintf("%s %s", user.Username, message.Content)
		} else {
			message.Content = fmt.Sprintf("%s %s", message.UserUUIDs[0], message.Content)
		}

		mailReq := types.EmailReq{
			To:          message.Mails,
			Subject:     message.Title,
			Body:        message.Content,
			ContentType: types.ContentTypeTextHTML,
		}
		if err := c.mailer.Send(mailReq); err != nil {
			return fmt.Errorf("failed to send emails, %w", err)
		}
	case types.MailWeeklyRecharges:
		err := c.sendMailWeeklyRecharges(message)
		if err != nil {
			return fmt.Errorf("failed to send weekly recharges, %w", err)
		}
	}

	return nil
}

func (c *mailComponentImpl) sendMailWeeklyRecharges(message types.MailMessage) error {
	csvBytes, err := makeWeeklyRechargesCSV(message.DataJson)
	if err != nil {
		return fmt.Errorf("sendMailWeeklyRecharges: failed to generate CSV: %w", err)
	}

	tmpFile, err := os.CreateTemp("", "weekly_recharges_*.csv")
	if err != nil {
		return fmt.Errorf("sendMailWeeklyRecharges: failed to create temp file: %w", err)
	}
	defer func() {
		if cerr := tmpFile.Close(); cerr != nil {
			slog.Warn("sendMailWeeklyRecharges: failed to close temp file", slog.Any("error", cerr))
		}
		if rerr := os.Remove(tmpFile.Name()); rerr != nil {
			slog.Warn("sendMailWeeklyRecharges: failed to remove temp file", slog.String("file", tmpFile.Name()), slog.Any("error", rerr))
		}
	}()

	if _, err := tmpFile.Write(csvBytes); err != nil {
		return fmt.Errorf("sendMailWeeklyRecharges: failed to write CSV to temp file: %w", err)
	}

	mailReq := types.EmailReq{
		To:          message.Mails,
		Subject:     message.Title,
		Body:        message.Content,
		ContentType: types.ContentTypeTextHTML,
		Attachments: []types.EmailAttachment{{Path: tmpFile.Name(), Name: message.FileName}},
	}

	if err := c.mailer.Send(mailReq); err != nil {
		return fmt.Errorf("failed to send email: %w", err)
	}
	return nil
}

func makeWeeklyRechargesCSV(data string) ([]byte, error) {
	bom := []byte{0xEF, 0xBB, 0xBF}

	timestamp := time.Now().Format("2006-01-02 15:04:05")
	finalCSV := fmt.Sprintf("%s\n数据创建时间: %s\n", data, timestamp)

	return append(bom, []byte(finalCSV)...), nil
}

func (c *mailComponentImpl) findByUUIDs(ctx context.Context, uuids []string) (map[string]*types.User, error) {
	usersMap, err := c.userSvcClient.FindByUUIDs(ctx, uuids)
	if err != nil {
		return nil, err
	}

	return usersMap, nil
}
