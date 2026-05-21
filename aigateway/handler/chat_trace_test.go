package handler

import (
	"net/http"
	"strings"
	"testing"
)

func TestExtractChatSessionIDPrecedence(t *testing.T) {
	headers := http.Header{}
	headers.Set(sessionHeaderConvID, "conversation")
	headers.Set(sessionHeaderSessionID, "session")
	headers.Set(sessionHeaderClaudeCode, "claude")

	if got := extractChatSessionID(headers); got != "claude" {
		t.Fatalf("expected claude session id, got %q", got)
	}

	headers.Del(sessionHeaderClaudeCode)
	if got := extractChatSessionID(headers); got != "session" {
		t.Fatalf("expected x-session id, got %q", got)
	}

	headers.Del(sessionHeaderSessionID)
	if got := extractChatSessionID(headers); got != "conversation" {
		t.Fatalf("expected conversation id, got %q", got)
	}
}

func TestExtractChatSessionIDTruncatesLongValue(t *testing.T) {
	longValue := strings.Repeat("a", maxSessionKeyLength+10)
	headers := http.Header{}
	headers.Set(sessionHeaderSessionID, longValue)

	got := extractChatSessionID(headers)
	if len(got) != maxSessionKeyLength {
		t.Fatalf("expected truncated session length %d, got %d", maxSessionKeyLength, len(got))
	}
}

func TestExtractChatSessionIDMissing(t *testing.T) {
	if got := extractChatSessionID(http.Header{}); got != "" {
		t.Fatalf("expected empty session id, got %q", got)
	}
}
