package notifychannel

import (
	"fmt"
	"strings"

	"opencsg.com/csghub-server/common/types"
)

type Receiver struct {
	// is broadcast
	IsBroadcast bool `json:"is_broadcast"`
	// recipient data - flexible key-value pairs
	Recipients map[string][]string `json:"recipients"`
	// channel specific metadata
	Metadata map[string]any `json:"metadata,omitempty"`
}

// common recipient keys
const (
	RecipientKeyUserUUIDs        = "user_uuids"
	RecipientKeyUserEmails       = "user_emails"
	RecipientKeyUserPhoneNumbers = "user_phone_numbers"
	RecipientKeyLarkReceiveIDs   = "lark_receive_ids"
)

func (r *Receiver) GetUserUUIDs() []string {
	return r.Recipients[RecipientKeyUserUUIDs]
}

func (r *Receiver) GetUserEmails() []string {
	return r.Recipients[RecipientKeyUserEmails]
}

func (r *Receiver) GetUserPhoneNumbers() []string {
	return r.Recipients[RecipientKeyUserPhoneNumbers]
}

func (r *Receiver) GetRecipients(recipientType string) []string {
	return r.Recipients[recipientType]
}

func (r *Receiver) AddRecipients(recipientType string, recipients []string) {
	if r.Recipients == nil {
		r.Recipients = make(map[string][]string)
	}
	r.Recipients[recipientType] = append(r.Recipients[recipientType], recipients...)
}

func (r *Receiver) GetMetadata(key string) any {
	return r.Metadata[key]
}

func (r *Receiver) SetMetadata(key string, value any) {
	if r.Metadata == nil {
		r.Metadata = make(map[string]any)
	}
	r.Metadata[key] = value
}

func (r *Receiver) Validate() error {
	if r == nil {
		return fmt.Errorf("receiver cannot be nil")
	}

	if r.IsBroadcast {
		return nil
	}

	if len(r.Recipients) == 0 {
		return fmt.Errorf("at least one recipient type must be specified")
	}

	hasRecipients := false
	for _, recipients := range r.Recipients {
		if len(recipients) > 0 {
			hasRecipients = true
			break
		}
	}

	if !hasRecipients {
		return fmt.Errorf("at least one recipient must be specified")
	}

	return nil
}

func (r *Receiver) SetLanguage(lang string) {
	r.SetMetadata("language", lang)
}

func (r *Receiver) GetLanguage() string {
	lang := r.GetMetadata("language")
	if lang == nil {
		return "en-US"
	}
	return lang.(string)
}

// FormatLarkReceiveID formats a lark receive ID with type in the format "{type}:{id}"
func FormatLarkReceiveID(receiveIDType, receiveID string) string {
	return fmt.Sprintf("%s:%s", receiveIDType, receiveID)
}

// ParseLarkReceiveID parses a formatted lark receive ID "{type}:{id}" and returns type and id
func ParseLarkReceiveID(formattedID string) (receiveIDType, receiveID string, err error) {
	parts := strings.SplitN(formattedID, ":", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid lark receive ID format, expected '{type}:{id}', got: %s", formattedID)
	}

	if !types.LarkMessageReceiveIDType(parts[0]).IsValid() {
		return "", "", fmt.Errorf("invalid lark receive ID type, expected 'chat_id' or 'open_id', got: %s", parts[0])
	}

	if parts[1] == "" {
		return "", "", fmt.Errorf("lark receive ID is empty")
	}

	return parts[0], parts[1], nil
}
