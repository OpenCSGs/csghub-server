package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestProtocolString(t *testing.T) {
	tests := []struct {
		protocol Protocol
		expected string
	}{
		{ProtocolChat, "chat"},
		{ProtocolResponses, "responses"},
		{ProtocolMessages, "messages"},
	}
	for _, tt := range tests {
		t.Run(string(tt.protocol), func(t *testing.T) {
			assert.Equal(t, tt.expected, string(tt.protocol))
		})
	}
}

func TestDefaultProtocolCapabilities(t *testing.T) {
	t.Run("chat capabilities", func(t *testing.T) {
		cap := CapabilityFor(ProtocolChat)
		assert.True(t, cap.Streaming)
		assert.True(t, cap.Tools)
		assert.True(t, cap.Vision)
		assert.True(t, cap.Thinking)
		assert.False(t, cap.PromptCaching)
		assert.True(t, cap.StructuredOutput)
	})

	t.Run("responses capabilities", func(t *testing.T) {
		cap := CapabilityFor(ProtocolResponses)
		assert.True(t, cap.Streaming)
		assert.True(t, cap.Tools)
		assert.True(t, cap.Vision)
		assert.True(t, cap.Thinking)
		assert.False(t, cap.PromptCaching)
		assert.True(t, cap.StructuredOutput)
	})

	t.Run("messages capabilities", func(t *testing.T) {
		cap := CapabilityFor(ProtocolMessages)
		assert.True(t, cap.Streaming)
		assert.True(t, cap.Tools)
		assert.True(t, cap.Vision)
		assert.True(t, cap.Thinking)
		assert.True(t, cap.PromptCaching)
		assert.False(t, cap.StructuredOutput)
	})
}

func TestCapabilityForUnknownProtocol(t *testing.T) {
	cap := CapabilityFor(Protocol("unknown"))
	assert.Equal(t, Protocol("unknown"), cap.Protocol)
	assert.False(t, cap.Streaming)
	assert.False(t, cap.Tools)
}
