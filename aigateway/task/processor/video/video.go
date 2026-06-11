package video

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	aigatewaycomp "opencsg.com/csghub-server/aigateway/component"
	"opencsg.com/csghub-server/aigateway/component/adapter/text2video"
	taskprocessor "opencsg.com/csghub-server/aigateway/task/processor"
	aigwtypes "opencsg.com/csghub-server/aigateway/types"
	"opencsg.com/csghub-server/builder/rpc"
	"opencsg.com/csghub-server/builder/store/database"
	commontypes "opencsg.com/csghub-server/common/types"
)

type videoProcessor struct {
	openaiComponent aigatewaycomp.OpenAIComponent
	t2vRegistry     *text2video.Registry
	httpClient      rpc.HttpDoer
}

var _ taskprocessor.ResourceProcessor = (*videoProcessor)(nil)

func NewProcessor(openaiComponent aigatewaycomp.OpenAIComponent, t2vRegistry *text2video.Registry, httpClient rpc.HttpDoer) taskprocessor.ResourceProcessor {
	if t2vRegistry == nil {
		t2vRegistry = text2video.NewRegistry()
	}
	if httpClient == nil {
		client := rpc.NewHttpClient("")
		client.SetTimeout(30 * time.Second)
		httpClient = client
	}
	return &videoProcessor{
		openaiComponent: openaiComponent,
		t2vRegistry:     t2vRegistry,
		httpClient:      httpClient,
	}
}

func (p *videoProcessor) ResourceType() string {
	return database.AIGenerationResourceTypeVideo
}

func (p *videoProcessor) Refresh(ctx context.Context, ref taskprocessor.GenerationRef) (*taskprocessor.GenerationStatus, error) {
	model, err := p.resolveModel(ctx, ref)
	if err != nil {
		return nil, err
	}
	adapter := p.t2vRegistry.GetAdapter(model)
	if adapter == nil {
		return nil, fmt.Errorf("no video adapter for model %q", ref.ModelID)
	}

	providerResourceID := ref.ProviderResourceID
	if providerResourceID == "" {
		providerResourceID = ref.ResourceID
	}
	providerReq, err := adapter.BuildRetrieveRequest(ctx, model, providerResourceID, ref.ProviderMetadata)
	if err != nil {
		return nil, err
	}
	body, err := p.fetchProviderResponse(ctx, model, providerReq)
	if err != nil {
		return nil, err
	}
	providerResp, err := adapter.ParseRetrieveResponse(ctx, body)
	if err != nil {
		return nil, err
	}
	return providerResponseToGenerationStatus(ref, providerResp), nil
}

func (p *videoProcessor) resolveModel(ctx context.Context, ref taskprocessor.GenerationRef) (*aigwtypes.Model, error) {
	if p.openaiComponent == nil {
		return nil, fmt.Errorf("aigateway async generation openai component is not configured")
	}
	model, err := p.openaiComponent.GetModelByID(ctx, "", ref.ModelID)
	if err != nil {
		return nil, err
	}
	if model == nil {
		return nil, fmt.Errorf("model %q not found for async generation", ref.ModelID)
	}

	modelCopy := *model
	if ref.UpstreamID > 0 {
		for _, upstream := range model.Upstreams {
			if upstream.ID != ref.UpstreamID {
				continue
			}
			if upstream.URL != "" {
				modelCopy.Endpoint = upstream.URL
			}
			if upstream.AuthHeader != "" {
				modelCopy.AuthHead = upstream.AuthHeader
			}
			if upstream.Provider != "" {
				modelCopy.Provider = upstream.Provider
			}
			break
		}
	}
	return &modelCopy, nil
}

func (p *videoProcessor) fetchProviderResponse(ctx context.Context, model *aigwtypes.Model, providerReq *text2video.ProviderRequest) ([]byte, error) {
	targetURL, err := providerRequestURL(model.Endpoint, providerReq)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, providerReq.Method, targetURL, bytes.NewReader(providerReq.Body))
	if err != nil {
		return nil, err
	}
	if providerReq.ContentType != "" {
		req.Header.Set("Content-Type", providerReq.ContentType)
	}
	req.Header.Set("Accept-Encoding", "identity")
	if err := aigwtypes.ApplyRequestAuthHeaders(req.Header, model.AuthHead); err != nil {
		slog.WarnContext(ctx, "invalid async generation auth head", slog.Any("error", err), slog.String("model", model.ID))
	}
	if p.httpClient == nil {
		return nil, fmt.Errorf("async generation http client is not configured")
	}
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return nil, readErr
	}
	if resp.StatusCode >= http.StatusBadRequest {
		return nil, fmt.Errorf("async generation status upstream returned %d: %s", resp.StatusCode, truncateErrorBody(body, 200))
	}
	return body, nil
}

func truncateErrorBody(body []byte, limit int) string {
	message := strings.TrimSpace(string(body))
	if limit <= 0 || len(message) <= limit {
		return message
	}
	return message[:limit] + "..."
}

func providerRequestURL(endpoint string, providerReq *text2video.ProviderRequest) (string, error) {
	if endpoint == "" {
		return "", fmt.Errorf("empty async generation endpoint")
	}
	baseURL, err := url.Parse(endpoint)
	if err != nil {
		return "", err
	}
	if providerReq.Path != "" {
		if absolute, parseErr := url.Parse(providerReq.Path); parseErr == nil && absolute.IsAbs() {
			baseURL = absolute
		} else {
			baseURL.Path = providerReq.Path
		}
	}
	query := baseURL.Query()
	for key, values := range providerReq.Query {
		for _, value := range values {
			query.Add(key, value)
		}
	}
	baseURL.RawQuery = query.Encode()
	return baseURL.String(), nil
}

func providerResponseToGenerationStatus(ref taskprocessor.GenerationRef, resp *text2video.ProviderResponse) *taskprocessor.GenerationStatus {
	status := &taskprocessor.GenerationStatus{
		ProviderMetadata: providerRespMetadata(resp),
	}
	if resp == nil || resp.Video == nil {
		return status
	}
	video := resp.Video
	now := time.Now()
	status.Status = video.Status
	if video.Progress != nil {
		status.Progress = strconv.FormatFloat(*video.Progress, 'f', -1, 64)
	}
	if video.Error != nil {
		status.FailReason = strings.TrimSpace(video.Error.Message)
	}
	if strings.EqualFold(strings.TrimSpace(video.Status), string(commontypes.AIGatewayAsyncGenerationStatusInProgress)) && ref.StartedAt == nil {
		status.StartedAt = &now
	}
	if isTerminalVideoStatus(video.Status) && ref.FinishedAt == nil {
		status.FinishedAt = &now
	}
	return status
}

func isTerminalVideoStatus(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case string(commontypes.AIGatewayAsyncGenerationStatusCompleted),
		string(commontypes.AIGatewayAsyncGenerationStatusFailed),
		string(commontypes.AIGatewayAsyncGenerationStatusCancelled):
		return true
	default:
		return false
	}
}

func providerRespMetadata(resp *text2video.ProviderResponse) map[string]any {
	if resp == nil {
		return nil
	}
	return resp.ProviderMetadata
}
