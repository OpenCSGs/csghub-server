//go:build ee || saas

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
		mock.Anything, mock.MatchedBy(func(req types.CapacityAdmissionRequest) bool {
			return req.PreferredUpstreamID == 7 && req.AllowSelect == true && req.EstimatedTokens == types.AdmissionNoTPMEstimate
		}),
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
		mock.Anything, mock.MatchedBy(func(req types.CapacityAdmissionRequest) bool {
			return req.PreferredUpstreamID == 7 && req.AllowSelect == true && req.EstimatedTokens == int64(1064)
		}),
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
		mock.Anything, mock.MatchedBy(func(req types.CapacityAdmissionRequest) bool {
			return req.PreferredUpstreamID == 7 && req.AllowSelect == true && req.EstimatedTokens == types.AdmissionNoTPMEstimate
		}),
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
		mock.Anything, mock.MatchedBy(func(req types.CapacityAdmissionRequest) bool {
			return req.PreferredUpstreamID == 7 && req.AllowSelect == true && req.EstimatedTokens == types.AdmissionNoTPMEstimate
		}),
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
		mock.Anything, mock.MatchedBy(func(req types.CapacityAdmissionRequest) bool {
			return req.PreferredUpstreamID == 7 && req.AllowSelect == true && req.EstimatedTokens == int64(1064)
		}),
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
		mock.Anything, mock.MatchedBy(func(req types.CapacityAdmissionRequest) bool {
			return req.PreferredUpstreamID == 7 && req.AllowSelect == true && req.EstimatedTokens == types.AdmissionNoTPMEstimate
		}),
	).Return(nil).Once()

	outcome, err := adapter.CheckAdmission(context.Background(), meta, mt)
	require.NoError(t, err)
	require.Nil(t, outcome)
}

func TestAdmissionCheckerAdapter_TextEstimatableModalities_UsePromptEstimate(t *testing.T) {
	// embedding / rerank / speech carry plain-text input: the text-based
	// estimate is meaningful, so they reserve like text generation.
	tester, _, _ := setupTest(t)
	adapter := &admissionCheckerAdapter{handler: tester.handler}
	mt := admissionAdapterTarget()

	cases := map[string]types.PromptTextProvider{
		"embedding": &types.EmbeddingRequest{},
		"rerank":    &types.RerankRequest{Query: "what is go", Documents: []string{"go is a language"}},
		"speech":    &speechParsedBody{Req: &types.SpeechRequest{Input: "hello there"}},
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			meta := &types.RequestMetadata{ParsedBody: body}
			tester.mocks.openAIComp.EXPECT().EstimateAdmissionTokens(body.PromptText()).Return(int64(1064)).Once()
			tester.mocks.openAIComp.EXPECT().CheckCapacityAdmission(
				mock.Anything, mock.MatchedBy(func(req types.CapacityAdmissionRequest) bool {
					return req.PreferredUpstreamID == 7 && req.AllowSelect == true && req.EstimatedTokens == int64(1064)
				}),
			).Return(nil).Once()

			outcome, err := adapter.CheckAdmission(context.Background(), meta, mt)
			require.NoError(t, err)
			require.Nil(t, outcome)
		})
	}
}

func TestAdmissionCheckerAdapter_MediaModalities_SkipTPMEstimate(t *testing.T) {
	// audio / ocr / text-to-video carry media content: no text-based
	// estimate, admitted without a TPM reservation.
	tester, _, _ := setupTest(t)
	adapter := &admissionCheckerAdapter{handler: tester.handler}
	mt := admissionAdapterTarget()

	cases := map[string]types.MultimodalContentProvider{
		"audio": &audioParsedBody{},
		"ocr":   &ocrParsedBody{},
		"video": &createVideoInput{},
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			require.True(t, body.HasMultimodalContent(), "%s must report multimodal content", name)
			meta := &types.RequestMetadata{ParsedBody: body}
			// EstimateAdmissionTokens must NOT be called (no EXPECT on it).
			tester.mocks.openAIComp.EXPECT().CheckCapacityAdmission(
				mock.Anything, mock.MatchedBy(func(req types.CapacityAdmissionRequest) bool {
					return req.PreferredUpstreamID == 7 && req.AllowSelect == true && req.EstimatedTokens == types.AdmissionNoTPMEstimate
				}),
			).Return(nil).Once()

			outcome, err := adapter.CheckAdmission(context.Background(), meta, mt)
			require.NoError(t, err)
			require.Nil(t, outcome)
		})
	}
}

func TestSpeechParsedBody_PromptText(t *testing.T) {
	// Batch branch: valid items join with "\n"; malformed JSON items and
	// empty inputs are skipped (mirrors BatchSpeechRequest.InputTexts()).
	batch := &speechParsedBody{BatchReq: &types.BatchSpeechRequest{
		Items: []json.RawMessage{
			json.RawMessage(`{"input":"first item"}`),
			json.RawMessage(`{not valid json}`),  // skipped: unparseable
			json.RawMessage(`{"input":""}`),      // skipped: empty input
			json.RawMessage(`{"voice":"alloy"}`), // skipped: no input field
			json.RawMessage(`{"input":"second item"}`),
		},
	}}
	require.Equal(t, "first item\nsecond item", batch.PromptText())

	single := &speechParsedBody{Req: &types.SpeechRequest{Input: "plain text"}}
	require.Equal(t, "plain text", single.PromptText())

	var nilBody *speechParsedBody
	require.Equal(t, "", nilBody.PromptText())
}

func TestAdmissionCheckerAdapter_DefensiveDeny_CarriesLease(t *testing.T) {
	// When admission admits but the re-selected upstream has vanished from
	// the candidate set, the defensive deny MUST carry the acquired lease:
	// the planner stores outcome.Decision on the plan, so the Orchestrator
	// safety net releases that lease. Dropping it would orphan a
	// renewer-registered lease (renewed forever; slot leaked until
	// expired-lease cleanup).
	tester, _, _ := setupTest(t)
	adapter := &admissionCheckerAdapter{handler: tester.handler}

	mt := &types.ModelTarget{
		Model: &types.Model{
			BaseModel: types.BaseModel{ID: "test-model"},
			Upstreams: []commontypes.UpstreamConfig{{
				ID:             7,
				URL:            "https://upstream.example.com/v1",
				CapacityPolicy: &commontypes.CapacityPolicy{Enabled: true, MaxConcurrency: 10, MaxRPM: 100, MaxTPM: 100000},
			}},
		},
		// Candidate set WITHOUT the ghost upstream (admission's pick is
		// missing from it → rebuild lookup fails → defensive deny).
		Upstream: commontypes.UpstreamConfig{ID: 7, URL: "https://upstream.example.com/v1"},
	}
	acquired := &types.AdmissionDecision{
		Action:             types.AdmissionAdmit,
		SelectedUpstreamID: 999,
		ReSelected:         true,
		Lease:              &types.AdmissionLease{ModelID: "test-model", UpstreamID: 999, Token: "orphan-candidate"},
	}

	tester.mocks.openAIComp.EXPECT().CheckCapacityAdmission(
		mock.Anything, mock.MatchedBy(func(req types.CapacityAdmissionRequest) bool {
			return req.PreferredUpstreamID == 7 && req.AllowSelect == true && req.EstimatedTokens == int64(1064)
		}),
	).Return(acquired).Once()
	tester.mocks.openAIComp.EXPECT().EstimateAdmissionTokens(mock.Anything).Return(int64(1064)).Once()

	meta := &types.RequestMetadata{
		ParsedBody: chatRequestBody(t, `{
			"model": "test-model",
			"messages": [{"role": "user", "content": "text"}]
		}`),
	}

	outcome, err := adapter.CheckAdmission(context.Background(), meta, mt)
	require.NoError(t, err)
	require.NotNil(t, outcome)
	require.NotNil(t, outcome.Decision)
	require.Equal(t, types.AdmissionReject, outcome.Decision.Action)
	require.NotNil(t, outcome.Decision.Lease, "defensive deny must carry the acquired lease")
	require.Equal(t, "orphan-candidate", outcome.Decision.Lease.Token)
	require.Nil(t, outcome.ReSelectedTarget, "no rebuild target on deny")
}

func TestAdmissionCheckerAdapter_CarriesNSUUID(t *testing.T) {
	// The admission request carries the tenant namespace UUID for logging.
	tester, _, _ := setupTest(t)
	adapter := &admissionCheckerAdapter{handler: tester.handler}

	meta := &types.RequestMetadata{
		TenantID:   "ns-uuid-123",
		ParsedBody: chatRequestBody(t, `{"model":"test-model","messages":[{"role":"user","content":"hi"}]}`),
	}
	// mt.Upstream carries the policy too: without it the adapter's pinned
	// fast-path (session key falls back to the tenant ID) would skip
	// admission entirely before reaching the checker.
	mt := &types.ModelTarget{
		Model:    admissionAdapterTarget().Model,
		Upstream: admissionAdapterTarget().Model.Upstreams[0],
	}

	var got types.CapacityAdmissionRequest
	tester.mocks.openAIComp.EXPECT().EstimateAdmissionTokens("hi").Return(int64(1064)).Once()
	tester.mocks.openAIComp.EXPECT().
		CheckCapacityAdmission(mock.Anything, mock.Anything).
		Run(func(_ context.Context, req types.CapacityAdmissionRequest) {
			got = req
		}).
		Return(nil).Once()

	outcome, err := adapter.CheckAdmission(context.Background(), meta, mt)
	require.NoError(t, err)
	require.Nil(t, outcome)
	t.Logf("DEBUG got: %+v", got)
	require.Same(t, tester.mocks.openAIComp, adapter.handler.openaiComponent, "the adapter must use the mocked component")
	require.Equal(t, "ns-uuid-123", got.NSUUID, "the tenant UUID must reach the admission request")
	require.Equal(t, int64(7), got.PreferredUpstreamID)
}
