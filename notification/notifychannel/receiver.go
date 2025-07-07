package notifychannel

import (
	"fmt"
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
	RecipientKeyUserUUIDs  = "user_uuids"
	RecipientKeyUserEmails = "user_emails"
)

func (r *Receiver) GetUserUUIDs() []string {
	return r.Recipients[RecipientKeyUserUUIDs]
}

func (r *Receiver) GetUserEmails() []string {
	return r.Recipients[RecipientKeyUserEmails]
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
