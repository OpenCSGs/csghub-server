package component

import (
	"fmt"
)

func validateUpstreamMetadata(metadata map[string]any) error {
	if len(metadata) == 0 {
		return nil
	}
	reasoningRequest := extractReasoningRequestConfig(metadata)
	if reasoningRequest == nil {
		return nil
	}
	if enabledRaw, ok := reasoningRequest["enabled"]; ok {
		if _, ok := enabledRaw.(bool); !ok {
			return fmt.Errorf("%w: metadata.responses.chat_adapter.reasoning_request.enabled must be a boolean", ErrInvalidLLMConfig)
		}
	}
	effortField := ""
	if effortFieldRaw, ok := reasoningRequest["effort_field"]; ok {
		effortFieldStr, ok := effortFieldRaw.(string)
		if !ok {
			return fmt.Errorf("%w: metadata.responses.chat_adapter.reasoning_request.effort_field must be a string", ErrInvalidLLMConfig)
		}
		effortField = effortFieldStr
	}
	if enableExtraRaw, ok := reasoningRequest["enable_extra"]; ok {
		if err := validateReasoningExtraObject(enableExtraRaw, "enable_extra"); err != nil {
			return err
		}
		if effortField != "" {
			if hasKey, err := reasoningExtraHasKey(enableExtraRaw, effortField); err != nil {
				return err
			} else if hasKey {
				return fmt.Errorf("%w: metadata.responses.chat_adapter.reasoning_request.effort_field must not also appear in enable_extra", ErrInvalidLLMConfig)
			}
		}
	}
	if disableExtraRaw, ok := reasoningRequest["disable_extra"]; ok {
		if err := validateReasoningExtraObject(disableExtraRaw, "disable_extra"); err != nil {
			return err
		}
		if effortField != "" {
			if hasKey, err := reasoningExtraHasKey(disableExtraRaw, effortField); err != nil {
				return err
			} else if hasKey {
				return fmt.Errorf("%w: metadata.responses.chat_adapter.reasoning_request.effort_field must not also appear in disable_extra", ErrInvalidLLMConfig)
			}
		}
	}
	return nil
}

func extractReasoningRequestConfig(metadata map[string]any) map[string]any {
	responses, ok := metadata["responses"].(map[string]any)
	if !ok {
		return nil
	}
	chatAdapter, ok := responses["chat_adapter"].(map[string]any)
	if !ok {
		return nil
	}
	reasoningRequest, ok := chatAdapter["reasoning_request"].(map[string]any)
	if !ok {
		return nil
	}
	return reasoningRequest
}

func validateReasoningExtraObject(raw any, fieldName string) error {
	switch raw.(type) {
	case map[string]any:
		return nil
	default:
		return fmt.Errorf("%w: metadata.responses.chat_adapter.reasoning_request.%s must be a JSON object", ErrInvalidLLMConfig, fieldName)
	}
}

func reasoningExtraHasKey(raw any, key string) (bool, error) {
	obj, ok := raw.(map[string]any)
	if !ok {
		return false, fmt.Errorf("%w: metadata.responses.chat_adapter.reasoning_request extra must be a JSON object", ErrInvalidLLMConfig)
	}
	_, exists := obj[key]
	return exists, nil
}
