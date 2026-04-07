package client

import (
	"fmt"
	"strings"

	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/errors"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/profile"
	sms "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/sms/v20190711"

	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
)

// TencentSMSClient Tencent Cloud SMS client
type TencentSMSClient struct {
	client *sms.Client
	config *config.Config
}

var _ SMSService = (*TencentSMSClient)(nil)

// NewTencentSMSClient creates a Tencent Cloud SMS client
func NewTencentSMSClient(config *config.Config) (SMSService, error) {
	// Validate configuration
	if config.Notification.SMSAccessKeySecret == "" || config.Notification.SMSAccessKeyID == "" {
		return nil, fmt.Errorf("Tencent SMS configuration incomplete, please set TENCENT_SMS_SECRET_ID and TENCENT_SMS_SECRET_KEY")
	}

	if config.Notification.SMSAppID == "" {
		return nil, fmt.Errorf("Tencent SMS configuration incomplete, please set TENCENT_SMS_SDK_APP_ID")
	}

	// Initialize Tencent Cloud SDK client
	credential := common.NewCredential(
		config.Notification.SMSAccessKeyID,
		config.Notification.SMSAccessKeySecret,
	)

	cpf := profile.NewClientProfile()
	cpf.HttpProfile.Endpoint = config.Notification.SMSEndpoint

	client, err := sms.NewClient(credential, config.Notification.SMSRegion, cpf)
	if err != nil {
		return nil, fmt.Errorf("failed to create Tencent SMS client: %w", err)
	}

	return &TencentSMSClient{
		client: client,
		config: config,
	}, nil
}

// Send sends SMS
func (c *TencentSMSClient) Send(req types.SMSReq) error {
	// Process phone number format (remove + prefix and add country code if needed)
	phoneNumbers := make([]string, len(req.PhoneNumbers))
	for i, phoneNumber := range req.PhoneNumbers {
		phoneNumbers[i] = strings.TrimPrefix(phoneNumber, "+")
		// Tencent Cloud requires phone numbers in format "+[CountryCode][PhoneNumber]"
		// We assume Chinese numbers (86) if not specified
		if !strings.HasPrefix(phoneNumbers[i], "86") && len(phoneNumbers[i]) == 11 {
			phoneNumbers[i] = "86" + phoneNumbers[i]
		}
	}

	// Build Tencent Cloud SMS request (v20190711 API version)
	request := sms.NewSendSmsRequest()

	// Set SDK AppId (note: field name is SmsSdkAppid in v20190711)
	request.SmsSdkAppid = common.StringPtr(c.config.Notification.SMSAppID)

	// Set sign name (Tencent Cloud calls it "Sign" in v20190711)
	request.Sign = common.StringPtr(req.SignName)

	// Set template ID (note: field name is TemplateID in v20190711)
	request.TemplateID = common.StringPtr(req.TemplateCode)

	request.TemplateParamSet = common.StringPtrs(req.Params)

	// Set phone numbers
	request.PhoneNumberSet = common.StringPtrs(phoneNumbers)

	// Note: SenderId is optional in Tencent Cloud, we don't have this field in config
	// If needed in the future, we can add TencentSMSSenderId field to config

	// Send SMS
	response, err := c.client.SendSms(request)
	if err != nil {
		// Check if it's a Tencent Cloud SDK error
		if sdkErr, ok := err.(*errors.TencentCloudSDKError); ok {
			return fmt.Errorf("Tencent Cloud SDK error: Code=%s, Message=%s", sdkErr.Code, sdkErr.Message)
		}
		return fmt.Errorf("failed to send SMS via Tencent Cloud: %w", err)
	}

	// Check response
	if response.Response == nil {
		return fmt.Errorf("empty response from Tencent Cloud")
	}

	// Check each send status
	for _, status := range response.Response.SendStatusSet {
		if status == nil {
			continue
		}
		if *status.Code != "Ok" {
			return fmt.Errorf("failed to send SMS to %s: %s", *status.PhoneNumber, *status.Message)
		}
	}

	return nil
}
