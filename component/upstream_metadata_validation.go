package component

import (
	"fmt"

	"opencsg.com/csghub-server/common/types"
)

func validateUpstreamMetadata(metadata *types.UpstreamMetadata) error {
	if metadata == nil {
		return nil
	}
	if metadata.ResponsesChatAdapter == nil {
		return nil
	}
	rr := metadata.ResponsesChatAdapter.ReasoningRequest
	if rr == nil {
		return nil
	}
	effortField := rr.EffortField
	if rr.EnableExtra != nil {
		if effortField != "" {
			if _, exists := rr.EnableExtra[effortField]; exists {
				return fmt.Errorf("%w: metadata.responses.chat_adapter.reasoning_request.effort_field must not also appear in enable_extra", ErrInvalidLLMConfig)
			}
		}
	}
	if rr.DisableExtra != nil {
		if effortField != "" {
			if _, exists := rr.DisableExtra[effortField]; exists {
				return fmt.Errorf("%w: metadata.responses.chat_adapter.reasoning_request.effort_field must not also appear in disable_extra", ErrInvalidLLMConfig)
			}
		}
	}
	return nil
}
