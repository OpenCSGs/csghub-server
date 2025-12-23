//go:build ee || saas

package csgbotcriticalerror

import (
	"context"
	"encoding/json"
	"fmt"

	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/notification/notifychannel"
	"opencsg.com/csghub-server/notification/scenariomgr"
)

// implement scenariomgr.GetDataFunc to get lark data
func GetLarkData(ctx context.Context, conf *config.Config, msg types.ScenarioMessage) (*scenariomgr.NotificationData, error) {
	var reqRaw map[string]json.RawMessage
	if err := json.Unmarshal([]byte(msg.Parameters), &reqRaw); err != nil {
		return nil, fmt.Errorf("invalid parameters: %w", err)
	}

	// Check payload exists and is valid JSON
	payloadRaw, ok := reqRaw["payload"]
	if !ok || payloadRaw == nil {
		return nil, fmt.Errorf("payload field is required")
	}
	if !json.Valid(payloadRaw) {
		return nil, fmt.Errorf("payload must be valid JSON")
	}

	// Unmarshal payload to map[string]any for template rendering
	var payload map[string]any
	if err := json.Unmarshal(payloadRaw, &payload); err != nil {
		return nil, fmt.Errorf("failed to unmarshal payload: %w", err)
	}
	if len(payload) == 0 {
		return nil, fmt.Errorf("payload is empty")
	}

	// check receiver exists and unmarshal
	var receiverData types.LarkReceiver
	receiverRaw, ok := reqRaw["receiver"]
	if !ok || receiverRaw == nil {
		return nil, fmt.Errorf("receiver field is required")
	}
	if err := json.Unmarshal(receiverRaw, &receiverData); err != nil {
		return nil, fmt.Errorf("invalid receiver: %w", err)
	}

	receiver := &notifychannel.Receiver{
		IsBroadcast: false,
	}
	formattedID := notifychannel.FormatLarkReceiveID(string(receiverData.Type), receiverData.ID)
	receiver.AddRecipients(notifychannel.RecipientKeyLarkReceiveIDs, []string{formattedID})
	receiver.SetLanguage("zh-CN")

	return &scenariomgr.NotificationData{
		Message:  nil,
		Payload:  payload,
		Receiver: receiver,
	}, nil
}
