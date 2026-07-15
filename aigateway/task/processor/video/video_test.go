package video

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/aigateway/component/adapter/text2video"
	taskprocessor "opencsg.com/csghub-server/aigateway/task/processor"
	"opencsg.com/csghub-server/aigateway/token"
	aigwtypes "opencsg.com/csghub-server/aigateway/types"
	commontypes "opencsg.com/csghub-server/common/types"
)

type fakeOpenAIComponent struct {
	model *aigwtypes.Model
}

func (c *fakeOpenAIComponent) GetAvailableModels(ctx context.Context, user string) ([]aigwtypes.Model, error) {
	return nil, nil
}

func (c *fakeOpenAIComponent) ListModels(ctx context.Context, user string, req aigwtypes.ListModelsReq) (aigwtypes.ModelList, error) {
	return aigwtypes.ModelList{}, nil
}

func (c *fakeOpenAIComponent) GetModelByID(ctx context.Context, username, modelID string) (*aigwtypes.Model, error) {
	return c.model, nil
}

func (c *fakeOpenAIComponent) RecordUsage(ctx context.Context, nsUUID string, model *aigwtypes.Model, targetModelName string, tokenCounter token.Counter, apikey string) error {
	return nil
}

func (c *fakeOpenAIComponent) RecordUsageFromTokenUsage(ctx context.Context, nsUUID string, model *aigwtypes.Model, targetModelName string, usage *token.Usage, apikey string) error {
	return nil
}

func (c *fakeOpenAIComponent) BuildUsageMeteringEvent(ctx context.Context, nsUUID string, model *aigwtypes.Model, targetModelName string, usage *token.Usage, apikey string) (*commontypes.MeteringEvent, error) {
	return nil, nil
}

func (c *fakeOpenAIComponent) CheckBalance(ctx context.Context, nsUUID string) error {
	return nil
}

func (c *fakeOpenAIComponent) CheckUsageLimit(ctx context.Context, userUUID string, model *aigwtypes.Model, endpoint string) error {
	return nil
}

func (c *fakeOpenAIComponent) CanManageModel(ctx context.Context, username, nsUUID string, model *aigwtypes.Model) (bool, error) {
	return false, nil
}

func (c *fakeOpenAIComponent) CommitUsageLimit(ctx context.Context, userUUID string, model *aigwtypes.Model, tokenCounter token.Counter) error {
	return nil
}

type fakeHTTPDoer struct {
	t        *testing.T
	wantURL  string
	wantAuth string
	called   bool
}

func (d *fakeHTTPDoer) Do(req *http.Request) (*http.Response, error) {
	d.called = true
	require.Equal(d.t, http.MethodGet, req.Method)
	require.Equal(d.t, d.wantURL, req.URL.String())
	require.Equal(d.t, d.wantAuth, req.Header.Get("Authorization"))
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(`{"id":"provider-id","object":"video","status":"completed"}`)),
	}, nil
}

func TestVideoProcessorRefreshUsesHTTPDoer(t *testing.T) {
	endpoint := "https://upstream.example/v1/videos"
	doer := &fakeHTTPDoer{
		t:        t,
		wantURL:  "https://upstream.example/v1/videos/provider-id",
		wantAuth: "Bearer token",
	}
	processor := NewProcessor(
		&fakeOpenAIComponent{
			model: &aigwtypes.Model{
				BaseModel: aigwtypes.BaseModel{
					ID:   "video-model",
					Task: string(commontypes.Text2Video),
				},
				Endpoint: endpoint,
				Upstreams: []commontypes.UpstreamConfig{
					{ID: 7, URL: endpoint, AuthHeader: "Bearer token"},
				},
			},
		},
		nil,
		doer,
	)

	status, err := processor.Refresh(context.Background(), taskprocessor.GenerationRef{
		ResourceID:         "gateway-id",
		ProviderResourceID: "provider-id",
		ModelID:            "video-model",
		UpstreamID:         7,
	})

	require.NoError(t, err)
	require.True(t, doer.called)
	require.Equal(t, string(commontypes.AIGatewayAsyncGenerationStatusCompleted), status.Status)
	require.Equal(t, "completed", status.ProviderMetadata[text2video.ProviderStatusMetadataKey])
}
