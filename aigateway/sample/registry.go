package sample

import (
	"opencsg.com/csghub-server/aigateway/types"
)

type Registry struct {
	providers []types.SampleProvider
}

func NewRegistry(providers ...types.SampleProvider) *Registry {
	return &Registry{
		providers: providers,
	}
}

func (r *Registry) Find(endpoint string) (types.SampleProvider, bool) {
	if r == nil {
		return nil, false
	}
	for _, provider := range r.providers {
		if provider.Supports(endpoint) {
			return provider, true
		}
	}
	return nil, false
}
