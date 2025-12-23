//go:build ee || saas

package lark

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	mockcache "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/notification/notifychannel/channel/lark/cache"
	mockclient "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/notification/notifychannel/channel/lark/client"
	"opencsg.com/csghub-server/common/types"
	notifychannel "opencsg.com/csghub-server/notification/notifychannel"
)

func TestLarkChannel_IsFormatRequired(t *testing.T) {
	mockClient := mockclient.NewMockLarkService(t)
	mockCache := mockcache.NewMockLarkCache(t)
	channel := NewChannel(mockClient, mockCache)

	assert.True(t, channel.IsFormatRequired())
}

func TestLarkChannel_Send_InvalidReceiver(t *testing.T) {
	mockClient := mockclient.NewMockLarkService(t)
	mockCache := mockcache.NewMockLarkCache(t)
	channel := NewChannel(mockClient, mockCache)

	req := &notifychannel.NotifyRequest{
		Receiver: nil,
	}

	err := channel.Send(context.Background(), req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid receiver")
}

func TestLarkChannel_Send_NilClient(t *testing.T) {
	mockCache := mockcache.NewMockLarkCache(t)
	channel := NewChannel(nil, mockCache)

	receiver := &notifychannel.Receiver{
		IsBroadcast: false,
	}
	receiver.AddRecipients(notifychannel.RecipientKeyLarkReceiveIDs, []string{"chat_id:oc_123"})

	req := &notifychannel.NotifyRequest{
		Receiver: receiver,
	}

	err := channel.Send(context.Background(), req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "lark client isn't initialized")
}

func TestLarkChannel_Send_NilCache(t *testing.T) {
	mockClient := mockclient.NewMockLarkService(t)
	channel := NewChannel(mockClient, nil)

	receiver := &notifychannel.Receiver{
		IsBroadcast: false,
	}
	receiver.AddRecipients(notifychannel.RecipientKeyLarkReceiveIDs, []string{"chat_id:oc_123"})

	req := &notifychannel.NotifyRequest{
		Receiver: receiver,
	}

	err := channel.Send(context.Background(), req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "lark cache isn't initialized")
}

func TestLarkChannel_Send_NoReceiveIDs(t *testing.T) {
	mockClient := mockclient.NewMockLarkService(t)
	mockCache := mockcache.NewMockLarkCache(t)
	channel := NewChannel(mockClient, mockCache)

	mockClient.On("GetChatIDsWithCache", mock.Anything, mockCache).Return([]string{"oc_123"}, nil)

	// Create a receiver with other recipients (passes validation) but empty lark receive IDs
	receiver := &notifychannel.Receiver{
		IsBroadcast: false,
		Recipients: map[string][]string{
			notifychannel.RecipientKeyLarkReceiveIDs: {},                   // Empty list
			notifychannel.RecipientKeyUserEmails:     {"test@example.com"}, // Has recipients to pass validation
		},
	}

	req := &notifychannel.NotifyRequest{
		Receiver: receiver,
	}

	err := channel.Send(context.Background(), req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no lark receive ids found")
}

func TestLarkChannel_Send_InvalidFormattedID(t *testing.T) {
	mockClient := mockclient.NewMockLarkService(t)
	mockCache := mockcache.NewMockLarkCache(t)
	channel := NewChannel(mockClient, mockCache)

	mockClient.On("GetChatIDsWithCache", mock.Anything, mockCache).Return([]string{"oc_123"}, nil)

	receiver := &notifychannel.Receiver{
		IsBroadcast: false,
	}
	receiver.AddRecipients(notifychannel.RecipientKeyLarkReceiveIDs, []string{"invalid_format"})

	req := &notifychannel.NotifyRequest{
		Receiver: receiver,
		FormattedData: &types.TemplateOutput{
			Content: `{"text":"test"}`,
		},
		Priority: types.MessagePriorityHigh,
	}

	err := channel.Send(context.Background(), req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse lark receive ID")
}

func TestLarkChannel_Send_ChatIDNotInAllowedList(t *testing.T) {
	mockClient := mockclient.NewMockLarkService(t)
	mockCache := mockcache.NewMockLarkCache(t)
	channel := NewChannel(mockClient, mockCache)

	mockClient.On("GetChatIDsWithCache", mock.Anything, mockCache).Return([]string{"oc_123"}, nil)

	receiver := &notifychannel.Receiver{
		IsBroadcast: false,
	}
	receiver.AddRecipients(notifychannel.RecipientKeyLarkReceiveIDs, []string{"chat_id:oc_456"})

	req := &notifychannel.NotifyRequest{
		Receiver: receiver,
		FormattedData: &types.TemplateOutput{
			Content: `{"text":"test"}`,
		},
		Priority: types.MessagePriorityHigh,
	}

	err := channel.Send(context.Background(), req)
	// Should not error, just skip the chat ID that's not in the list
	assert.NoError(t, err)
	mockClient.AssertExpectations(t)
}

func TestLarkChannel_Send_HighPriority_Success(t *testing.T) {
	mockClient := mockclient.NewMockLarkService(t)
	mockCache := mockcache.NewMockLarkCache(t)
	channel := NewChannel(mockClient, mockCache)

	chatIDs := []string{"oc_123"}
	mockClient.On("GetChatIDsWithCache", mock.Anything, mockCache).Return(chatIDs, nil)
	mockClient.On("SendChatMessage", mock.Anything, "oc_123", "post", `{"text":"test"}`).Return("msg_123", nil)

	receiver := &notifychannel.Receiver{
		IsBroadcast: false,
	}
	receiver.AddRecipients(notifychannel.RecipientKeyLarkReceiveIDs, []string{"chat_id:oc_123"})

	req := &notifychannel.NotifyRequest{
		Receiver: receiver,
		FormattedData: &types.TemplateOutput{
			Content: `{"text":"test"}`,
		},
		Priority: types.MessagePriorityHigh,
	}

	err := channel.Send(context.Background(), req)
	assert.NoError(t, err)
	mockClient.AssertExpectations(t)
	mockCache.AssertExpectations(t)
}

func TestLarkChannel_Send_NormalPriority_Success(t *testing.T) {
	mockClient := mockclient.NewMockLarkService(t)
	mockCache := mockcache.NewMockLarkCache(t)
	channel := NewChannel(mockClient, mockCache)

	chatIDs := []string{"oc_123"}
	mockClient.On("GetChatIDsWithCache", mock.Anything, mockCache).Return(chatIDs, nil)

	receiver := &notifychannel.Receiver{
		IsBroadcast: false,
	}
	receiver.AddRecipients(notifychannel.RecipientKeyLarkReceiveIDs, []string{"chat_id:oc_123"})

	req := &notifychannel.NotifyRequest{
		Receiver: receiver,
		FormattedData: &types.TemplateOutput{
			Content: `{"text":"test"}`,
		},
		Priority: types.MessagePriorityNormal,
	}

	// Create a message that will be pushed to cache
	expectedMessage := types.LarkMessage{
		ReceiveID:     "oc_123",
		ReceiveIDType: types.LarkMessageReceiveIDTypeChatID,
		MsgType:       types.LarkMessageTypePost,
		Content:       `{"text":"test"}`,
		Priority:      types.LarkMessagePriorityNormal,
	}

	mockCache.On("PushMessage", mock.Anything, mock.MatchedBy(func(msg types.LarkMessage) bool {
		return msg.ReceiveID == expectedMessage.ReceiveID &&
			msg.ReceiveIDType == expectedMessage.ReceiveIDType &&
			msg.MsgType == expectedMessage.MsgType &&
			msg.Content == expectedMessage.Content &&
			msg.Priority == expectedMessage.Priority
	})).Return(nil)

	err := channel.Send(context.Background(), req)
	assert.NoError(t, err)
	mockClient.AssertExpectations(t)
	mockCache.AssertExpectations(t)
}

func TestLarkChannel_Send_OpenID_Success(t *testing.T) {
	mockClient := mockclient.NewMockLarkService(t)
	mockCache := mockcache.NewMockLarkCache(t)
	channel := NewChannel(mockClient, mockCache)

	chatIDs := []string{"oc_123"}
	mockClient.On("GetChatIDsWithCache", mock.Anything, mockCache).Return(chatIDs, nil)
	mockClient.On("SendChatMessage", mock.Anything, "ou_456", "post", `{"text":"test"}`).Return("msg_123", nil)

	receiver := &notifychannel.Receiver{
		IsBroadcast: false,
	}
	receiver.AddRecipients(notifychannel.RecipientKeyLarkReceiveIDs, []string{"open_id:ou_456"})

	req := &notifychannel.NotifyRequest{
		Receiver: receiver,
		FormattedData: &types.TemplateOutput{
			Content: `{"text":"test"}`,
		},
		Priority: types.MessagePriorityHigh,
	}

	err := channel.Send(context.Background(), req)
	assert.NoError(t, err)
	mockClient.AssertExpectations(t)
}

func TestLarkChannel_Send_GetChatIDsError(t *testing.T) {
	mockClient := mockclient.NewMockLarkService(t)
	mockCache := mockcache.NewMockLarkCache(t)
	channel := NewChannel(mockClient, mockCache)

	mockClient.On("GetChatIDsWithCache", mock.Anything, mockCache).Return([]string(nil), errors.New("cache error"))

	receiver := &notifychannel.Receiver{
		IsBroadcast: false,
	}
	receiver.AddRecipients(notifychannel.RecipientKeyLarkReceiveIDs, []string{"chat_id:oc_123"})

	req := &notifychannel.NotifyRequest{
		Receiver: receiver,
		FormattedData: &types.TemplateOutput{
			Content: `{"text":"test"}`,
		},
		Priority: types.MessagePriorityHigh,
	}

	err := channel.Send(context.Background(), req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get chat ids from cache")
}

func TestLarkChannel_Send_HighPriority_SendError(t *testing.T) {
	mockClient := mockclient.NewMockLarkService(t)
	mockCache := mockcache.NewMockLarkCache(t)
	channel := NewChannel(mockClient, mockCache)

	chatIDs := []string{"oc_123"}
	mockClient.On("GetChatIDsWithCache", mock.Anything, mockCache).Return(chatIDs, nil)
	mockClient.On("SendChatMessage", mock.Anything, "oc_123", "post", `{"text":"test"}`).Return("", errors.New("send error"))

	receiver := &notifychannel.Receiver{
		IsBroadcast: false,
	}
	receiver.AddRecipients(notifychannel.RecipientKeyLarkReceiveIDs, []string{"chat_id:oc_123"})

	req := &notifychannel.NotifyRequest{
		Receiver: receiver,
		FormattedData: &types.TemplateOutput{
			Content: `{"text":"test"}`,
		},
		Priority: types.MessagePriorityHigh,
	}

	err := channel.Send(context.Background(), req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to send lark message")
}

func TestLarkChannel_Send_NormalPriority_PushError(t *testing.T) {
	mockClient := mockclient.NewMockLarkService(t)
	mockCache := mockcache.NewMockLarkCache(t)
	channel := NewChannel(mockClient, mockCache)

	chatIDs := []string{"oc_123"}
	mockClient.On("GetChatIDsWithCache", mock.Anything, mockCache).Return(chatIDs, nil)
	mockCache.On("PushMessage", mock.Anything, mock.Anything).Return(errors.New("push error"))

	receiver := &notifychannel.Receiver{
		IsBroadcast: false,
	}
	receiver.AddRecipients(notifychannel.RecipientKeyLarkReceiveIDs, []string{"chat_id:oc_123"})

	req := &notifychannel.NotifyRequest{
		Receiver: receiver,
		FormattedData: &types.TemplateOutput{
			Content: `{"text":"test"}`,
		},
		Priority: types.MessagePriorityNormal,
	}

	err := channel.Send(context.Background(), req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to push lark message to cache")
}

func TestLarkChannel_Send_MultipleRecipients(t *testing.T) {
	mockClient := mockclient.NewMockLarkService(t)
	mockCache := mockcache.NewMockLarkCache(t)
	channel := NewChannel(mockClient, mockCache)

	chatIDs := []string{"oc_123", "oc_456"}
	mockClient.On("GetChatIDsWithCache", mock.Anything, mockCache).Return(chatIDs, nil)
	mockClient.On("SendChatMessage", mock.Anything, "oc_123", "post", `{"text":"test"}`).Return("msg_123", nil)
	mockClient.On("SendChatMessage", mock.Anything, "oc_456", "post", `{"text":"test"}`).Return("msg_456", nil)

	receiver := &notifychannel.Receiver{
		IsBroadcast: false,
	}
	receiver.AddRecipients(notifychannel.RecipientKeyLarkReceiveIDs, []string{"chat_id:oc_123", "chat_id:oc_456"})

	req := &notifychannel.NotifyRequest{
		Receiver: receiver,
		FormattedData: &types.TemplateOutput{
			Content: `{"text":"test"}`,
		},
		Priority: types.MessagePriorityHigh,
	}

	err := channel.Send(context.Background(), req)
	assert.NoError(t, err)
	mockClient.AssertExpectations(t)
}
