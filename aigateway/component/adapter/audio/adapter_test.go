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

	amdFunasr := registry.GetAdapter(&types.Model{InternalModelInfo: types.InternalModelInfo{RuntimeFramework: frameworkAMDFunASR}})
	require.Equal(t, "funasr", amdFunasr.Name())

	normalizedAMDFunasr := registry.GetAdapter(&types.Model{InternalModelInfo: types.InternalModelInfo{RuntimeFramework: " AMD-FunASR "}})
	require.Equal(t, "funasr", normalizedAMDFunasr.Name())

	opencsg := registry.GetAdapter(&types.Model{ExternalModelInfo: types.ExternalModelInfo{Provider: " OpenCSG "}})
	require.Equal(t, "funasr", opencsg.Name())

	openaiCompatible := registry.GetAdapter(&types.Model{})
	require.Equal(t, "openai-compatible", openaiCompatible.Name())
}

func TestFunASRAdapter_CanHandle(t *testing.T) {
	adapter := NewFunASRAdapter()

	tests := []struct {
		name  string
		model *types.Model
		want  bool
	}{
		{
			name:  "funasr runtime framework",
			model: &types.Model{InternalModelInfo: types.InternalModelInfo{RuntimeFramework: frameworkFunASR}},
			want:  true,
		},
		{
			name:  "amd-funasr runtime framework",
			model: &types.Model{InternalModelInfo: types.InternalModelInfo{RuntimeFramework: frameworkAMDFunASR}},
			want:  true,
		},
		{
			name:  "runtime framework matching is normalized",
			model: &types.Model{InternalModelInfo: types.InternalModelInfo{RuntimeFramework: " AMD-FunASR "}},
			want:  true,
		},
		{
			name:  "opencsg provider",
			model: &types.Model{ExternalModelInfo: types.ExternalModelInfo{Provider: "opencsg"}},
			want:  true,
		},
		{
			name:  "reject nil model",
			model: nil,
			want:  false,
		},
		{
			name:  "reject unknown runtime framework",
			model: &types.Model{InternalModelInfo: types.InternalModelInfo{RuntimeFramework: "whisper"}},
			want:  false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, adapter.CanHandle(tt.model))
		})
	}
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

func TestOpenAICompatibleAdapterDurationFromHeader(t *testing.T) {
	adapter := NewOpenAICompatibleAdapter()

	tests := []struct {
		name string
		val  string
		want float64
		ok   bool
	}{
		{name: "valid", val: "9.2", want: 9.2, ok: true},
		{name: "invalid", val: "nope"},
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
