package client

import (
	"errors"
	"testing"

	dysmsapi20170525 "github.com/alibabacloud-go/dysmsapi-20170525/v5/client"
	util "github.com/alibabacloud-go/tea-utils/v2/service"
	"opencsg.com/csghub-server/common/types"
)

type mockSMSClient struct {
	sendFunc func(req *dysmsapi20170525.SendSmsRequest, runtime *util.RuntimeOptions) (*dysmsapi20170525.SendSmsResponse, error)
}

func (m *mockSMSClient) SendSmsWithOptions(
	req *dysmsapi20170525.SendSmsRequest,
	runtime *util.RuntimeOptions,
) (*dysmsapi20170525.SendSmsResponse, error) {
	return m.sendFunc(req, runtime)
}

func TestAliyunSMSClient_Send_Success(t *testing.T) {
	mock := &mockSMSClient{
		sendFunc: func(req *dysmsapi20170525.SendSmsRequest, runtime *util.RuntimeOptions) (*dysmsapi20170525.SendSmsResponse, error) {
			if *req.PhoneNumbers != "1234567890" {
				t.Errorf("unexpected phone number: %s", *req.PhoneNumbers)
			}
			if *req.SignName != "TestSign" {
				t.Errorf("unexpected sign name: %s", *req.SignName)
			}
			return &dysmsapi20170525.SendSmsResponse{}, nil
		},
	}

	client := &AliyunSMSClient{client: mock}
	req := types.SMSReq{
		PhoneNumbers:  []string{"1234567890"},
		SignName:      "TestSign",
		TemplateCode:  "TEMPLATE_001",
		TemplateParam: `{"code":"1234"}`,
	}

	err := client.Send(req)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestAliyunSMSClient_Send_Failure(t *testing.T) {
	mock := &mockSMSClient{
		sendFunc: func(req *dysmsapi20170525.SendSmsRequest, runtime *util.RuntimeOptions) (*dysmsapi20170525.SendSmsResponse, error) {
			return nil, errors.New("send failed")
		},
	}

	client := &AliyunSMSClient{client: mock}
	req := types.SMSReq{
		PhoneNumbers:  []string{"1234567890"},
		SignName:      "TestSign",
		TemplateCode:  "TEMPLATE_001",
		TemplateParam: `{"code":"1234"}`,
	}

	err := client.Send(req)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
