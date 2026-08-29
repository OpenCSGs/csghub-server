package responses

import (
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	commonutils "opencsg.com/csghub-server/common/utils/common"
)

type ResponsesExecutionMode string

const (
	ResponsesModeNative      ResponsesExecutionMode = "native"
	ResponsesModeChatAdapter ResponsesExecutionMode = "chat_adapter"
	ResponsesModeDisabled    ResponsesExecutionMode = "disabled"
)

type RoutingDecision struct {
	Mode       ResponsesExecutionMode
	BackendURL string
	Reason     string
}

type RoutingTarget struct {
	ModelID          string
	Target           string
	CSGHubHosted     bool
	RuntimeFramework string
	ImageID          string
}

func ResolveRouting(modelTarget RoutingTarget) (RoutingDecision, error) {
	if strings.TrimSpace(modelTarget.ModelID) == "" {
		return RoutingDecision{}, fmt.Errorf("model target is nil")
	}
	target := strings.TrimSpace(modelTarget.Target)
	parsed, err := parseResponsesRoutingTarget(target, modelTarget.CSGHubHosted)
	if err != nil {
		return RoutingDecision{}, fmt.Errorf("cannot resolve responses mode from upstream url %q: %w", target, err)
	}

	path := strings.TrimRight(parsed.Path, "/")
	switch {
	case PathEndsWithSegments(path, "responses"):
		return RoutingDecision{Mode: ResponsesModeNative, BackendURL: target, Reason: "upstream_url_responses"}, nil
	case PathEndsWithSegments(path, "chat", "completions"):
		return RoutingDecision{Mode: ResponsesModeChatAdapter, BackendURL: target, Reason: "upstream_url_chat_completions"}, nil
	}

	if modelTarget.CSGHubHosted {
		return resolveHostedRouting(modelTarget, parsed)
	}

	return RoutingDecision{Mode: ResponsesModeDisabled, Reason: "unsupported_upstream_url"}, nil
}

func parseResponsesRoutingTarget(target string, allowSchemeLessHost bool) (*url.URL, error) {
	parsed, err := url.Parse(target)
	if err != nil {
		return nil, err
	}
	if parsed.Scheme != "" && parsed.Host != "" {
		return parsed, nil
	}
	if !allowSchemeLessHost {
		return nil, fmt.Errorf("upstream url must include scheme and host")
	}
	if !strings.Contains(target, "://") && strings.Contains(target, "/") && strings.Contains(target, ":") {
		return nil, fmt.Errorf("target %q looks like host:port with path, please prepend \"http://\"", target)
	}
	if !strings.Contains(target, "://") && !strings.Contains(target, "/") {
		parsed, err := url.Parse("http://" + target)
		if err != nil || parsed.Host == "" {
			return nil, fmt.Errorf("upstream url must include host")
		}
		return parsed, nil
	}
	return nil, fmt.Errorf("upstream url must include host")
}

func PathEndsWithSegments(path string, segments ...string) bool {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) < len(segments) {
		return false
	}
	offset := len(parts) - len(segments)
	for idx, segment := range segments {
		if parts[offset+idx] != segment {
			return false
		}
	}
	return true
}

func resolveHostedRouting(modelTarget RoutingTarget, parsed *url.URL) (RoutingDecision, error) {
	runtime := strings.ToLower(strings.TrimSpace(modelTarget.RuntimeFramework))
	image := strings.ToLower(strings.TrimSpace(modelTarget.ImageID))
	switch {
	case isHostedVLLM(runtime, image) && isVLLMNativeResponsesCapable(runtime, image):
		backendURL := appendEndpointPath(parsed, "v1", "responses")
		return RoutingDecision{Mode: ResponsesModeNative, BackendURL: backendURL, Reason: "csghub_hosted_vllm_native"}, nil
	case isHostedVLLM(runtime, image):
		return RoutingDecision{Mode: ResponsesModeChatAdapter, BackendURL: appendEndpointPath(parsed, "v1", "chat", "completions"), Reason: "csghub_hosted_vllm_chat_adapter"}, nil
	case isHostedSGLang(runtime, image) && isSGLangNativeResponsesCapable(runtime, image):
		backendURL := appendEndpointPath(parsed, "v1", "responses")
		return RoutingDecision{Mode: ResponsesModeNative, BackendURL: backendURL, Reason: "csghub_hosted_sglang_native"}, nil
	case strings.Contains(runtime, "sglang") || strings.Contains(image, "sglang"):
		return RoutingDecision{Mode: ResponsesModeChatAdapter, BackendURL: appendEndpointPath(parsed, "v1", "chat", "completions"), Reason: "csghub_hosted_sglang_chat_adapter"}, nil
	default:
		return RoutingDecision{Mode: ResponsesModeDisabled, Reason: "unsupported_csghub_hosted_runtime"}, nil
	}
}

func isHostedVLLM(runtime, image string) bool {
	return strings.Contains(runtime, "vllm") || strings.Contains(image, "vllm")
}

func isHostedSGLang(runtime, image string) bool {
	return strings.Contains(runtime, "sglang") || strings.Contains(image, "sglang")
}

func isVLLMNativeResponsesCapable(runtime, image string) bool {
	for _, value := range []string{runtime, image} {
		if vllmVersionAtLeast(value, 0, 24, 0) {
			return true
		}
	}
	return false
}

var vllmVersionPattern = regexp.MustCompile(`(?i)(?:^|[/_:])vllm[:_]?(v?)([0-9]+)\.([0-9]+)(?:\.([0-9]+))?`)

func vllmVersionAtLeast(value string, major, minor, patch int) bool {
	matches := vllmVersionPattern.FindAllStringSubmatch(value, -1)
	capable := false
	for _, match := range matches {
		if len(match) != 5 {
			continue
		}
		explicitVersionPrefix := strings.EqualFold(match[1], "v")
		gotMajor, errMajor := strconv.Atoi(match[2])
		gotMinor, errMinor := strconv.Atoi(match[3])
		gotPatch := 0
		var errPatch error
		if match[4] != "" {
			gotPatch, errPatch = strconv.Atoi(match[4])
		}
		if errMajor != nil || errMinor != nil || errPatch != nil {
			continue
		}
		// Hosted image tags such as opencsghq/vllm:2.5 are packaging versions,
		// not upstream vLLM semver. Treat major > 0 as an upstream vLLM version
		// only when the tag explicitly uses a v-prefixed semver token.
		if gotMajor > major && !explicitVersionPrefix {
			continue
		}
		if gotMajor != major {
			capable = capable || gotMajor > major
			continue
		}
		if gotMinor != minor {
			capable = capable || gotMinor > minor
			continue
		}
		if gotPatch != patch {
			capable = capable || gotPatch >= patch
			continue
		}
		capable = true
	}
	return capable
}

func isSGLangNativeResponsesCapable(runtime, image string) bool {
	for _, value := range []string{runtime, image} {
		if versionAtLeast(sglangVersionPattern, value, 0, 5, 0) {
			return true
		}
	}
	return false
}

var sglangVersionPattern = regexp.MustCompile(`(?i)(?:^|[/_:])sglang[:_]?v?([0-9]+)\.([0-9]+)(?:\.([0-9]+))?`)

func versionAtLeast(pattern *regexp.Regexp, value string, major, minor, patch int) bool {
	matches := pattern.FindAllStringSubmatch(value, -1)
	capable := false
	for _, match := range matches {
		if len(match) != 4 {
			continue
		}
		gotMajor, errMajor := strconv.Atoi(match[1])
		gotMinor, errMinor := strconv.Atoi(match[2])
		gotPatch := 0
		var errPatch error
		if match[3] != "" {
			gotPatch, errPatch = strconv.Atoi(match[3])
		}
		if errMajor != nil || errMinor != nil || errPatch != nil {
			continue
		}
		if gotMajor != major {
			capable = capable || gotMajor > major
			continue
		}
		if gotMinor != minor {
			capable = capable || gotMinor > minor
			continue
		}
		if gotPatch != patch {
			capable = capable || gotPatch >= patch
			continue
		}
		capable = true
	}
	return capable
}

func appendEndpointPath(base *url.URL, segments ...string) string {
	u := *base
	u.Path = commonutils.JoinURLPath(u.Path, segments...)
	u.RawQuery = ""
	u.Fragment = ""
	return u.String()
}
