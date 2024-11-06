package checker

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_getSensitiveWordList(t *testing.T) {
	words := getSensitiveWordList(`5pWP5oSf6K+NLHNlbnNpdGl2ZXdvcmQ=`)

	assert.Equal(t, 2, len(words))
	assert.Equal(t, "敏感词", words[0])
	assert.Equal(t, "sensitiveword", words[1])
}
