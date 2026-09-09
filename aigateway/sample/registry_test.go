package sample

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRegistryFind(t *testing.T) {
	provider := newProtocolProvider("/custom", modelsL7Request("/custom"), responsesRequest, defaultSampleTimeout)
	registry := NewRegistry(provider)
	found, ok := registry.Find("https://api.example.com/v1/custom")
	require.True(t, ok)
	require.NotNil(t, found)
	_, ok = registry.Find("https://api.example.com/v1/other")
	require.False(t, ok)
}

func TestNilRegistryFind(t *testing.T) {
	var registry *Registry
	_, ok := registry.Find("https://api.example.com/v1/custom")
	require.False(t, ok)
}
