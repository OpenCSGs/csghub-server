package types

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestApplyRequestAuthHeaders_JSON(t *testing.T) {
	header := http.Header{}
	err := ApplyRequestAuthHeaders(header, `{"Authorization":"Bearer provider-token","X-Test":"ok"}`)
	require.NoError(t, err)
	require.Equal(t, "Bearer provider-token", header.Get("Authorization"))
	require.Equal(t, "ok", header.Get("X-Test"))
}

func TestApplyRequestAuthHeaders_BearerString(t *testing.T) {
	header := http.Header{}
	err := ApplyRequestAuthHeaders(header, "Bearer provider-token")
	require.NoError(t, err)
	require.Equal(t, "Bearer provider-token", header.Get("Authorization"))
}

func TestApplyRequestAuthHeaders_Invalid(t *testing.T) {
	header := http.Header{}
	err := ApplyRequestAuthHeaders(header, "not-json")
	require.Error(t, err)
	require.Empty(t, header.Get("Authorization"))
}

func TestApplyRequestAuthHeaders_Empty(t *testing.T) {
	header := http.Header{}
	err := ApplyRequestAuthHeaders(header, "")
	require.NoError(t, err)
	require.Empty(t, header.Get("Authorization"))
}
