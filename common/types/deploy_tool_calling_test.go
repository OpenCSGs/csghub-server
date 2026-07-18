package types

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEngineArgToolCallingEnabled(t *testing.T) {
	tests := []struct {
		name             string
		engineArgs       string
		runtimeFramework string
		want             bool
	}{
		{
			name:             "empty engine args",
			engineArgs:       "",
			runtimeFramework: "vllm",
			want:             false,
		},
		{
			name:             "vllm tool calling enabled",
			engineArgs:       `{"enable-tool-calling":"enable"}`,
			runtimeFramework: "vllm",
			want:             true,
		},
		{
			name:             "vllm tool calling disabled by default sentinel",
			engineArgs:       `{"enable-tool-calling":"disable"}`,
			runtimeFramework: "vllm",
			want:             false,
		},
		{
			name:             "nvidia vllm tool calling enabled",
			engineArgs:       `{"enable-tool-calling":"true"}`,
			runtimeFramework: "nvidia-vllm",
			want:             true,
		},
		{
			name:             "sglang tool calling disabled by default sentinel",
			engineArgs:       `{"enable-tool-calling":"enable"}`,
			runtimeFramework: "sglang",
			want:             false,
		},
		{
			name:             "sglang tool calling enabled with non-default value",
			engineArgs:       `{"enable-tool-calling":"true"}`,
			runtimeFramework: "sglang",
			want:             true,
		},
		{
			name:             "missing enable-tool-calling key",
			engineArgs:       `{"max-model-len":"8192"}`,
			runtimeFramework: "vllm",
			want:             false,
		},
		{
			name:             "tool calling enabled via custom-options",
			engineArgs:       `{"max-model-len":"160000","custom-options":"--enable-auto-tool-choice --tool-call-parser deepseek_v31"}`,
			runtimeFramework: "vllm",
			want:             true,
		},
		{
			name:             "custom-options without tool calling flags",
			engineArgs:       `{"max-model-len":"8192","custom-options":"--enforce-eager"}`,
			runtimeFramework: "vllm",
			want:             false,
		},
		{
			name:             "legacy cli vllm engine args",
			engineArgs:       "--max-model-len 8192 --enable-auto-tool-choice --tool-call-parser qwen",
			runtimeFramework: "vllm",
			want:             true,
		},
		{
			name:             "legacy cli sglang engine args",
			engineArgs:       "--context-length 8192 --tool-call-parser auto",
			runtimeFramework: "sglang",
			want:             true,
		},
		{
			name:             "legacy cli without tool calling flags",
			engineArgs:       "--max-model-len 8192",
			runtimeFramework: "vllm",
			want:             false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, EngineArgToolCallingEnabled(tt.engineArgs, tt.runtimeFramework))
		})
	}
}
