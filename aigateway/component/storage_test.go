package component

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewStorage(t *testing.T) {
	t.Run("nil config returns nil", func(t *testing.T) {
		s, err := NewStorage(nil)
		assert.NoError(t, err)
		assert.Nil(t, s)
	})
}
