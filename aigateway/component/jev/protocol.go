// Package jev holds the component-layer contract helpers for the Jev
// (System One) protocol served under /v1/systemone.  The upstream exchange
// itself is a unified reverse-proxy pass-through executed by the handler;
// this package owns the protocol-level rules that stay independent of how
// the exchange is performed: what a success response must contain, how to
// read an upstream failure body, and the default call timeout.
package jev

import (
	"encoding/json"
	"errors"

	"opencsg.com/csghub-server/aigateway/types"
)

// ErrMissingRequiredFields marks a 2xx body that decodes as JSON but lacks
// fields the protocol guarantees on a success response.  Accepting it would
// render a protocol failure as a zero-usage success completion.
var ErrMissingRequiredFields = errors.New("success response is missing required fields (answers, usage)")

// ValidateSuccessResponse enforces the required fields of a successful
// System One response: answers must be present (an empty object is allowed)
// and usage must be reported.
func ValidateSuccessResponse(resp *types.JevResponse) error {
	if resp == nil || resp.Answers == nil || resp.Usage == nil {
		return ErrMissingRequiredFields
	}
	return nil
}

// ExtractUpstreamErrorMessage attempts to extract a human-readable message
// from an upstream error body: {"error":{"message":"..."}} first, then
// {"error":"..."} / {"message":"..."}, then the truncated raw body.
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
