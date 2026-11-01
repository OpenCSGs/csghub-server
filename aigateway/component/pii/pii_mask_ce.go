//go:build !ee && !saas

package pii

import (
	"context"

	aitypes "opencsg.com/csghub-server/aigateway/types"
	"opencsg.com/csghub-server/common/config"
)

type piiMaskerImpl struct{}

// NewPIIMasker returns an inert masker: PII masking ships in the EE/SAAS
// editions only.
func NewPIIMasker(_ *config.Config, _ PIIRuleSource) PIIMasker {
	return piiMaskerImpl{}
}

func (piiMaskerImpl) Enabled() bool { return false }

func (piiMaskerImpl) MaskChatRequest(_ context.Context, _ *aitypes.ChatCompletionRequest) bool {
	return false
}
