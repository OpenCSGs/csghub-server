package audio

import (
	"net/http"

	"opencsg.com/csghub-server/aigateway/types"
)

type Adapter interface {
	Name() string
	CanHandle(model *types.Model) bool
	DurationFromHeader(header http.Header) (float64, bool)
}

type Registry struct {
	adapters []Adapter
}

func NewRegistry() *Registry {
	return &Registry{
		adapters: []Adapter{
			NewFunASRAdapter(),
			NewOpenAICompatibleAdapter(),
		},
	}
}

func (r *Registry) GetAdapter(model *types.Model) Adapter {
	if r == nil {
		return nil
	}
	for _, adapter := range r.adapters {
		if adapter.CanHandle(model) {
			return adapter
		}
	}
	return nil
}
