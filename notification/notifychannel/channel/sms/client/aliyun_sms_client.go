package client

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	openapi "github.com/alibabacloud-go/darabonba-openapi/v2/client"
	dysmsapi20170525 "github.com/alibabacloud-go/dysmsapi-20170525/v5/client"
	util "github.com/alibabacloud-go/tea-utils/v2/service"
	"github.com/alibabacloud-go/tea/tea"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
)

type SMSClient interface {
	SendSmsWithOptions(
		request *dysmsapi20170525.SendSmsRequest,
		runtime *util.RuntimeOptions,
	) (*dysmsapi20170525.SendSmsResponse, error)
}

type AliyunSMSClient struct {
	client SMSClient
}

var _ SMSService = (*AliyunSMSClient)(nil)

func NewAliyunSMSClient(config *config.Config) (SMSService, error) {
	// Use new config fields, but maintain backward compatibility
	accessKeyID := config.Notification.SMSAccessKeyID
	accessKeySecret := config.Notification.SMSAccessKeySecret

	// If new config fields are empty, try to use old config fields (compatibility handling)
	if accessKeyID == "" {
		accessKeyID = config.Notification.SMSAccessKeyID
	}
	if accessKeySecret == "" {
		accessKeySecret = config.Notification.SMSAccessKeySecret
	}

	if accessKeyID == "" || accessKeySecret == "" {
		return nil, fmt.Errorf("Aliyun SMS configuration incomplete, please set ALIYUN_SMS_ACCESS_KEY_ID and ALIYUN_SMS_ACCESS_KEY_SECRET")
	}

	SMSConfig := &openapi.Config{
		AccessKeyId:     tea.String(accessKeyID),
		AccessKeySecret: tea.String(accessKeySecret),
		Endpoint:        tea.String(config.Notification.SMSEndpoint),
	}
	client, err := dysmsapi20170525.NewClient(SMSConfig)
	if err != nil {
		return nil, err
	}
	return &AliyunSMSClient{
		client: client,
	}, nil
}

func (c *AliyunSMSClient) Send(req types.SMSReq) error {
	// refer to sms client doc, the phone area should not have '+' prefix when send sms code to overseas,
	for i, phoneNumber := range req.PhoneNumbers {
		req.PhoneNumbers[i] = strings.TrimPrefix(phoneNumber, "+")
	}
	phoneNumbers := strings.Join(req.PhoneNumbers, ",")

	templateParam, err := json.Marshal(req.MapParams)
	if err != nil {
		slog.Error("Failed to marshal map params to JSON", slog.Any("error", err))
		return err
	}
	smsReq := &dysmsapi20170525.SendSmsRequest{
		PhoneNumbers:  tea.String(phoneNumbers),
		SignName:      tea.String(req.SignName),
		TemplateCode:  tea.String(req.TemplateCode),
		TemplateParam: tea.String(string(templateParam)),
	}

	_, err = c.client.SendSmsWithOptions(smsReq, &util.RuntimeOptions{})
	if err != nil {
		return err
	}
	return nil
}
