package responses

import (
	"fmt"
	"net/url"
	"strings"
)

type ResponsesExecutionMode string

const (
	ResponsesModeNative      ResponsesExecutionMode = "native"
	ResponsesModeChatAdapter ResponsesExecutionMode = "chat_adapter"
	ResponsesModeDisabled    ResponsesExecutionMode = "disabled"
)

type RoutingDecision struct {
	Mode      ResponsesExecutionMode
	NativeURL string
	Reason    string
}

type RoutingTarget struct {
	ModelID string
	Target  string
}

func ResolveRouting(modelTarget RoutingTarget) (RoutingDecision, error) {
	if strings.TrimSpace(modelTarget.ModelID) == "" {
		return RoutingDecision{}, fmt.Errorf("model target is nil")
	}
	target := strings.TrimSpace(modelTarget.Target)
	parsed, err := url.Parse(target)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return RoutingDecision{}, fmt.Errorf("cannot resolve responses mode from upstream url %q", target)
	}

	path := strings.TrimRight(parsed.Path, "/")
	switch {
	case PathEndsWithSegments(path, "responses"):
		return RoutingDecision{Mode: ResponsesModeNative, NativeURL: target, Reason: "upstream_url_responses"}, nil
	case PathEndsWithSegments(path, "chat", "completions"):
		return RoutingDecision{Mode: ResponsesModeChatAdapter, Reason: "upstream_url_chat_completions"}, nil
	default:
		return RoutingDecision{Mode: ResponsesModeDisabled, Reason: "unsupported_upstream_url"}, nil
	}
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
