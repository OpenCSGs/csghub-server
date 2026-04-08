package client

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/huaweicloud/huaweicloud-sdk-go-v3/core/def"
	smsapi "github.com/huaweicloud/huaweicloud-sdk-go-v3/services/smsapi/v1"
	"github.com/huaweicloud/huaweicloud-sdk-go-v3/services/smsapi/v1/model"

	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
)

// HuaweiSMSClient Huawei Cloud SMS client
type HuaweiSMSClient struct {
	client *smsapi.SMSApiClient
	config *config.Config
}

var _ SMSService = (*HuaweiSMSClient)(nil)

// NewHuaweiSMSClient creates a Huawei Cloud SMS client
func NewHuaweiSMSClient(config *config.Config) (SMSService, error) {
	// Validate configuration
	if config.Notification.SMSAccessKeyID == "" || config.Notification.SMSAccessKeySecret == "" {
		return nil, fmt.Errorf("Huawei SMS configuration incomplete, please set HUAWEI_SMS_ACCESS_KEY_ID and HUAWEI_SMS_SECRET_KEY_SECRET")
	}

	if config.Notification.SMSAppID == "" {
		return nil, fmt.Errorf("Huawei SMS configuration incomplete, please set HUAWEI_SMS_PROJECT_ID")
	}

	// Create SMS API client builder
	builder := smsapi.SMSApiClientBuilder()

	// Build client with endpoint and credentials
	hcClient, err := builder.
		WithEndpoint(config.Notification.SMSEndpoint).
		WithCredential(&smsapi.SMSApiCredentials{
			AK: config.Notification.SMSAccessKeyID,
			SK: config.Notification.SMSAccessKeySecret,
		}).
		SafeBuild()
	if err != nil {
		return nil, fmt.Errorf("failed to build Huawei SMS client: %w", err)
	}

	// Create SMS API client
	client := smsapi.NewSMSApiClient(hcClient)

	return &HuaweiSMSClient{
		client: client,
		config: config,
	}, nil
}

// Send sends SMS
func (c *HuaweiSMSClient) Send(req types.SMSReq) error {
	// Process phone number format (remove + prefix)
	phoneNumbers := make([]string, len(req.PhoneNumbers))
	for i, phoneNumber := range req.PhoneNumbers {
		phoneNumbers[i] = strings.TrimPrefix(phoneNumber, "+")
		// Huawei Cloud requires phone numbers with country code
		// We assume Chinese numbers (86) if not specified
		if !strings.HasPrefix(phoneNumbers[i], "86") && len(phoneNumbers[i]) == 11 {
			phoneNumbers[i] = "86" + phoneNumbers[i]
		}
	}

	// Join phone numbers with comma
	to := strings.Join(phoneNumbers, ",")

	templateParam, err := json.Marshal(req.MapParams)
	if err != nil {
		slog.Error("Failed to marshal map params to JSON", slog.Any("error", err))
		return err
	}
	// Create request body
	requestBody := &model.BatchSendSmsRequestBody{
		From:       def.NewMultiPart(req.SignName),
		To:         def.NewMultiPart(to),
		TemplateId: def.NewMultiPart(req.TemplateCode),

		TemplateParas: def.NewMultiPart(string(templateParam)),
		Signature:     def.NewMultiPart(req.SignName),
	}

	// Create request
	request := &model.BatchSendSmsRequest{
		Body: requestBody,
	}

	// Send SMS
	response, err := c.client.BatchSendSms(request)
	if err != nil {
		return fmt.Errorf("failed to send SMS via Huawei Cloud: %w", err)
	}

	// Check response
	if response.HttpStatusCode >= 400 {
		return fmt.Errorf("Huawei Cloud API error: HTTP %d", response.HttpStatusCode)
	}

	// Check for errors in response
	if response.Code != nil && *response.Code != "" {
		errorMsg := ""
		if response.Description != nil {
			errorMsg = *response.Description
		}
		return fmt.Errorf("Huawei Cloud SMS error: %s - %s", *response.Code, errorMsg)
	}

	return nil
}
