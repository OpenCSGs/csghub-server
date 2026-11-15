// Package decisions contains protocol contract checks for OpenAI Decisions.
package decisions

import (
	"bytes"
	"encoding/json"
	"errors"

	"opencsg.com/csghub-server/aigateway/types"
)

var ErrMissingRequiredFields = errors.New("success response is missing required fields (answers, usage)")

func ValidateSuccessResponse(resp *types.DecisionResponse) error {
	if resp == nil || resp.Usage == nil {
		return ErrMissingRequiredFields
	}
	answers := bytes.TrimSpace(resp.Answers)
	if len(answers) == 0 || !bytes.HasPrefix(answers, []byte("[")) {
		return ErrMissingRequiredFields
	}
	return nil
}

// ExtractUpstreamErrorMessage extracts the common OpenAI error envelope,
// falling back to a bounded excerpt of the raw response.
func ExtractUpstreamErrorMessage(body []byte) string {
	if len(body) == 0 {
		return "upstream returned an error with no response body"
	}
	var nested struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &nested); err == nil && nested.Error.Message != "" {
		return nested.Error.Message
	}
	var simple struct {
		Error   string `json:"error"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(body, &simple); err == nil {
		if simple.Error != "" {
			return simple.Error
		}
		if simple.Message != "" {
			return simple.Message
		}
	}
	const maxLen = 512
	if len(body) > maxLen {
		return string(body[:maxLen])
	}
	return string(body)
}
