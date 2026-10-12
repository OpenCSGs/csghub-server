package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseModelScope(t *testing.T) {
	t.Run("empty scope parses to nil", func(t *testing.T) {
		assert.Nil(t, ParseModelScope(""))
		assert.Nil(t, ParseModelScope("   "))
	})

	t.Run("comma separated ids are trimmed", func(t *testing.T) {
		assert.Equal(t, []string{"model-a", "model-b"}, ParseModelScope("model-a, model-b"))
	})

	t.Run("empty entries are dropped", func(t *testing.T) {
		assert.Equal(t, []string{"model-a"}, ParseModelScope(", model-a,, "))
	})
}

func TestModelInScope(t *testing.T) {
	t.Run("empty scope allows every model", func(t *testing.T) {
		assert.True(t, ModelInScope(nil, "any-model"))
	})

	t.Run("exact match required", func(t *testing.T) {
		scope := []string{"model-a", "model-b"}
		assert.True(t, ModelInScope(scope, "model-a"))
		assert.False(t, ModelInScope(scope, "model-c"))
		assert.False(t, ModelInScope(scope, "MODEL-A"))
	})
}
