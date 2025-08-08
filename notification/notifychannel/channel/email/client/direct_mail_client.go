package client

import (
	"bytes"
	"fmt"
	"log/slog"
	"strings"

	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/notification/utils"

	openapi "github.com/alibabacloud-go/darabonba-openapi/v2/client"
	dm20151123 "github.com/alibabacloud-go/dm-20151123/v2/client"
	"github.com/alibabacloud-go/tea/dara"
	"github.com/alibabacloud-go/tea/tea"
	credential "github.com/aliyun/credentials-go/credentials"
)

type directMailClient struct {
	dmClient *dm20151123.Client
}

var _ EmailService = (*directMailClient)(nil)

func NewDirectMailClient(config *config.Config) (EmailService, error) {
	credential, err := credential.NewCredential(&credential.Config{
		Type:            tea.String("access_key"),
		AccessKeyId:     tea.String(config.Notification.DirectMailAccessKeyID),
		AccessKeySecret: tea.String(config.Notification.DirectMailAccessKeySecret),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create credential for direct mail client: %w", err)
	}

	dmConfig := &openapi.Config{
		Endpoint:   tea.String(config.Notification.DirectMailEndpoint),
		RegionId:   tea.String(config.Notification.DirectMailRegionId),
		Credential: credential,
	}

	dmClient, err := dm20151123.NewClient(dmConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create direct mail client: %w", err)
	}

	return &directMailClient{
		dmClient: dmClient,
	}, nil
}

func (c *directMailClient) Send(req types.EmailReq) error {
	if c.dmClient == nil {
		return fmt.Errorf("direct mail client is not initialized")
	}

	toAddress := strings.Join(req.To, ",")
	displayName := utils.ExtractDisplayNameFromEmail(req.From)

	sendMailRequest := &dm20151123.SingleSendMailAdvanceRequest{
		AccountName:    tea.String(req.From),
		FromAlias:      tea.String(displayName),
		AddressType:    tea.Int32(1), // 1 for sender address
		ReplyToAddress: tea.Bool(false),
		ToAddress:      tea.String(toAddress),
		Subject:        tea.String(req.Subject),
	}

	if req.ContentType == types.ContentTypeTextHTML {
		sendMailRequest.HtmlBody = tea.String(req.Body)
	} else {
		sendMailRequest.TextBody = tea.String(req.Body)
	}

	if len(req.CC) > 0 {
		slog.Warn("cc is not supported for direct mail, will be ignored", "cc", req.CC)
	}

	if len(req.BCC) > 0 {
		slog.Warn("bcc is not supported for direct mail, will be ignored", "bcc", req.BCC)
	}

	// add attachments
	if len(req.Attachments) > 0 {
		for _, attachment := range req.Attachments {
			singleAttachment := &dm20151123.SingleSendMailAdvanceRequestAttachments{
				AttachmentName:      tea.String(attachment.Name),
				AttachmentUrlObject: bytes.NewReader(attachment.Content),
			}
			sendMailRequest.Attachments = append(sendMailRequest.Attachments, singleAttachment)
		}
	}

	runtime := &dara.RuntimeOptions{}
	_, err := c.dmClient.SingleSendMailAdvance(sendMailRequest, runtime)
	if err != nil {
		slog.Error("failed to send email via direct mail", "error", err, "to", req.To, "subject", req.Subject)
		return fmt.Errorf("failed to send email: %w", err)
	}

	return nil
}
