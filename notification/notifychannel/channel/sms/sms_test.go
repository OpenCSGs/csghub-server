package sms

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/notification/notifychannel"
)

// MockSMSService is a mock implementation of client.SMSService
type MockSMSService struct {
	mock.Mock
}

func (m *MockSMSService) Send(req types.SMSReq) error {
	args := m.Called(req)
	return args.Error(0)
}

func TestSMSChannel_Send(t *testing.T) {
	tests := []struct {
		name           string
		request        *notifychannel.NotifyRequest
		mockSetup      func(*MockSMSService)
		expectedError  string
		expectedCalled bool
	}{
		{
			name: "successful send with valid SMS request",
			request: &notifychannel.NotifyRequest{
				Message: types.SMSReq{
					PhoneNumbers:  []string{"+1234567890"},
					SignName:      "TestSign",
					TemplateCode:  "SMS_123456",
					TemplateParam: `{"code":"123456"}`,
				},
				Receiver: &notifychannel.Receiver{
					IsBroadcast: false,
					Recipients: map[string][]string{
						"user_phone_numbers": {"+1234567890"},
					},
				},
			},
			mockSetup: func(m *MockSMSService) {
				m.On("Send", mock.AnythingOfType("types.SMSReq")).Return(nil)
			},
			expectedError:  "",
			expectedCalled: true,
		},
		{
			name: "successful send with broadcast receiver",
			request: &notifychannel.NotifyRequest{
				Message: types.SMSReq{
					PhoneNumbers:  []string{"+1234567890", "+0987654321"},
					SignName:      "TestSign",
					TemplateCode:  "SMS_123456",
					TemplateParam: `{"code":"123456"}`,
				},
				Receiver: &notifychannel.Receiver{
					IsBroadcast: true,
					Recipients:  map[string][]string{},
				},
			},
			mockSetup: func(m *MockSMSService) {
				m.On("Send", mock.AnythingOfType("types.SMSReq")).Return(nil)
			},
			expectedError:  "",
			expectedCalled: true,
		},
		{
			name: "error when receiver validation fails - nil receiver",
			request: &notifychannel.NotifyRequest{
				Message: types.SMSReq{
					PhoneNumbers:  []string{"+1234567890"},
					SignName:      "TestSign",
					TemplateCode:  "SMS_123456",
					TemplateParam: `{"code":"123456"}`,
				},
				Receiver: nil,
			},
			mockSetup: func(m *MockSMSService) {
				// No expectations set - should not be called
			},
			expectedError:  "invalid receiver: receiver cannot be nil",
			expectedCalled: false,
		},
		{
			name: "error when receiver validation fails - no recipients",
			request: &notifychannel.NotifyRequest{
				Message: types.SMSReq{
					PhoneNumbers:  []string{"+1234567890"},
					SignName:      "TestSign",
					TemplateCode:  "SMS_123456",
					TemplateParam: `{"code":"123456"}`,
				},
				Receiver: &notifychannel.Receiver{
					IsBroadcast: false,
					Recipients:  map[string][]string{},
				},
			},
			mockSetup: func(m *MockSMSService) {
				// No expectations set - should not be called
			},
			expectedError:  "invalid receiver: at least one recipient type must be specified",
			expectedCalled: false,
		},
		{
			name: "error when receiver validation fails - empty recipients",
			request: &notifychannel.NotifyRequest{
				Message: types.SMSReq{
					PhoneNumbers:  []string{"+1234567890"},
					SignName:      "TestSign",
					TemplateCode:  "SMS_123456",
					TemplateParam: `{"code":"123456"}`,
				},
				Receiver: &notifychannel.Receiver{
					IsBroadcast: false,
					Recipients: map[string][]string{
						"user_phone_numbers": {},
					},
				},
			},
			mockSetup: func(m *MockSMSService) {
				// No expectations set - should not be called
			},
			expectedError:  "invalid receiver: at least one recipient must be specified",
			expectedCalled: false,
		},
		{
			name: "error when message is not SMSReq type",
			request: &notifychannel.NotifyRequest{
				Message: "invalid message type",
				Receiver: &notifychannel.Receiver{
					IsBroadcast: false,
					Recipients: map[string][]string{
						"user_phone_numbers": {"+1234567890"},
					},
				},
			},
			mockSetup: func(m *MockSMSService) {
				// No expectations set - should not be called
			},
			expectedError:  "invalid sms message format",
			expectedCalled: false,
		},
		{
			name: "error when message is nil",
			request: &notifychannel.NotifyRequest{
				Message: nil,
				Receiver: &notifychannel.Receiver{
					IsBroadcast: false,
					Recipients: map[string][]string{
						"user_phone_numbers": {"+1234567890"},
					},
				},
			},
			mockSetup: func(m *MockSMSService) {
				// Should be called with empty SMSReq
				m.On("Send", types.SMSReq{}).Return(nil)
			},
			expectedError:  "",
			expectedCalled: true,
		},
		{
			name: "error when SMS service fails",
			request: &notifychannel.NotifyRequest{
				Message: types.SMSReq{
					PhoneNumbers:  []string{"+1234567890"},
					SignName:      "TestSign",
					TemplateCode:  "SMS_123456",
					TemplateParam: `{"code":"123456"}`,
				},
				Receiver: &notifychannel.Receiver{
					IsBroadcast: false,
					Recipients: map[string][]string{
						"user_phone_numbers": {"+1234567890"},
					},
				},
			},
			mockSetup: func(m *MockSMSService) {
				m.On("Send", mock.AnythingOfType("types.SMSReq")).Return(errors.New("SMS service error"))
			},
			expectedError:  "failed to send sms",
			expectedCalled: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			mockSMS := &MockSMSService{}
			tt.mockSetup(mockSMS)

			channel := NewSMSChannel(mockSMS)
			ctx := context.Background()

			// Execute
			err := channel.Send(ctx, tt.request)

			// Assert
			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			} else {
				assert.NoError(t, err)
			}

			if tt.expectedCalled {
				mockSMS.AssertExpectations(t)
			} else {
				mockSMS.AssertNotCalled(t, "Send")
			}
		})
	}
}

func TestSMSChannel_IsFormatRequired(t *testing.T) {
	mockSMS := &MockSMSService{}
	channel := NewSMSChannel(mockSMS)

	assert.False(t, channel.IsFormatRequired())
}

func TestNewSMSChannel(t *testing.T) {
	mockSMS := &MockSMSService{}
	channel := NewSMSChannel(mockSMS)

	assert.NotNil(t, channel)
	assert.Implements(t, (*notifychannel.Notifier)(nil), channel)
}
