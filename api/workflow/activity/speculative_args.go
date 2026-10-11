package activity

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"go.temporal.io/sdk/log"
)

// Spec-decode virtual parameters (see configs/inference/*.json). They never
// render as standalone flags; composeSpeculativeArgs turns them into one
// engine-specific argument group.
const (
	specArgMethod     = "spec-decode-method"
	specArgNumTokens  = "spec-num-tokens"
	specArgDraftModel = "spec-draft-model"

	specMethodOff    = "off"
	specMethodMTP    = "mtp"
	specMethodEagle3 = "eagle3"
	specMethodNgram  = "ngram"

	// defaultSpecNumTokens is the vLLM num_speculative_tokens default.
	defaultSpecNumTokens = 2
	// defaultSGLangDraftTokens matches the upstream NEXTN recommendation for
	// DeepSeek/GLM MTP models (steps 3, eagle-topk 1, draft tokens 4).
	defaultSGLangDraftTokens = 4

	ngramPromptLookupMax = 4
	ngramPromptLookupMin = 2

	// maxSpecNumTokens caps the speculative token count: values above 8
	// waste draft compute without measurable gain (vLLM and SGLang both
	// recommend 1-4 in practice).
	maxSpecNumTokens = 8
)

// specDecodeState carries the spec-decode composition outcome to later render
// phases. Today the consumers are setInferenceEnv (which must suppress the
// serve script's default --async-scheduling on conflicting vLLM versions) and
// setEngineArgs (which surfaces a visible skip reason when the user's
// spec-decode selection was rejected).
type specDecodeState struct {
	// Active is true when a speculative config was composed into ENGINE_ARGS.
	Active bool
	// AsyncConflict marks vLLM versions where speculative decoding
	// hard-conflicts with --async-scheduling (upstream 0.10.0-0.11.x, mapped
	// NGC 25.09-25.11; lifted upstream in 0.12.0).
	AsyncConflict bool
	// SkipReason carries a short reason when the user selected a spec-decode
	// method but composition was rejected (fail-closed). Empty when Active or
	// when the user did not select a method. Surfaced as
	// SPEC_DECODE_SKIP_REASON in the container env so operators can see that
	// the user's selection was silently dropped (issue #1489 review R11).
	SkipReason string
}

// speculativeComposeInput groups the inputs of composeSpeculativeArgs.
type speculativeComposeInput struct {
	// EngineArgsStr is the ENGINE_ARGS composed so far (non-speculative args).
	EngineArgsStr string
	// ArgValues is the user-selected deploy engine_args map.
	ArgValues map[string]string
	// Runtime is the resolved runtime framework metadata.
	Runtime runtimeConfig
	// ArchLookup lazily resolves the model architecture; it is called
	// when the vLLM eagle3 target whitelist must be enforced and when
	// the MTP method alias must be selected on pre-0.11 versions.
	ArchLookup func() string
}

// composeSpeculativeArgs appends the engine-specific speculative decoding
// arguments when the user selected a spec-decode method. It fails closed:
// unsupported engines, unparsable/too-old versions, missing or unsafe draft
// model values, and invalid token counts leave ENGINE_ARGS untouched and only
// log. The returned state reports whether a config was composed.
func composeSpeculativeArgs(logger log.Logger, in speculativeComposeInput) (string, specDecodeState) {
	skip := func(reason string) (string, specDecodeState) {
		logger.Warn("speculative decoding skipped", "reason", reason, "method", in.ArgValues[specArgMethod])
		return in.EngineArgsStr, specDecodeState{SkipReason: reason}
	}
	method := strings.TrimSpace(in.ArgValues[specArgMethod])
	if method == "" || method == specMethodOff {
		return in.EngineArgsStr, specDecodeState{}
	}
	if method != specMethodMTP && method != specMethodEagle3 && method != specMethodNgram {
		return skip("unknown spec-decode-method")
	}

	family := engineFamily(in.Runtime.EngineName)
	if family == "" {
		return skip("unsupported engine")
	}

	ver := parseEngineVersion(in.Runtime.EngineVersion)
	switch {
	case family == "vllm" && !vllmSupportsSpeculativeJSON(ver):
		return skip("vLLM version does not support --speculative-config")
	case family == "vllm" && method == specMethodEagle3 && !vllmSupportsEagle3(ver):
		return skip("vLLM version does not support eagle3")
	case family == "sglang" && method != specMethodNgram && !sglangSupportsSpeculative(ver):
		return skip("SGLang version does not support speculative decoding")
	case family == "sglang" && method == specMethodNgram && !sglangSupportsNgram(ver):
		return skip("SGLang version does not support ngram")
	}

	numTokens, err := specNumTokens(in.ArgValues[specArgNumTokens], family, method)
	if err != nil {
		return skip(fmt.Sprintf("invalid spec-num-tokens: %s", in.ArgValues[specArgNumTokens]))
	}

	draftModel := strings.TrimSpace(in.ArgValues[specArgDraftModel])
	if method == specMethodEagle3 {
		if draftModel == "" {
			return skip("eagle3 requires spec-draft-model")
		}
		if !jsonSafeValue(draftModel) {
			return skip("spec-draft-model contains unsafe characters")
		}
	}

	state := specDecodeState{Active: true}
	if family == "vllm" {
		if vllmAsyncSchedulingConflict(ver) {
			state.AsyncConflict = true
		}
		// Architecture lookup is deferred until needed: eagle3 requires it for
		// the target whitelist, and MTP on pre-0.11 versions needs it for the
		// architecture-specific alias. Other methods (ngram, MTP on 0.11+)
		// never call ArchLookup.
		var archStr string
		if method == specMethodEagle3 {
			archStr = strings.ToLower(strings.TrimSpace(in.ArchLookup()))
			if archStr == "" || !vllmEagle3ArchSupported(archStr, ver) {
				return skip("eagle3 target architecture not whitelisted for this vLLM version")
			}
		} else if method == specMethodMTP && !vllmSupportsMTPAlias(ver) {
			archStr = in.ArchLookup()
		}
		config := buildVllmSpeculativeConfig(ver, method, numTokens, draftModel, archStr)
		if config == "" {
			return skip("cannot select MTP alias for this architecture on this vLLM version")
		}
		return in.EngineArgsStr + " " + config, state
	}
	if method == specMethodEagle3 {
		// SGLang eagle3 also requires architecture metadata so the draft model
		// target is known; an empty lookup is rejected (fail-closed).
		arch := strings.ToLower(strings.TrimSpace(in.ArchLookup()))
		if arch == "" {
			return skip("eagle3 requires architecture metadata for SGLang")
		}
	}
	sgArgs := buildSGLangSpeculativeArgs(method, numTokens, draftModel)
	if sgArgs == "" {
		return skip("cannot build SGLang speculative args for this method")
	}
	return in.EngineArgsStr + " " + sgArgs, state
}

// speculativeAutoInjectInput groups the inputs of autoInjectMTPSpeculativeArgs.
type speculativeAutoInjectInput struct {
	// EngineArgsStr is the ENGINE_ARGS composed so far (non-speculative args).
	EngineArgsStr string
	// Runtime is the resolved runtime framework metadata.
	Runtime runtimeConfig
	// MTPMeta lazily resolves the model's MTP capability from the metadata
	// store; it is called only after the engine and version gates pass.
	MTPMeta func() (hasWeights bool, declaredLayers int)
	// ArchLookup lazily resolves the model architecture; it is called only
	// when the vLLM MTP alias must be selected on pre-0.11 versions.
	ArchLookup func() string
}

// autoInjectMTPSpeculativeArgs appends MTP speculative decoding arguments when
// the checkpoint embeds MTP weights and the caller already ensured the user
// made no spec-decode choice. Like composeSpeculativeArgs it fails closed:
// unknown engines, unparsable/too-old versions and missing weights leave
// ENGINE_ARGS untouched.
func autoInjectMTPSpeculativeArgs(logger log.Logger, in speculativeAutoInjectInput) (string, specDecodeState) {
	var zero specDecodeState
	family := engineFamily(in.Runtime.EngineName)
	if family == "" {
		logger.Warn("speculative decoding is not supported for this engine, skipping automatic injection",
			"engine", in.Runtime.EngineName)
		return in.EngineArgsStr, zero
	}
	// MTP auto-injection is gated by compute type: only GPU and NPU
	// runtimes are eligible (CPU and DCU lack the CUDA graph support
	// that speculative decoding depends on). AMD and Metax GPUs share
	// the "vllm" engine name fragment but have not been validated for
	// MTP, so they are excluded by engine name.
	if !mtpAutoInjectSupported(in.Runtime.ComputeType, in.Runtime.EngineName) {
		logger.Warn("MTP auto-injection is not supported for this compute type or vendor, skipping",
			"engine", in.Runtime.EngineName, "compute_type", in.Runtime.ComputeType)
		return in.EngineArgsStr, zero
	}

	ver := parseEngineVersion(in.Runtime.EngineVersion)
	switch {
	case family == "vllm" && !vllmSupportsSpeculativeJSON(ver):
		logger.Warn("vLLM version does not support --speculative-config, skipping automatic injection",
			"version", in.Runtime.EngineVersion)
		return in.EngineArgsStr, zero
	case family == "sglang" && !sglangSupportsSpeculative(ver):
		logger.Warn("SGLang version does not support speculative decoding, skipping automatic injection",
			"version", in.Runtime.EngineVersion)
		return in.EngineArgsStr, zero
	}

	hasWeights, declaredLayers := in.MTPMeta()
	if !hasWeights {
		logger.Info("MTP auto-injection skipped: model checkpoint has no MTP weights")
		return in.EngineArgsStr, zero
	}

	state := specDecodeState{Active: true}
	if family == "vllm" {
		// Clamp to the declared MTP layer count and fall back to a single
		// speculative token when weights were detected without a declared
		// layer count.
		tokens := min(defaultSpecNumTokens, max(1, declaredLayers))
		if vllmAsyncSchedulingConflict(ver) {
			state.AsyncConflict = true
		}
		// Only look up architecture when the version lacks the unified "mtp"
		// alias (pre-0.11): those versions need an architecture-specific name
		// (deepseek_mtp / glm4_moe_mtp). 0.11+ uses "mtp" for all archs.
		arch := ""
		if !vllmSupportsMTPAlias(ver) && in.ArchLookup != nil {
			arch = in.ArchLookup()
		}
		config := buildVllmSpeculativeConfig(ver, specMethodMTP, tokens, "", arch)
		if config == "" {
			// Pre-0.11 versions need an architecture-specific MTP alias; if
			// the architecture is unknown, the alias cannot be selected
			// safely (fail-closed).
			logger.Warn("auto-injecting vLLM MTP skipped: architecture unknown, cannot select MTP alias",
				"version", in.Runtime.EngineVersion)
			return in.EngineArgsStr, zero
		}
		logger.Info("auto-injecting vLLM MTP speculative decoding",
			"version", in.Runtime.EngineVersion, "num_speculative_tokens", tokens)
		return in.EngineArgsStr + " " + config, state
	}
	// SGLang: clamp draft tokens to the declared MTP layer count, with a
	// minimum of 2 (num_draft_tokens = num_steps + 1, num_steps >= 1).
	tokens := defaultSGLangDraftTokens
	if declaredLayers > 0 {
		tokens = min(defaultSGLangDraftTokens, max(2, declaredLayers))
	}
	logger.Info("auto-injecting SGLang MTP speculative decoding",
		"version", in.Runtime.EngineVersion, "num_draft_tokens", tokens)
	return in.EngineArgsStr + " " + buildSGLangSpeculativeArgs(specMethodMTP, tokens, ""), state
}

// buildVllmSpeculativeConfig renders the --speculative-config argument group.
// The JSON value stays space-free because ENGINE_ARGS is expanded unquoted by
// the serve scripts (same convention as --hf-overrides). The arch parameter
// selects the MTP method alias on pre-0.11 versions that lack the unified
// "mtp" name.
func buildVllmSpeculativeConfig(ver engineVersion, method string, numTokens int, draftModel, arch string) string {
	const flag = "--speculative-config "
	switch method {
	case specMethodMTP:
		name := mtpMethodAlias(ver, arch)
		if name == "" {
			return ""
		}
		return flag + fmt.Sprintf(`{"method":"%s","num_speculative_tokens":%d}`, name, numTokens)
	case specMethodEagle3:
		// The draft repo is snapshot-downloaded to /workspace/<repo-id> by the
		// container entry script.
		return flag + fmt.Sprintf(`{"method":"eagle3","model":"/workspace/%s","num_speculative_tokens":%d}`, draftModel, numTokens)
	case specMethodNgram:
		return flag + fmt.Sprintf(`{"method":"ngram","num_speculative_tokens":%d,"prompt_lookup_max":%d,"prompt_lookup_min":%d}`,
			numTokens, ngramPromptLookupMax, ngramPromptLookupMin)
	}
	return ""
}

// buildSGLangSpeculativeArgs renders the SGLang speculative flag group. With
// eagle-topk=1 the engine forces num_draft_tokens = num_steps + 1, so the
// steps value is derived from the user-selected draft token count.
func buildSGLangSpeculativeArgs(method string, numDraftTokens int, draftModel string) string {
	switch method {
	case specMethodMTP:
		// NEXTN reads the embedded MTP weights from the target checkpoint; no
		// draft model path is needed (DeepSeek-V3/R1/V3.1, GLM-4.5/4.6).
		return fmt.Sprintf("--speculative-algorithm NEXTN --speculative-num-steps %d --speculative-eagle-topk 1 --speculative-num-draft-tokens %d",
			numDraftTokens-1, numDraftTokens)
	case specMethodEagle3:
		return fmt.Sprintf("--speculative-algorithm EAGLE3 --speculative-draft-model-path /workspace/%s --speculative-num-steps %d --speculative-eagle-topk 1 --speculative-num-draft-tokens %d",
			draftModel, numDraftTokens-1, numDraftTokens)
	case specMethodNgram:
		return fmt.Sprintf("--speculative-algorithm NGRAM --speculative-num-draft-tokens %d", numDraftTokens)
	}
	return ""
}

// specNumTokens resolves the speculative token count. SGLang derives
// num_steps = n-1 for NEXTN/EAGLE3, so those methods require n >= 2; ngram
// has no such constraint and allows n >= 1.
func specNumTokens(raw string, family, method string) (int, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		if family == "sglang" {
			return defaultSGLangDraftTokens, nil
		}
		return defaultSpecNumTokens, nil
	}
	n, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("spec-num-tokens must be an integer, got %q", raw)
	}
	if n < 1 {
		return 0, fmt.Errorf("spec-num-tokens must be positive, got %d", n)
	}
	if n > maxSpecNumTokens {
		return 0, fmt.Errorf("spec-num-tokens must be <= %d, got %d", maxSpecNumTokens, n)
	}
	if family == "sglang" && method != specMethodNgram && n < 2 {
		return 0, fmt.Errorf("spec-num-tokens must be >= 2 for SGLang %s, got %d", method, n)
	}
	return n, nil
}

// engineFamily maps a runtime framework name to a render family. Framework
// names carry vendor prefixes and historical casing ("nvidia-vllm",
// "VLLM", "SGLang"), so matching stays substring-based and case-insensitive.
func engineFamily(engineName string) string {
	n := strings.ToLower(strings.TrimSpace(engineName))
	switch {
	case strings.Contains(n, "sglang"):
		return "sglang"
	case strings.Contains(n, "vllm"):
		return "vllm"
	default:
		return ""
	}
}

// engineVersion is a parsed runtime framework version. NGC containers use
// year.month tags ("25.11-py3", year >= 20) while upstream tags are
// semver-ish ("v0.10.2-cu128", "0.5.4.post1", "v0.25.1rc").
type engineVersion struct {
	ok                  bool
	isNGC               bool
	major, minor, patch int
}

var engineVersionRe = regexp.MustCompile(`^[vV]?(\d+)(?:\.(\d+))?(?:\.(\d+))?`)

func parseEngineVersion(version string) engineVersion {
	m := engineVersionRe.FindStringSubmatch(strings.TrimSpace(version))
	if m == nil {
		return engineVersion{}
	}
	major, _ := strconv.Atoi(m[1])
	parsed := engineVersion{ok: true, major: major}
	if m[2] != "" {
		parsed.minor, _ = strconv.Atoi(m[2])
	}
	if m[3] != "" {
		parsed.patch, _ = strconv.Atoi(m[3])
	}
	if major >= 20 {
		parsed.isNGC = true
	}
	return parsed
}

// ngcAtLeast reports whether an NGC version tag is at least year.month.
// Non-NGC versions always return false.
func ngcAtLeast(v engineVersion, year, month int) bool {
	return v.isNGC && (v.major > year || (v.major == year && v.minor >= month))
}

// vllmSupportsSpeculativeJSON reports whether the engine takes the
// --speculative-config JSON form: introduced upstream in 0.8.2 (old flags
// deprecated), old flags removed in 0.8.3. However, MTP speculative decoding
// was only validated on v0.10.0+ for community builds; older releases
// (0.8.x-0.9.x) may accept the flag but fail at runtime. NGC containers
// start at 25.09 (= upstream 0.10.1), so only NGC 25.09+ passes.
func vllmSupportsSpeculativeJSON(v engineVersion) bool {
	if v.isNGC {
		return ngcAtLeast(v, 25, 9)
	}
	return v.ok && (v.major > 0 || v.minor >= 10)
}

// vllmSupportsEagle3 reports whether method "eagle3" exists: it landed in
// upstream 0.8.5, and older releases silently rewrite it to "eagle"/
// "draft_model" and fail at startup. NGC containers start at 25.09
// (= upstream 0.10.1), so only NGC 25.09+ passes.
func vllmSupportsEagle3(v engineVersion) bool {
	if v.isNGC {
		return ngcAtLeast(v, 25, 9)
	}
	return v.ok && (v.major > 0 || v.minor > 8 || (v.minor == 8 && v.patch >= 5))
}

// vllmSupportsMTPAlias reports whether method "mtp" resolves to DeepSeek MTP:
// the alias exists since upstream 0.11.0 (NGC 25.11+); older versions need
// the explicit "deepseek_mtp" name.
func vllmSupportsMTPAlias(v engineVersion) bool {
	if v.isNGC {
		return ngcAtLeast(v, 25, 11)
	}
	return v.ok && (v.major > 0 || v.minor >= 11)
}

// mtpMethodAlias resolves the vLLM speculative-config method name for MTP.
// Versions that support the unified "mtp" alias (0.11.0+, NGC 25.11+) use it
// for every architecture. Older versions need an architecture-specific name:
// "deepseek_mtp" for DeepSeek-family and "glm4_moe_mtp" for GLM-family. An
// empty arch on a pre-0.11 version returns "" (fail-closed: the wrong alias
// would make the engine fail at startup).
func mtpMethodAlias(ver engineVersion, arch string) string {
	if vllmSupportsMTPAlias(ver) {
		return "mtp"
	}
	a := strings.ToLower(strings.TrimSpace(arch))
	switch {
	case strings.Contains(a, "glm"):
		return "glm4_moe_mtp"
	case strings.Contains(a, "deepseek"):
		return "deepseek_mtp"
	default:
		return ""
	}
}

// vllmAsyncSchedulingConflict reports the window where vLLM rejects
// speculative decoding combined with --async-scheduling at startup:
// upstream 0.10.0-0.11.x, lifted in 0.12.0. NGC 25.09-25.11 map to that
// window.
func vllmAsyncSchedulingConflict(v engineVersion) bool {
	if v.isNGC {
		return v.major == 25 && v.minor >= 9 && v.minor <= 11
	}
	return v.ok && v.major == 0 && v.minor >= 10 && v.minor < 12
}

// sglangSupportsSpeculative gates the NEXTN/EAGLE3 methods: both predate the
// oldest fleet image (0.4.6). NGC sglang containers start at 25.10
// (ships 0.5.3rc1), so only NGC 25.10+ passes.
func sglangSupportsSpeculative(v engineVersion) bool {
	if v.isNGC {
		return ngcAtLeast(v, 25, 10)
	}
	return v.ok && (v.major > 0 || v.minor >= 4)
}

// sglangSupportsNgram gates the NGRAM algorithm: it landed in SGLang 0.5.4.
// NGC 25.10 ships 0.5.3rc1 which rejects it at startup, while NGC 25.11
// ships 0.5.4.post1 (verified from the image labels), so NGC 25.11+ passes.
func sglangSupportsNgram(v engineVersion) bool {
	if v.isNGC {
		return ngcAtLeast(v, 25, 11)
	}
	return v.ok && (v.major > 0 || v.minor > 5 || (v.minor == 5 && v.patch >= 4))
}

// eagle3 target-family whitelists, matched as lowercase fragments of the
// model architecture. The whitelist shipped together with eagle3 itself and
// only grew over time, so vllmEagle3TargetFragments selects the list that
// matches the engine version.
var (
	// vllmEagle3TargetsLlama is the whitelist from 0.8.5 until 0.10.0.
	vllmEagle3TargetsLlama = []string{"llama"}
	// vllmEagle3TargetsLlamaQwen adds qwen: upstream 0.10.1-0.10.2, which is
	// also what NGC 25.09/25.10 ship.
	vllmEagle3TargetsLlamaQwen = []string{"llama", "qwen"}
	// vllmEagle3TargetsLlamaQwenMinicpmGptOss adds minicpm and gpt_oss:
	// upstream 0.11.0 and NGC 25.11+. The lowercase fragments "gptoss" and
	// "gpt_oss" match model_type values like "gpt_oss" for the current family
	// set.
	vllmEagle3TargetsLlamaQwenMinicpmGptOss = []string{"llama", "qwen", "minicpm", "gptoss", "gpt_oss"}
)

// vllmEagle3TargetFragments returns the eagle3 target whitelist enforced by
// the given engine version.
func vllmEagle3TargetFragments(v engineVersion) []string {
	if !v.ok {
		return vllmEagle3TargetsLlama
	}
	if v.isNGC {
		switch {
		case v.major > 25 || (v.major == 25 && v.minor >= 11):
			return vllmEagle3TargetsLlamaQwenMinicpmGptOss
		case v.major == 25 && v.minor >= 9:
			return vllmEagle3TargetsLlamaQwen
		default:
			return vllmEagle3TargetsLlama
		}
	}
	switch {
	case v.major > 0 || v.minor >= 11:
		return vllmEagle3TargetsLlamaQwenMinicpmGptOss
	case v.minor == 10 && v.patch >= 1:
		return vllmEagle3TargetsLlamaQwen
	default:
		return vllmEagle3TargetsLlama
	}
}

func vllmEagle3ArchSupported(archLower string, v engineVersion) bool {
	for _, fragment := range vllmEagle3TargetFragments(v) {
		if strings.Contains(archLower, fragment) {
			return true
		}
	}
	return false
}

// mtpAutoInjectSupported reports whether MTP auto-injection is safe for the
// given compute type and engine vendor. Only GPU and NPU runtimes are eligible
// (CPU and DCU lack CUDA graph support). AMD and Metax GPUs share the "vllm"
// engine name fragment but have not been validated for MTP, so they are
// excluded by engine name.
func mtpAutoInjectSupported(computeType, engineName string) bool {
	n := strings.ToLower(engineName)
	if strings.Contains(n, "amd") || strings.Contains(n, "metax") {
		return false
	}
	ct := strings.ToLower(strings.TrimSpace(computeType))
	return ct == "gpu" || ct == "npu"
}

// jsonSafeValue reports whether s can be embedded as a value in the
// space-free JSON of the unquoted $ENGINE_ARGS expansion. It is an allowlist
// of the characters a hub repo id or local model path can contain: each of
// them is inert in JSON strings and cannot trigger shell word splitting,
// globbing, or expansion, so no metacharacter blacklist has to track the
// serve scripts' shell behavior.
func jsonSafeValue(s string) bool {
	if s == "" {
		return false
	}
	if strings.Contains(s, "..") {
		return false
	}
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z',
			r >= 'A' && r <= 'Z',
			r >= '0' && r <= '9',
			r == '.' || r == '_' || r == '-' || r == '/':
		default:
			return false
		}
	}
	return true
}

// SpecDecodeStateFromEngineArgs reports the speculative decoding state
// selected by a deploy's persisted engine_args JSON. It reads only the
// user-visible selection: the deploy-time MTP auto-injection never writes
// back to engine_args and is therefore not visible here. Unparsable or empty
// input reports disabled with an empty method.
func SpecDecodeStateFromEngineArgs(raw string) (enabled bool, method string) {
	if strings.TrimSpace(raw) == "" {
		return false, ""
	}
	var values map[string]any
	if err := json.Unmarshal([]byte(raw), &values); err != nil {
		return false, ""
	}
	method, _ = values[specArgMethod].(string)
	method = strings.TrimSpace(method)
	return method != "" && method != specMethodOff, method
}
