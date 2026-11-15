package decisions

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/aigateway/types"
)

func TestUpstreamRequestBodyPreservesInputWhileRewritingModel(t *testing.T) {
	raw := []byte(`{"model":"client-model","input":[{"role":"user","content":[{"type":"input_image","image_url":"data:image/png;base64,AAAA"}]}],"questions":[{"type":"predicate","name":"damage"}],"extension":{"value":1}}`)
	var req types.DecisionRequest
	require.NoError(t, json.Unmarshal(raw, &req))

	body, err := upstreamRequestBody(&req, "resolved-model")
	require.NoError(t, err)
	var got map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(body, &got))
	var model string
	require.NoError(t, json.Unmarshal(got["model"], &model))
	require.Equal(t, "resolved-model", model)
	require.JSONEq(t, string(req.Input), string(got["input"]))
	require.JSONEq(t, string(req.Questions), string(got["questions"]))
	require.JSONEq(t, `{"value":1}`, string(got["extension"]))
}

func TestExtractUsageIncludesDecisionUsageDetails(t *testing.T) {
	resp := &types.DecisionResponse{Usage: &types.DecisionUsage{
		InputTokens: 42, OutputTokens: 3, TotalTokens: 45,
		InputTokensDetails:  types.DecisionInputTokenDetails{CachedTokens: 7, CacheWriteTokens: 2},
		OutputTokensDetails: types.DecisionOutputTokenDetails{ReasoningTokens: 1},
		ComputeUnits:        int64Ptr(4),
	}}
	usage := extractUsage(resp)
	require.Equal(t, int64(42), usage.PromptTokens)
	require.Equal(t, int64(3), usage.CompletionTokens)
	require.Equal(t, int64(45), usage.TotalTokens)
	require.Equal(t, int64(7), usage.CachedPromptTokens)
	require.Equal(t, int64(2), usage.CacheCreationPromptTokens)
	require.Equal(t, int64(1), usage.ReasoningTokens)
	require.Equal(t, int64(4), usage.ComputeUnits)
}

func int64Ptr(value int64) *int64 { return &value }

func TestRecordDecisionTraceCompletionIncludesUsageAndResponseModel(t *testing.T) {
	usage := &tokenUsage{
		PromptTokens: 42, CompletionTokens: 3, TotalTokens: 45,
		CachedPromptTokens: 7, CacheCreationPromptTokens: 2, ReasoningTokens: 1,
		ComputeUnits: 4,
	}
	recorder := &capturedDecisionTrace{}
	recordDecisionTraceCompletion(postProcessInput{
		Model:           &types.Model{ExternalModelInfo: types.ExternalModelInfo{Provider: "OpenAI"}},
		TargetModelName: "requested-model",
		ResponseModel:   "resolved-model",
		Usage:           usage,
		Recorder:        recorder,
		StatusCode:      200,
	})
	require.Equal(t, types.TokenUsage{
		InputTokens: 42, OutputTokens: 3, TotalTokens: 45,
		CacheReadInputTokens: 7, CacheWriteInputTokens: 2, ReasoningTokens: 1,
	}, recorder.usage)
	require.Equal(t, "resolved-model", recorder.response.Model)
	require.Equal(t, int64(4), recorder.response.Metadata["aigateway.usage.compute_units"])
}

type capturedDecisionTrace struct {
	usage    types.TokenUsage
	response types.GenerationResponse
}

func (r *capturedDecisionTrace) SetUsage(usage types.TokenUsage) { r.usage = usage }
func (r *capturedDecisionTrace) SetResponse(response types.GenerationResponse) {
	r.response = response
}
func (*capturedDecisionTrace) SetFirstChunk(types.GenerationFirstChunk) {}
func (*capturedDecisionTrace) SetError(error, string)                   {}
func (*capturedDecisionTrace) End()                                     {}
