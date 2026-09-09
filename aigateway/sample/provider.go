package sample

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"opencsg.com/csghub-server/aigateway/types"
)

type requestFactory func(types.SampleInput) (*types.SampleRequest, error)

type protocolProvider struct {
	route            string
	l7               requestFactory
	inference        requestFactory
	inferenceTimeout time.Duration
	multimodal       bool
}

func newProtocolProvider(route string, l7 requestFactory, inference requestFactory, inferenceTimeout time.Duration) protocolProvider {
	return protocolProvider{
		route:            route,
		l7:               l7,
		inference:        inference,
		inferenceTimeout: inferenceTimeout,
		multimodal:       isMultimodalRoute(route),
	}
}

func (p protocolProvider) Supports(endpoint string) bool {
	return endpointMatchesRoute(endpoint, p.route)
}

func (p protocolProvider) factory(kind types.SampleKind) (requestFactory, error) {
	switch kind {
	case types.SampleKindL7API:
		return p.l7, nil
	case types.SampleKindInference:
		return p.inference, nil
	default:
		return nil, fmt.Errorf("unsupported sample kind %q", kind)
	}
}

func (p protocolProvider) ExecutionPolicy(kind types.SampleKind) (types.SampleExecutionPolicy, error) {
	switch kind {
	case types.SampleKindL7API:
		return types.SampleExecutionPolicy{Timeout: defaultSampleTimeout}, nil
	case types.SampleKindInference:
		return types.SampleExecutionPolicy{Timeout: p.inferenceTimeout, Multimodal: p.multimodal}, nil
	default:
		return types.SampleExecutionPolicy{}, fmt.Errorf("unsupported sample kind %q", kind)
	}
}

func (p protocolProvider) Build(kind types.SampleKind, input types.SampleInput) (*types.SampleRequest, error) {
	factory, err := p.factory(kind)
	if err != nil {
		return nil, err
	}
	return factory(input)
}

func (p protocolProvider) Execute(ctx context.Context, kind types.SampleKind, input types.SampleInput, client types.HTTPDoer) (*types.SampleExecutionResult, error) {
	factory, err := p.factory(kind)
	if err != nil {
		return nil, err
	}
	result, err := executeSampleFactory(ctx, factory, input, client)
	if err == nil && kind == types.SampleKindL7API && modelsEndpointUnsupported(result) {
		result.InferenceFallbackRequired = true
	}
	return result, err
}

func isMultimodalRoute(route string) bool {
	switch route {
	case imageGenerationsRoute, imageEditsRoute, transcriptionsRoute, speechRoute, batchSpeechRoute, voiceUploadRoute, videoGenerationsRoute:
		return true
	default:
		return false
	}
}

func executeSampleFactory(ctx context.Context, factory requestFactory, input types.SampleInput, client types.HTTPDoer) (*types.SampleExecutionResult, error) {
	sampleRequest, err := factory(input)
	if err != nil {
		return nil, err
	}
	return executeSampleRequest(ctx, sampleRequest, input, client), nil
}

func executeSampleRequest(ctx context.Context, sampleRequest *types.SampleRequest, input types.SampleInput, client types.HTTPDoer) *types.SampleExecutionResult {
	result := &types.SampleExecutionResult{Request: sampleRequest}
	req, err := http.NewRequestWithContext(ctx, sampleRequest.Method, sampleRequest.Endpoint, bytes.NewReader(sampleRequest.Body))
	if err != nil {
		result.Error = err
		return result
	}
	req.Header = sampleRequest.Headers.Clone()

	start := time.Now()
	resp, err := client.Do(req)
	result.Latency = time.Since(start)
	if err != nil {
		result.Error = err
		return result
	}
	defer resp.Body.Close()

	result.StatusCode = resp.StatusCode
	result.Status = resp.Status
	reader := io.Reader(resp.Body)
	if input.MaxResponseBodyBytes > 0 {
		reader = io.LimitReader(resp.Body, input.MaxResponseBodyBytes)
	}
	result.ResponseBody, err = io.ReadAll(reader)
	if err != nil {
		result.Error = err
	}
	return result
}

func modelsEndpointUnsupported(result *types.SampleExecutionResult) bool {
	if result == nil || result.Request == nil || result.Error != nil || result.Request.Method != http.MethodGet {
		return false
	}
	unsupportedStatus := result.StatusCode == http.StatusNotFound || result.StatusCode == http.StatusMethodNotAllowed
	return unsupportedStatus && endpointMatchesRoute(result.Request.Endpoint, "/models")
}

func sampleText(input types.SampleInput) string {
	if input.Text != "" {
		return input.Text
	}
	return "hi"
}

func cloneSampleHeaders(headers http.Header) http.Header {
	cloned := headers.Clone()
	if cloned == nil {
		cloned = make(http.Header)
	}
	return cloned
}

var _ types.SampleProvider = protocolProvider{}
var _ types.SampleRequestBuilder = protocolProvider{}
