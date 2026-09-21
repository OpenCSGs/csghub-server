package handler

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/aigateway/types"
	commontypes "opencsg.com/csghub-server/common/types"
)

// The admission checker adapter must skip the text-based TPM estimate for
// multimodal requests: a text-only estimate is fiction for them, so they are
// admitted without a TPM reservation (types.AdmissionNoTPMEstimate).

func admissionAdapterTarget() *types.ModelTarget {
	return admissionAdapterTargetWithPolicy(&commontypes.CapacityPolicy{
		Enabled: true, MaxConcurrency: 10, MaxRPM: 100, MaxTPM: 100000,
	})
}

func admissionAdapterTargetWithPolicy(policy *commontypes.CapacityPolicy) *types.ModelTarget {
	return &types.ModelTarget{
		Model: &types.Model{
			BaseModel: types.BaseModel{ID: "test-model"},
			Upstreams: []commontypes.UpstreamConfig{{
				ID:             7,
				URL:            "https://upstream.example.com/v1",
				CapacityPolicy: policy,
			}},
		},
		Upstream: commontypes.UpstreamConfig{
			ID:  7,
			URL: "https://upstream.example.com/v1",
		},
	}
}

func chatRequestBody(t *testing.T, raw string) *types.ChatCompletionRequest {
	t.Helper()
	req := &types.ChatCompletionRequest{}
	require.NoError(t, json.Unmarshal([]byte(raw), req))
	return req
}

func TestAdmissionCheckerAdapter_Multimodal_SkipsTPMEstimate(t *testing.T) {
	tester, _, _ := setupTest(t)
	adapter := &admissionCheckerAdapter{handler: tester.handler}

	meta := &types.RequestMetadata{
		ParsedBody: chatRequestBody(t, `{
			"model": "test-model",
			"messages": [
				{"role": "user", "content": [
					{"type": "image_url", "image_url": {"url": "https://example.com/a.png"}}
				]}
			]
		}`),
	}
	mt := admissionAdapterTarget()

	// EstimateAdmissionTokens must NOT be called for multimodal bodies (no
	// EXPECT on it); admission runs without a TPM reservation.
	tester.mocks.openAIComp.EXPECT().CheckCapacityAdmission(
		mock.Anything, mt.Model, int64(7), true, types.AdmissionNoTPMEstimate,
	).Return(nil).Once()

	outcome, err := adapter.CheckAdmission(context.Background(), meta, mt)
	require.NoError(t, err)
	require.Nil(t, outcome)
}

func TestAdmissionCheckerAdapter_Text_UsesPromptEstimate(t *testing.T) {
	tester, _, _ := setupTest(t)
	adapter := &admissionCheckerAdapter{handler: tester.handler}

	meta := &types.RequestMetadata{
		ParsedBody: chatRequestBody(t, `{
			"model": "test-model",
			"messages": [
				{"role": "user", "content": "please summarize this"}
			]
		}`),
	}
	mt := admissionAdapterTarget()

	tester.mocks.openAIComp.EXPECT().EstimateAdmissionTokens("please summarize this").Return(int64(1064)).Once()
	tester.mocks.openAIComp.EXPECT().CheckCapacityAdmission(
		mock.Anything, mt.Model, int64(7), true, int64(1064),
	).Return(nil).Once()

	outcome, err := adapter.CheckAdmission(context.Background(), meta, mt)
	require.NoError(t, err)
	require.Nil(t, outcome)
}

func TestAdmissionCheckerAdapter_NoTPMPolicy_SkipsEstimate(t *testing.T) {
	// Policy-level shortcut: when the candidate set has no MaxTPM > 0 (the
	// documented "no TPM dimension" configuration), the text estimate is
	// never computed and the request is admitted without a TPM reservation
	// even for plain text.
	tester, _, _ := setupTest(t)
	adapter := &admissionCheckerAdapter{handler: tester.handler}

	meta := &types.RequestMetadata{
		ParsedBody: chatRequestBody(t, `{
			"model": "test-model",
			"messages": [
				{"role": "user", "content": "please summarize this"}
			]
		}`),
	}
	mt := admissionAdapterTargetWithPolicy(&commontypes.CapacityPolicy{
		Enabled: true, MaxConcurrency: 10, MaxRPM: 100,
	})

	// EstimateAdmissionTokens must NOT be called (no EXPECT on it).
	tester.mocks.openAIComp.EXPECT().CheckCapacityAdmission(
		mock.Anything, mt.Model, int64(7), true, types.AdmissionNoTPMEstimate,
	).Return(nil).Once()

	outcome, err := adapter.CheckAdmission(context.Background(), meta, mt)
	require.NoError(t, err)
	require.Nil(t, outcome)
}

// The adapter must see through the pipelines' PARSED BODY WRAPPERS
// (chatParsedBody / responsesParsedBody): asserting only the raw request
// types here would miss exactly the shape the real plan phase hands over.

func TestAdmissionCheckerAdapter_ChatWrapper_MultimodalSkipsTPMEstimate(t *testing.T) {
	tester, _, _ := setupTest(t)
	adapter := &admissionCheckerAdapter{handler: tester.handler}

	chatReq := chatRequestBody(t, `{
		"model": "test-model",
		"messages": [
			{"role": "user", "content": [
				{"type": "image_url", "image_url": {"url": "https://example.com/a.png"}}
			]}
		]
	}`)
	meta := &types.RequestMetadata{ParsedBody: &chatParsedBody{Req: chatReq}}
	mt := admissionAdapterTarget()

	// EstimateAdmissionTokens must NOT be called for multimodal bodies.
	tester.mocks.openAIComp.EXPECT().CheckCapacityAdmission(
		mock.Anything, mt.Model, int64(7), true, types.AdmissionNoTPMEstimate,
	).Return(nil).Once()

	outcome, err := adapter.CheckAdmission(context.Background(), meta, mt)
	require.NoError(t, err)
	require.Nil(t, outcome)
}

func TestAdmissionCheckerAdapter_ChatWrapper_TextUsesPromptEstimate(t *testing.T) {
	tester, _, _ := setupTest(t)
	adapter := &admissionCheckerAdapter{handler: tester.handler}

	chatReq := chatRequestBody(t, `{
		"model": "test-model",
		"messages": [{"role": "user", "content": "text via wrapper"}]
	}`)
	meta := &types.RequestMetadata{ParsedBody: &chatParsedBody{Req: chatReq}}
	mt := admissionAdapterTarget()

	tester.mocks.openAIComp.EXPECT().EstimateAdmissionTokens("text via wrapper").Return(int64(1064)).Once()
	tester.mocks.openAIComp.EXPECT().CheckCapacityAdmission(
		mock.Anything, mt.Model, int64(7), true, int64(1064),
	).Return(nil).Once()

	outcome, err := adapter.CheckAdmission(context.Background(), meta, mt)
	require.NoError(t, err)
	require.Nil(t, outcome)
}

func TestAdmissionCheckerAdapter_ResponsesWrapper_MultimodalSkipsTPMEstimate(t *testing.T) {
	tester, _, _ := setupTest(t)
	adapter := &admissionCheckerAdapter{handler: tester.handler}

	responsesReq := &types.ResponsesRequest{}
	require.NoError(t, json.Unmarshal([]byte(`{
		"model": "test-model",
		"input": [
			{"type": "message", "role": "user", "content": [
				{"type": "input_image", "image_url": "https://example.com/a.png"}
			]}
		]
	}`), responsesReq))
	meta := &types.RequestMetadata{ParsedBody: &responsesParsedBody{Req: responsesReq}}
	mt := admissionAdapterTarget()

	tester.mocks.openAIComp.EXPECT().CheckCapacityAdmission(
		mock.Anything, mt.Model, int64(7), true, types.AdmissionNoTPMEstimate,
	).Return(nil).Once()

	outcome, err := adapter.CheckAdmission(context.Background(), meta, mt)
	require.NoError(t, err)
	require.Nil(t, outcome)
}
