package ocr

import (
	"opencsg.com/csghub-server/aigateway/types"
)

// PaddleX fileType values for the upstream serving contract.
const (
	FileTypePDF   = 0
	FileTypeImage = 1
)

// UpstreamInput carries everything an adapter needs to build the upstream
// OCR runtime request body.
type UpstreamInput struct {
	FileBytes                 []byte
	FileType                  int
	UseDocOrientationClassify *bool
	UseDocUnwarping           *bool
	UseTextlineOrientation    *bool
	Visualize                 bool
}

// ResponseOptions controls how the upstream response is normalized.
type ResponseOptions struct {
	ModelID     string
	RawResponse bool
	ReturnImage bool
}

// Adapter transforms AIGateway OCR requests to an upstream OCR runtime
// protocol and normalizes its responses.
type Adapter interface {
	Name() string
	CanHandle(model *types.Model) bool
	// EndpointPath is the upstream OCR route path, default "/ocr".
	EndpointPath(model *types.Model) string
	BuildUpstreamRequest(in *UpstreamInput) ([]byte, error)
	TransformResponse(respBody []byte, opts *ResponseOptions) (*types.OCRResponse, error)
}

type Registry struct {
	adapters []Adapter
}

func NewRegistry() *Registry {
	return &Registry{
		adapters: []Adapter{
			NewPaddleXAdapter(),
		},
	}
}

func (r *Registry) GetAdapter(model *types.Model) Adapter {
	if model == nil {
		return nil
	}
	for _, adapter := range r.adapters {
		if adapter.CanHandle(model) {
			return adapter
		}
	}
	return nil
}
