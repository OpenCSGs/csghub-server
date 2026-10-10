package activity

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/log"

	mockdb "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
)

// nopLogger discards log output for pure-function composition tests.
func nopLogger() log.Logger {
	return log.NewStructuredLogger(slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func composeForTest(t *testing.T, engineName, version string, argValues map[string]string, arch string) (string, specDecodeState) {
	t.Helper()
	return composeSpeculativeArgs(nopLogger(), speculativeComposeInput{
		EngineArgsStr: "--max-model-len 8192",
		ArgValues:     argValues,
		Runtime:       runtimeConfig{EngineName: engineName, EngineVersion: version},
		ArchLookup:    func() string { return arch },
	})
}

func TestComposeSpeculativeArgs(t *testing.T) {
	tests := []struct {
		name              string
		engineName        string
		version           string
		argValues         map[string]string
		arch              string
		wantContains      string
		wantNotInject     bool
		wantActive        bool
		wantAsyncConflict bool
	}{
		{
			name:         "vllm NGC 25.11 mtp default tokens",
			engineName:   "nvidia-vllm",
			version:      "25.11-py3",
			argValues:    map[string]string{"spec-decode-method": "mtp"},
			wantContains: `--speculative-config {"method":"mtp","num_speculative_tokens":2}`,
			wantActive:   true,
			// NGC 25.11 = vLLM 0.11.0, still inside the async-scheduling
			// conflict window.
			wantAsyncConflict: true,
		},
		{
			name:              "vllm upstream 0.10.2 uses deepseek_mtp alias and conflicts with async scheduling",
			engineName:        "vllm",
			version:           "v0.10.2-cu128",
			argValues:         map[string]string{"spec-decode-method": "mtp"},
			arch:              "DeepseekV3ForCausalLM",
			wantContains:      `--speculative-config {"method":"deepseek_mtp","num_speculative_tokens":2}`,
			wantActive:        true,
			wantAsyncConflict: true,
		},
		{
			name:              "vllm NGC 25.10 maps to 0.10.x behavior",
			engineName:        "nvidia-vllm",
			version:           "25.10-py3",
			argValues:         map[string]string{"spec-decode-method": "mtp"},
			arch:              "DeepseekV3ForCausalLM",
			wantContains:      `--speculative-config {"method":"deepseek_mtp","num_speculative_tokens":2}`,
			wantActive:        true,
			wantAsyncConflict: true,
		},
		{
			name:         "vllm 0.28.0 has no async scheduling conflict",
			engineName:   "vllm",
			version:      "v0.28.0",
			argValues:    map[string]string{"spec-decode-method": "mtp"},
			wantContains: `--speculative-config {"method":"mtp","num_speculative_tokens":2}`,
			wantActive:   true,
		},
		{
			name:         "vllm NGC 25.12 has no async scheduling conflict",
			engineName:   "nvidia-vllm",
			version:      "25.12-py3",
			argValues:    map[string]string{"spec-decode-method": "mtp"},
			wantContains: `{"method":"mtp","num_speculative_tokens":2}`,
			wantActive:   true,
		},
		{
			name:         "vllm mtp with explicit token count",
			engineName:   "nvidia-vllm",
			version:      "25.11-py3",
			argValues:    map[string]string{"spec-decode-method": "mtp", "spec-num-tokens": "1"},
			wantContains: `--speculative-config {"method":"mtp","num_speculative_tokens":1}`,
			wantActive:   true,
			// NGC 25.11 = vLLM 0.11.0, still inside the async-scheduling
			// conflict window.
			wantAsyncConflict: true,
		},
		{
			name:          "vllm pre 0.10 is skipped",
			engineName:    "vllm",
			version:       "v0.8.2",
			argValues:     map[string]string{"spec-decode-method": "mtp"},
			wantNotInject: true,
		},
		{
			name:          "vllm unparsable custom version fails closed",
			engineName:    "vllm",
			version:       "vllm-local:3.0",
			argValues:     map[string]string{"spec-decode-method": "mtp"},
			wantNotInject: true,
		},
		{
			name:         "vllm eagle3 with draft model and whitelisted arch",
			engineName:   "nvidia-vllm",
			version:      "25.11-py3",
			argValues:    map[string]string{"spec-decode-method": "eagle3", "spec-draft-model": "opencsg/eagle3-llama", "spec-num-tokens": "3"},
			arch:         "LlamaForCausalLM",
			wantContains: `--speculative-config {"method":"eagle3","model":"/workspace/opencsg/eagle3-llama","num_speculative_tokens":3}`,
			wantActive:   true,
			// NGC 25.11 = vLLM 0.11.0, still inside the async-scheduling
			// conflict window.
			wantAsyncConflict: true,
		},
		{
			name:          "vllm eagle3 without draft model is skipped",
			engineName:    "nvidia-vllm",
			version:       "25.11-py3",
			argValues:     map[string]string{"spec-decode-method": "eagle3"},
			arch:          "LlamaForCausalLM",
			wantNotInject: true,
		},
		{
			name:          "vllm eagle3 non-whitelisted target arch is skipped",
			engineName:    "nvidia-vllm",
			version:       "25.11-py3",
			argValues:     map[string]string{"spec-decode-method": "eagle3", "spec-draft-model": "opencsg/eagle3-x"},
			arch:          "MiniMaxM2ForCausalLM",
			wantNotInject: true,
		},
		{
			// eagle3 landed in v0.8.5; older releases rewrite the method and
			// fail at startup.
			name:          "vllm 0.8.4 eagle3 is skipped",
			engineName:    "vllm",
			version:       "v0.8.4",
			argValues:     map[string]string{"spec-decode-method": "eagle3", "spec-draft-model": "opencsg/eagle3-llama"},
			arch:          "LlamaForCausalLM",
			wantNotInject: true,
		},
		{
			// 0.9.x enforces a llama-only eagle3 target whitelist.
			name:          "vllm 0.9.0 eagle3 qwen target is skipped",
			engineName:    "vllm",
			version:       "v0.9.0",
			argValues:     map[string]string{"spec-decode-method": "eagle3", "spec-draft-model": "opencsg/eagle3-qwen"},
			arch:          "Qwen2ForCausalLM",
			wantNotInject: true,
		},
		{
			// 0.10.1+ widens the eagle3 target whitelist to qwen.
			name:         "vllm 0.10.1 eagle3 qwen target is composed",
			engineName:   "vllm",
			version:      "v0.10.1",
			argValues:    map[string]string{"spec-decode-method": "eagle3", "spec-draft-model": "opencsg/eagle3-qwen", "spec-num-tokens": "3"},
			arch:         "Qwen2ForCausalLM",
			wantContains: `--speculative-config {"method":"eagle3","model":"/workspace/opencsg/eagle3-qwen","num_speculative_tokens":3}`,
			wantActive:   true,
			// 0.10.1 is inside the async-scheduling conflict window.
			wantAsyncConflict: true,
		},
		{
			// NGC 25.09 ships upstream 0.10.1, so minicpm targets are still
			// rejected there.
			name:          "vllm NGC 25.09 eagle3 minicpm target is skipped",
			engineName:    "nvidia-vllm",
			version:       "25.09-py3",
			argValues:     map[string]string{"spec-decode-method": "eagle3", "spec-draft-model": "opencsg/eagle3-minicpm"},
			arch:          "MiniCPMForCausalLM",
			wantNotInject: true,
		},
		{
			name:         "vllm-ascend mtp is composed",
			engineName:   "ascend-vllm",
			version:      "25.11-py3",
			argValues:    map[string]string{"spec-decode-method": "mtp"},
			wantContains: `--speculative-config {"method":"mtp","num_speculative_tokens":2}`,
			wantActive:   true,
			// NGC 25.11 = vLLM 0.11.0, still inside the async-scheduling
			// conflict window.
			wantAsyncConflict: true,
		},
		{
			name:          "vllm eagle3 draft model with shell metacharacters is rejected",
			engineName:    "nvidia-vllm",
			version:       "25.11-py3",
			argValues:     map[string]string{"spec-decode-method": "eagle3", "spec-draft-model": "opencsg/$(id)"},
			arch:          "LlamaForCausalLM",
			wantNotInject: true,
		},
		{
			name:          "vllm eagle3 draft model with spaces is rejected",
			engineName:    "nvidia-vllm",
			version:       "25.11-py3",
			argValues:     map[string]string{"spec-decode-method": "eagle3", "spec-draft-model": "a b"},
			arch:          "LlamaForCausalLM",
			wantNotInject: true,
		},
		{
			name:         "vllm ngram renders prompt lookup bounds",
			engineName:   "nvidia-vllm",
			version:      "25.11-py3",
			argValues:    map[string]string{"spec-decode-method": "ngram", "spec-num-tokens": "4"},
			wantContains: `--speculative-config {"method":"ngram","num_speculative_tokens":4,"prompt_lookup_max":4,"prompt_lookup_min":2}`,
			wantActive:   true,
			// NGC 25.11 = vLLM 0.11.0, still inside the async-scheduling
			// conflict window.
			wantAsyncConflict: true,
		},
		{
			name:         "sglang NGC 25.10 mtp renders NEXTN with upstream defaults",
			engineName:   "nvidia-sglang",
			version:      "25.10-py3",
			argValues:    map[string]string{"spec-decode-method": "mtp"},
			wantContains: "--speculative-algorithm NEXTN --speculative-num-steps 3 --speculative-eagle-topk 1 --speculative-num-draft-tokens 4",
			wantActive:   true,
		},
		{
			name:         "sglang mtp derives steps from draft tokens",
			engineName:   "sglang",
			version:      "v0.4.6.post1",
			argValues:    map[string]string{"spec-decode-method": "mtp", "spec-num-tokens": "3"},
			wantContains: "--speculative-algorithm NEXTN --speculative-num-steps 2 --speculative-eagle-topk 1 --speculative-num-draft-tokens 3",
			wantActive:   true,
		},
		{
			name:         "sglang eagle3 requires draft path",
			engineName:   "nvidia-sglang",
			version:      "25.10-py3",
			argValues:    map[string]string{"spec-decode-method": "eagle3", "spec-draft-model": "opencsg/eagle3-qwen", "spec-num-tokens": "4"},
			arch:         "Qwen2ForCausalLM",
			wantContains: "--speculative-algorithm EAGLE3 --speculative-draft-model-path /workspace/opencsg/eagle3-qwen --speculative-num-steps 3 --speculative-eagle-topk 1 --speculative-num-draft-tokens 4",
			wantActive:   true,
		},
		{
			name:         "sglang ngram works on 0.5.14",
			engineName:   "sglang",
			version:      "v0.5.14-cu130",
			argValues:    map[string]string{"spec-decode-method": "ngram"},
			wantContains: "--speculative-algorithm NGRAM --speculative-num-draft-tokens 4",
			wantActive:   true,
		},
		{
			name:          "sglang NGC 25.10 rejects ngram at render time",
			engineName:    "nvidia-sglang",
			version:       "25.10-py3",
			argValues:     map[string]string{"spec-decode-method": "ngram"},
			wantNotInject: true,
		},
		{
			name:         "sglang NGC 25.11 ngram works (ships 0.5.4.post1)",
			engineName:   "nvidia-sglang",
			version:      "25.11-py3",
			argValues:    map[string]string{"spec-decode-method": "ngram"},
			wantContains: "--speculative-algorithm NGRAM --speculative-num-draft-tokens 4",
			wantActive:   true,
		},
		{
			name:          "sglang single draft token is invalid",
			engineName:    "nvidia-sglang",
			version:       "25.10-py3",
			argValues:     map[string]string{"spec-decode-method": "mtp", "spec-num-tokens": "1"},
			wantNotInject: true,
		},
		{
			name:          "non numeric token count is rejected",
			engineName:    "nvidia-vllm",
			version:       "25.11-py3",
			argValues:     map[string]string{"spec-decode-method": "mtp", "spec-num-tokens": "abc"},
			wantNotInject: true,
		},
		{
			name:          "spec num tokens above max is rejected",
			engineName:    "nvidia-vllm",
			version:       "25.11-py3",
			argValues:     map[string]string{"spec-decode-method": "mtp", "spec-num-tokens": "9"},
			wantNotInject: true,
		},
		{
			name:              "spec num tokens at max boundary is accepted",
			engineName:        "nvidia-vllm",
			version:           "25.11-py3",
			argValues:         map[string]string{"spec-decode-method": "mtp", "spec-num-tokens": "8"},
			wantContains:      `"num_speculative_tokens":8`,
			wantActive:        true,
			wantAsyncConflict: true,
		},
		{
			name:          "unsupported engine is skipped",
			engineName:    "mindie",
			version:       "1.8-csg-1.0.RC2",
			argValues:     map[string]string{"spec-decode-method": "mtp"},
			wantNotInject: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args, state := composeForTest(t, tt.engineName, tt.version, tt.argValues, tt.arch)
			if tt.wantNotInject {
				require.Equal(t, "--max-model-len 8192", args)
				require.False(t, state.Active)
				require.False(t, state.AsyncConflict)
				return
			}
			require.Contains(t, args, tt.wantContains)
			require.Equal(t, tt.wantActive, state.Active)
			require.Equal(t, tt.wantAsyncConflict, state.AsyncConflict)
		})
	}
}

func TestComposeSpeculativeArgs_NoMethodKeepsArgsUntouched(t *testing.T) {
	for _, values := range []map[string]string{
		{"spec-decode-method": "off"},
		{"spec-decode-method": ""},
		{"spec-decode-method": "eagle"}, // unknown method
	} {
		args, state := composeForTest(t, "nvidia-vllm", "25.11-py3", values, "")
		require.Equal(t, "--max-model-len 8192", args)
		require.False(t, state.Active)
	}
}

func TestParseEngineVersion(t *testing.T) {
	tests := []struct {
		version string
		want    engineVersion
	}{
		{version: "25.11-py3", want: engineVersion{ok: true, isNGC: true, major: 25, minor: 11}},
		{version: "26.1-py3", want: engineVersion{ok: true, isNGC: true, major: 26, minor: 1}},
		{version: "v0.10.2-cu128", want: engineVersion{ok: true, major: 0, minor: 10, patch: 2}},
		{version: "0.5.4.post1", want: engineVersion{ok: true, major: 0, minor: 5, patch: 4}},
		{version: "v0.25.1rc", want: engineVersion{ok: true, major: 0, minor: 25, patch: 1}},
		{version: "25.10", want: engineVersion{ok: true, isNGC: true, major: 25, minor: 10}},
		{version: "", want: engineVersion{}},
		{version: "vllm-local:3.0", want: engineVersion{}},
		{version: "1.8-csg-1.0.RC2", want: engineVersion{ok: true, major: 1, minor: 8}},
	}
	for _, tt := range tests {
		require.Equal(t, tt.want, parseEngineVersion(tt.version), "version %q", tt.version)
	}
}

func TestEngineVersionGates(t *testing.T) {
	ngc := func(y, m int) engineVersion { return engineVersion{ok: true, isNGC: true, major: y, minor: m} }
	sem := func(maj, min, pat int) engineVersion {
		return engineVersion{ok: true, major: maj, minor: min, patch: pat}
	}

	require.True(t, vllmSupportsSpeculativeJSON(ngc(25, 9)))
	require.True(t, vllmSupportsSpeculativeJSON(sem(0, 10, 0)))
	require.False(t, vllmSupportsSpeculativeJSON(sem(0, 9, 2)))
	require.False(t, vllmSupportsSpeculativeJSON(sem(0, 8, 3)))
	require.False(t, vllmSupportsSpeculativeJSON(sem(0, 8, 2)))
	require.False(t, vllmSupportsSpeculativeJSON(engineVersion{}))

	require.False(t, vllmSupportsMTPAlias(ngc(25, 10)))
	require.True(t, vllmSupportsMTPAlias(ngc(25, 11)))
	require.False(t, vllmSupportsMTPAlias(sem(0, 10, 2)))
	require.True(t, vllmSupportsMTPAlias(sem(0, 11, 0)))

	require.True(t, vllmAsyncSchedulingConflict(ngc(25, 9)))
	require.True(t, vllmAsyncSchedulingConflict(ngc(25, 11)))
	require.False(t, vllmAsyncSchedulingConflict(ngc(25, 12)))
	require.True(t, vllmAsyncSchedulingConflict(sem(0, 10, 0)))
	require.True(t, vllmAsyncSchedulingConflict(sem(0, 11, 9)))
	require.False(t, vllmAsyncSchedulingConflict(sem(0, 12, 0)))
	require.False(t, vllmAsyncSchedulingConflict(sem(0, 9, 2)))

	require.True(t, sglangSupportsSpeculative(ngc(25, 10)))
	require.True(t, sglangSupportsSpeculative(sem(0, 4, 6)))

	require.False(t, sglangSupportsNgram(ngc(25, 10)))
	require.True(t, sglangSupportsNgram(ngc(25, 11)))
	require.True(t, sglangSupportsNgram(ngc(26, 1)))
	require.True(t, sglangSupportsNgram(sem(0, 5, 4)))
	require.False(t, sglangSupportsNgram(sem(0, 5, 3)))

	// eagle3 exists since 0.8.5; the target whitelist shipped with it and
	// only grew over time.
	require.False(t, vllmSupportsEagle3(sem(0, 8, 4)))
	require.True(t, vllmSupportsEagle3(sem(0, 8, 5)))
	require.True(t, vllmSupportsEagle3(ngc(25, 9)))
	require.False(t, vllmSupportsEagle3(engineVersion{}))

	require.Equal(t, vllmEagle3TargetsLlama, vllmEagle3TargetFragments(sem(0, 9, 0)))
	require.Equal(t, vllmEagle3TargetsLlama, vllmEagle3TargetFragments(sem(0, 10, 0)))
	require.Equal(t, vllmEagle3TargetsLlamaQwen, vllmEagle3TargetFragments(sem(0, 10, 1)))
	require.Equal(t, vllmEagle3TargetsLlamaQwen, vllmEagle3TargetFragments(ngc(25, 9)))
	require.Equal(t, vllmEagle3TargetsLlamaQwen, vllmEagle3TargetFragments(ngc(25, 10)))
	require.Equal(t, vllmEagle3TargetsLlamaQwenMinicpmGptOss, vllmEagle3TargetFragments(sem(0, 11, 0)))
	require.Equal(t, vllmEagle3TargetsLlamaQwenMinicpmGptOss, vllmEagle3TargetFragments(ngc(25, 11)))
	require.True(t, vllmEagle3ArchSupported("llamaforcausallm", sem(0, 9, 0)))
	require.False(t, vllmEagle3ArchSupported("qwen3forcausallm", sem(0, 9, 0)))
	require.True(t, vllmEagle3ArchSupported("qwen3forcausallm", sem(0, 10, 1)))
	require.False(t, vllmEagle3ArchSupported("minimaxm2forcausallm", ngc(25, 11)))
	require.True(t, vllmEagle3ArchSupported("gptossforcausallm", ngc(25, 11)))
}

func TestEngineFamily(t *testing.T) {
	require.Equal(t, "vllm", engineFamily("nvidia-vllm"))
	require.Equal(t, "vllm", engineFamily("VLLM"))
	require.Equal(t, "vllm", engineFamily("ascend-vllm"))
	require.Equal(t, "sglang", engineFamily("nvidia-sglang"))
	require.Equal(t, "sglang", engineFamily("SGLang"))
	require.Equal(t, "", engineFamily("mindie"))
	require.Equal(t, "", engineFamily(""))
}

// The virtual parameters must never leak into ENGINE_ARGS as raw values; the
// same values are instead consumed by the composition step.
func TestSetEngineArgsSkipsVirtualParams(t *testing.T) {
	a := &DeployActivity{}
	envMap := map[string]string{}
	rc := runtimeConfig{
		EngineName:    "nvidia-vllm",
		EngineVersion: "25.11-py3",
		EngineArgsTemplates: []types.EngineArg{
			{Name: "max-model-len", Format: "--max-model-len %s"},
			{Name: "spec-decode-method", Value: "off", Format: "%s", Virtual: true},
			{Name: "spec-num-tokens", Format: "%s", Virtual: true},
		},
	}
	deployInfo := &database.Deploy{
		EngineArgs: `{"max-model-len":"8192","spec-num-tokens":"3","spec-decode-method":"mtp"}`,
	}

	state := a.setEngineArgs(context.Background(), nopLogger(), envMap, deployInfo, rc, "opencsg/test-model")

	require.Contains(t, envMap["ENGINE_ARGS"], "--max-model-len 8192")
	require.NotContains(t, envMap["ENGINE_ARGS"], "--spec-decode-method")
	require.NotContains(t, envMap["ENGINE_ARGS"], " mtp ")
	require.Contains(t, envMap["ENGINE_ARGS"], `--speculative-config {"method":"mtp","num_speculative_tokens":3}`)
	require.True(t, state.Active)
}

// The async-scheduling suppression must also fire from the speculative
// composition outcome, not only from an explicit async-scheduling engine arg.
func TestSetInferenceEnvAsyncConflict(t *testing.T) {
	a := &DeployActivity{}

	envMap := map[string]string{}
	a.setInferenceEnv(envMap, &database.Deploy{Type: types.InferenceType}, types.HardWare{}, "25.10-py3", specDecodeState{AsyncConflict: true})
	require.Equal(t, "true", envMap["ASYNC_SCHEDULING_DISABLED"])

	envMap = map[string]string{}
	a.setInferenceEnv(envMap, &database.Deploy{Type: types.InferenceType}, types.HardWare{}, "25.12-py3", specDecodeState{})
	require.NotContains(t, envMap, "ASYNC_SCHEDULING_DISABLED")

	envMap = map[string]string{}
	a.setInferenceEnv(envMap, &database.Deploy{Type: types.FinetuneType}, types.HardWare{}, "25.10-py3", specDecodeState{AsyncConflict: true})
	require.NotContains(t, envMap, "ASYNC_SCHEDULING_DISABLED")
}

func TestAutoInjectMTPSpeculativeArgs(t *testing.T) {
	tests := []struct {
		name              string
		engineName        string
		computeType       string
		version           string
		hasWeights        bool
		declaredLayers    int
		arch              string
		wantContains      string
		wantNotInject     bool
		wantActive        bool
		wantAsyncConflict bool
	}{
		{
			name:           "vllm NGC 25.11 clamps tokens to declared MTP layers",
			engineName:     "nvidia-vllm",
			computeType:    "gpu",
			version:        "25.11-py3",
			hasWeights:     true,
			declaredLayers: 1,
			wantContains:   `--speculative-config {"method":"mtp","num_speculative_tokens":1}`,
			wantActive:     true,
			// NGC 25.11 = vLLM 0.11.0, still inside the async-scheduling
			// conflict window.
			wantAsyncConflict: true,
		},
		{
			name:              "vllm NGC 25.10 uses deepseek_mtp and caps tokens at default",
			engineName:        "nvidia-vllm",
			computeType:       "gpu",
			version:           "25.10-py3",
			hasWeights:        true,
			declaredLayers:    3,
			arch:              "DeepseekV3ForCausalLM",
			wantContains:      `--speculative-config {"method":"deepseek_mtp","num_speculative_tokens":2}`,
			wantActive:        true,
			wantAsyncConflict: true,
		},
		{
			name:              "vllm NGC 25.10 GLM uses glm4_moe_mtp alias",
			engineName:        "nvidia-vllm",
			computeType:       "gpu",
			version:           "25.10-py3",
			hasWeights:        true,
			declaredLayers:    2,
			arch:              "Glm4moeForCausalLM",
			wantContains:      `--speculative-config {"method":"glm4_moe_mtp","num_speculative_tokens":2}`,
			wantActive:        true,
			wantAsyncConflict: true,
		},
		{
			name:           "vllm NGC 25.10 unknown arch skips injection (fail-closed)",
			engineName:     "nvidia-vllm",
			computeType:    "gpu",
			version:        "25.10-py3",
			hasWeights:     true,
			declaredLayers: 2,
			arch:           "LlamaForCausalLM",
			wantNotInject:  true,
		},
		{
			name:           "vllm weights without declared layers fall back to one token",
			engineName:     "nvidia-vllm",
			computeType:    "gpu",
			version:        "25.12-py3",
			hasWeights:     true,
			declaredLayers: 0,
			wantContains:   `--speculative-config {"method":"mtp","num_speculative_tokens":1}`,
			wantActive:     true,
		},
		{
			name:           "sglang NGC renders NEXTN with upstream defaults",
			engineName:     "nvidia-sglang",
			computeType:    "gpu",
			version:        "25.10-py3",
			hasWeights:     true,
			declaredLayers: 0,
			wantContains:   "--speculative-algorithm NEXTN --speculative-num-steps 3 --speculative-eagle-topk 1 --speculative-num-draft-tokens 4",
			wantActive:     true,
		},
		{
			name:           "sglang NGC clamps draft tokens to declared layers",
			engineName:     "nvidia-sglang",
			computeType:    "gpu",
			version:        "25.10-py3",
			hasWeights:     true,
			declaredLayers: 1,
			wantContains:   "--speculative-algorithm NEXTN --speculative-num-steps 1 --speculative-eagle-topk 1 --speculative-num-draft-tokens 2",
			wantActive:     true,
		},
		{
			name:           "no weights keeps args untouched",
			engineName:     "nvidia-vllm",
			computeType:    "gpu",
			version:        "25.11-py3",
			hasWeights:     false,
			declaredLayers: 1,
			wantNotInject:  true,
		},
		{
			name:           "unsupported engine is skipped",
			engineName:     "mindie",
			computeType:    "gpu",
			version:        "1.8-csg-1.0.RC2",
			hasWeights:     true,
			declaredLayers: 1,
			wantNotInject:  true,
		},
		{
			name:           "amd-vllm is skipped by vendor gate",
			engineName:     "amd-vllm",
			computeType:    "gpu",
			version:        "0.10.0",
			hasWeights:     true,
			declaredLayers: 1,
			wantNotInject:  true,
		},
		{
			name:           "vllm pre 0.10 is skipped",
			engineName:     "vllm",
			computeType:    "gpu",
			version:        "v0.8.2",
			hasWeights:     true,
			declaredLayers: 1,
			wantNotInject:  true,
		},
		{
			name:           "sglang pre 0.4 is skipped",
			engineName:     "sglang",
			computeType:    "gpu",
			version:        "v0.3.9",
			hasWeights:     true,
			declaredLayers: 1,
			wantNotInject:  true,
		},
		{
			name:           "generic vllm gpu v0.28.0 passes compute-type gate",
			engineName:     "vllm",
			computeType:    "gpu",
			version:        "v0.28.0",
			hasWeights:     true,
			declaredLayers: 1,
			wantContains:   `--speculative-config {"method":"mtp","num_speculative_tokens":1}`,
			wantActive:     true,
		},
		{
			name:           "generic vllm cpu is skipped by compute-type gate",
			engineName:     "vllm",
			computeType:    "cpu",
			version:        "v0.24.0",
			hasWeights:     true,
			declaredLayers: 1,
			wantNotInject:  true,
		},
		{
			name:           "generic vllm dcu is skipped by compute-type gate",
			engineName:     "vllm",
			computeType:    "dcu",
			version:        "v0.8.5",
			hasWeights:     true,
			declaredLayers: 1,
			wantNotInject:  true,
		},
		{
			name:           "ascend-vllm npu passes compute-type gate",
			engineName:     "ascend-vllm",
			computeType:    "npu",
			version:        "0.25.1rc",
			hasWeights:     true,
			declaredLayers: 1,
			wantContains:   `--speculative-config {"method":"mtp","num_speculative_tokens":1}`,
			wantActive:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args, state := autoInjectMTPSpeculativeArgs(nopLogger(), speculativeAutoInjectInput{
				EngineArgsStr: "--max-model-len 8192",
				Runtime:       runtimeConfig{EngineName: tt.engineName, EngineVersion: tt.version, ComputeType: tt.computeType},
				MTPMeta: func() (bool, int) {
					return tt.hasWeights, tt.declaredLayers
				},
				ArchLookup: func() string { return tt.arch },
			})
			if tt.wantNotInject {
				require.Equal(t, "--max-model-len 8192", args)
				require.False(t, state.Active)
				require.False(t, state.AsyncConflict)
				return
			}
			require.Contains(t, args, tt.wantContains)
			require.Equal(t, tt.wantActive, state.Active)
			require.Equal(t, tt.wantAsyncConflict, state.AsyncConflict)
		})
	}
}

// setEngineArgs must auto-inject MTP speculative decoding only when the flag
// is on, the user made no spec-decode choice, and the deploy is not PD
// disaggregation.
func TestSetEngineArgsAutoInjectMTP(t *testing.T) {
	tests := []struct {
		name             string
		flagOn           bool
		deployEngineArgs string
		pd               bool
		hasWeights       bool
		declaredLayers   int
		wantNotContains  string
		wantContains     string
		wantActive       bool
		wantMetaLookup   bool
	}{
		{
			name:             "injects when flag on, no method choice, weights present",
			flagOn:           true,
			deployEngineArgs: `{"max-model-len":"8192"}`,
			hasWeights:       true,
			declaredLayers:   1,
			wantContains:     `--speculative-config {"method":"mtp","num_speculative_tokens":1}`,
			wantActive:       true,
			wantMetaLookup:   true,
		},
		{
			name:             "no injection when flag off",
			flagOn:           false,
			deployEngineArgs: `{}`,
			hasWeights:       true,
			declaredLayers:   1,
			wantNotContains:  "--speculative-config",
		},
		{
			name:             "explicit off is never overridden",
			flagOn:           true,
			deployEngineArgs: `{"spec-decode-method":"off"}`,
			hasWeights:       true,
			declaredLayers:   1,
			wantNotContains:  "--speculative-config",
		},
		{
			name:             "unknown method is never overridden",
			flagOn:           true,
			deployEngineArgs: `{"spec-decode-method":"eagle"}`,
			hasWeights:       true,
			declaredLayers:   1,
			wantNotContains:  "--speculative-config",
		},
		{
			name:             "skips PD disaggregation deploys",
			flagOn:           true,
			deployEngineArgs: `{}`,
			pd:               true,
			hasWeights:       true,
			declaredLayers:   1,
			wantNotContains:  "--speculative-config",
		},
		{
			name:             "no injection without MTP weights",
			flagOn:           true,
			deployEngineArgs: `{}`,
			hasWeights:       false,
			wantNotContains:  "--speculative-config",
			wantMetaLookup:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tester := setupTest(t)
			tester.activities.cfg.AutoSpeculativeDecoding = tt.flagOn
			tester.mockDeployTaskStore.EXPECT().UpdateInTx(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
			if tt.wantMetaLookup {
				tester.mockMetadataStore.EXPECT().FindByRepoID(mock.Anything, int64(42)).Return(&database.Metadata{
					HasMTPWeights:         tt.hasWeights,
					NumNextNPredictLayers: tt.declaredLayers,
				}, nil)
			}

			deployInfo := &database.Deploy{
				EngineArgs: tt.deployEngineArgs,
				RepoID:     42,
			}
			if tt.pd {
				deployInfo.PD = &types.PDConfig{}
			}
			rc := runtimeConfig{
				EngineName:    "nvidia-vllm",
				EngineVersion: "25.11-py3",
				ComputeType:   "gpu",
				EngineArgsTemplates: []types.EngineArg{
					{Name: "max-model-len", Format: "--max-model-len %s"},
					{Name: "spec-decode-method", Value: "off", Format: "%s", Virtual: true},
				},
			}

			envMap := map[string]string{}
			state := tester.activities.setEngineArgs(context.Background(), nopLogger(), envMap, deployInfo, rc, "opencsg/test-model")

			if tt.wantNotContains != "" {
				require.NotContains(t, envMap["ENGINE_ARGS"], tt.wantNotContains)
				require.False(t, state.Active)
				return
			}
			require.Contains(t, envMap["ENGINE_ARGS"], tt.wantContains)
			require.Contains(t, envMap["ENGINE_ARGS"], "--max-model-len 8192")
			require.Equal(t, tt.wantActive, state.Active)
		})
	}
}

// eagle3 draft weights are downloaded by the container entry script; the
// selected draft repo must be forwarded as SPEC_DRAFT_REPO_ID and never set for
// other methods.
func TestSetEngineArgsDraftRepoEnv(t *testing.T) {
	rc := runtimeConfig{
		EngineName:    "nvidia-vllm",
		EngineVersion: "25.11-py3",
		EngineArgsTemplates: []types.EngineArg{
			{Name: "spec-decode-method", Value: "off", Format: "%s", Virtual: true},
			{Name: "spec-draft-model", Format: "%s", Virtual: true},
		},
	}
	// The "test" value keeps getLogger off the temporal activity logger.
	ctx := context.WithValue(context.Background(), "test", "test")

	// vLLM 25.11 allows the full eagle3 target whitelist, so the composition
	// resolves the model architecture from metadata.
	mds := mockdb.NewMockMetadataStore(t)
	mds.EXPECT().FindByRepoID(mock.Anything, int64(0)).Return(&database.Metadata{
		Architecture: "LlamaForCausalLM",
	}, nil)
	a := &DeployActivity{mds: mds}
	envMap := map[string]string{}
	deployInfo := &database.Deploy{
		EngineArgs: `{"spec-decode-method":"eagle3","spec-draft-model":"opencsg/eagle3-llama","spec-num-tokens":"3"}`,
	}
	state := a.setEngineArgs(ctx, nopLogger(), envMap, deployInfo, rc, "opencsg/test-model")
	require.Equal(t, "opencsg/eagle3-llama", envMap["SPEC_DRAFT_REPO_ID"])
	require.True(t, state.Active)

	// A draft repo selection without eagle3 must not leak into SPEC_DRAFT_REPO_ID.
	envMap = map[string]string{}
	deployInfo = &database.Deploy{
		EngineArgs: `{"spec-decode-method":"mtp","spec-draft-model":"opencsg/eagle3-llama"}`,
	}
	state = a.setEngineArgs(ctx, nopLogger(), envMap, deployInfo, rc, "opencsg/test-model")
	require.NotContains(t, envMap, "SPEC_DRAFT_REPO_ID")
	require.True(t, state.Active)
}

func TestJsonSafeValue(t *testing.T) {
	// Hub repo ids and local model paths are the legitimate values.
	require.True(t, jsonSafeValue("opencsg/EAGLE3-Llama-3.1-8B-Instruct"))
	require.True(t, jsonSafeValue("/workspace/models/eagle3-draft_2"))

	// Path traversal is rejected even though '.' is individually allowed.
	require.False(t, jsonSafeValue("../etc/passwd"))
	require.False(t, jsonSafeValue("opencsg/.."))
	// Everything else fails closed: JSON breakers, shell metacharacters,
	// globbing characters, whitespace, and non-ASCII.
	for _, s := range []string{
		"",
		"a b",
		`a"b`,
		`a\b`,
		"a\tb",
		"a;b",
		"a$b",
		"a`b",
		"a*b",
		"a?b",
		"a[b]",
		"a{b}",
		"opencsg/$(id)",
		"模型",
	} {
		require.Falsef(t, jsonSafeValue(s), "expected %q to be rejected", s)
	}
}

func TestSpecDecodeStateFromEngineArgs(t *testing.T) {
	tests := []struct {
		name        string
		engineArgs  string
		wantEnabled bool
		wantMethod  string
	}{
		{name: "empty", engineArgs: "", wantEnabled: false},
		{name: "not json", engineArgs: "not-json", wantEnabled: false},
		{name: "unset", engineArgs: `{"spec-num-tokens":"2"}`, wantEnabled: false},
		{name: "off", engineArgs: `{"spec-decode-method":"off"}`, wantEnabled: false, wantMethod: "off"},
		{name: "eagle3", engineArgs: `{"spec-decode-method":"eagle3","spec-draft-model":"opencsg/eagle3-llama"}`, wantEnabled: true, wantMethod: "eagle3"},
		{name: "ngram", engineArgs: `{"spec-decode-method":"ngram"}`, wantEnabled: true, wantMethod: "ngram"},
		{name: "mtp", engineArgs: `{"spec-decode-method":"mtp"}`, wantEnabled: true, wantMethod: "mtp"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			enabled, method := SpecDecodeStateFromEngineArgs(tt.engineArgs)
			require.Equal(t, tt.wantEnabled, enabled)
			require.Equal(t, tt.wantMethod, method)
		})
	}
}
