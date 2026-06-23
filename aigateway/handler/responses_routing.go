package handler

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

type responsesRoutingDecision struct {
	Mode      ResponsesExecutionMode
	NativeURL string
	Reason    string
}

func resolveResponsesRouting(modelTarget *resolvedModelTarget) (responsesRoutingDecision, error) {
	if modelTarget == nil || modelTarget.Model == nil {
		return responsesRoutingDecision{}, fmt.Errorf("model target is nil")
	}
	target := strings.TrimSpace(modelTarget.Target)
	parsed, err := url.Parse(target)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return responsesRoutingDecision{}, fmt.Errorf("cannot resolve responses mode from upstream url %q", target)
	}

	path := strings.TrimRight(parsed.Path, "/")
	switch {
	case pathEndsWithSegments(path, "responses"):
		return responsesRoutingDecision{Mode: ResponsesModeNative, NativeURL: target, Reason: "upstream_url_responses"}, nil
	case pathEndsWithSegments(path, "chat", "completions"):
		return responsesRoutingDecision{Mode: ResponsesModeChatAdapter, Reason: "upstream_url_chat_completions"}, nil
	default:
		return responsesRoutingDecision{Mode: ResponsesModeDisabled, Reason: "unsupported_upstream_url"}, nil
	}
}

func pathEndsWithSegments(path string, segments ...string) bool {
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
