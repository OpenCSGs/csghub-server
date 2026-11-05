package pii

import (
	"context"

	aitypes "opencsg.com/csghub-server/aigateway/types"
)

// PIIMasker masks sensitive content (PII, credentials) in request prompts
// before they reach the upstream model. The engine lives in
// builder/sensitive/pii; this component owns the gateway-side wiring
// (edition gating and protocol-level rewriting). CE builds get an inert
// masker.
type PIIMasker interface {
	// Enabled reports whether the deployment turned PII masking on.
	Enabled() bool
	// MaskChatRequest rewrites the chat request in place when it carries
	// sensitive content and reports whether anything changed. Callers gate
	// the per-request opt-in (X-PII-Check header) before invoking it.
	MaskChatRequest(ctx context.Context, chatReq *aitypes.ChatCompletionRequest) bool
}
