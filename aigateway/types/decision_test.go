package types

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDecisionRequestPreservesOpaqueInput(t *testing.T) {
	raw := []byte(`{"model":"gpt-6-luna","input":[{"role":"user","content":[{"type":"input_text","text":"inspect"},{"type":"input_image","image_url":"data:image/png;base64,AAAA"}]}],"questions":[{"type":"predicate","name":"damage","instructions":"is it damaged?"}],"future_field":{"keep":true}}`)
	var req DecisionRequest
	require.NoError(t, json.Unmarshal(raw, &req))
	require.NoError(t, req.Validate())
	require.JSONEq(t, `[{"role":"user","content":[{"type":"input_text","text":"inspect"},{"type":"input_image","image_url":"data:image/png;base64,AAAA"}]}]`, string(req.Input))
	require.Equal(t, raw, []byte(req.RawBody))
}

func TestDecisionRequestValidateRequiresFields(t *testing.T) {
	require.ErrorContains(t, (&DecisionRequest{}).Validate(), "model is required")
	require.ErrorContains(t, (&DecisionRequest{Model: "m"}).Validate(), "input is required")
	require.ErrorContains(t, (&DecisionRequest{Model: "m", Input: json.RawMessage(`""`)}).Validate(), "questions is required")
}

func TestDecisionRequestPromptText(t *testing.T) {
	req := &DecisionRequest{
		Input:     json.RawMessage(`[{"role":"user","content":[{"type":"input_text","text":"The package arrived damaged."}]}]`),
		Questions: json.RawMessage(`[{"type":"predicate","name":"damaged","instructions":"Is the item damaged?"}]`),
	}
	text := req.PromptText()
	require.Contains(t, text, "The package arrived damaged.")
	require.Contains(t, text, "Is the item damaged?")
	// Deterministic across calls.
	require.Equal(t, text, req.PromptText())

	// Empty containers and null carry no checkable content.
	require.Empty(t, (&DecisionRequest{Input: json.RawMessage(`{}`), Questions: json.RawMessage(`[]`)}).PromptText())
	require.Empty(t, (&DecisionRequest{}).PromptText())
}
