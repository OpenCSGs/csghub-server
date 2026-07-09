package audio

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/aigateway/types"
)

func TestRegistryGetAdapter(t *testing.T) {
	registry := NewRegistry()

	funasr := registry.GetAdapter(&types.Model{InternalModelInfo: types.InternalModelInfo{RuntimeFramework: "FunASR"}})
	require.Equal(t, "funasr", funasr.Name())

	opencsg := registry.GetAdapter(&types.Model{ExternalModelInfo: types.ExternalModelInfo{Provider: " OpenCSG "}})
	require.Equal(t, "funasr", opencsg.Name())

	openaiCompatible := registry.GetAdapter(&types.Model{})
	require.Equal(t, "openai-compatible", openaiCompatible.Name())
}

func TestFunASRAdapterDurationFromHeader(t *testing.T) {
	adapter := NewFunASRAdapter()

	tests := []struct {
		name string
		val  string
		want float64
		ok   bool
	}{
		{name: "valid", val: "9.2", want: 9.2, ok: true},
		{name: "invalid", val: "nope"},
		{name: "zero", val: "0"},
		{name: "negative", val: "-1"},
		{name: "missing"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			header := http.Header{}
			if tt.val != "" {
				header.Set(audioDurationHeader, tt.val)
			}
			got, ok := adapter.DurationFromHeader(header)
			require.Equal(t, tt.ok, ok)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestOpenAICompatibleAdapterIgnoresDurationHeader(t *testing.T) {
	header := http.Header{}
	header.Set(audioDurationHeader, "9.2")

	got, ok := NewOpenAICompatibleAdapter().DurationFromHeader(header)
	require.False(t, ok)
	require.Zero(t, got)
}
