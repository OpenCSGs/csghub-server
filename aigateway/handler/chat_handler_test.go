package handler

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
)

func TestTruncateChatResponseBodyForLog(t *testing.T) {
	tests := []struct {
		name     string
		body     string
		expected string
	}{
		{
			name:     "empty body returned as is",
			body:     "",
			expected: "",
		},
		{
			name:     "body at exactly the limit returned as is",
			body:     strings.Repeat("a", chatResponseBodyLogLimit),
			expected: strings.Repeat("a", chatResponseBodyLogLimit),
		},
		{
			name:     "body one byte over the limit is truncated",
			body:     strings.Repeat("a", chatResponseBodyLogLimit) + "b",
			expected: strings.Repeat("a", chatResponseBodyLogLimit) + "...(truncated)",
		},
		{
			name: "cut point inside a multibyte rune backs off to the rune start",
			// "中" is 3 bytes; the limit falls on its 2nd byte.
			body:     strings.Repeat("a", chatResponseBodyLogLimit-1) + "中",
			expected: strings.Repeat("a", chatResponseBodyLogLimit-1) + "...(truncated)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncateChatResponseBodyForLog(tt.body)
			assert.Equal(t, tt.expected, got)
			assert.True(t, utf8.ValidString(got), "truncated log body must stay valid UTF-8")
		})
	}
}
