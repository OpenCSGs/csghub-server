package types

import (
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOwnedRequestBodyDoesNotCopyOrConsume(t *testing.T) {
	raw := []byte(strings.Repeat("x", 1<<20))
	body := NewRequestBody(raw)
	got, err := ReadRequestBody(body)
	require.NoError(t, err)
	require.Equal(t, &raw[0], &got[0])
	require.Zero(t, testing.AllocsPerRun(5, func() { _, _ = ReadRequestBody(body) }))
	forwarded, err := io.ReadAll(body)
	require.NoError(t, err)
	require.Equal(t, raw, forwarded)
}
