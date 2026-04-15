package handler

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/aigateway/types"
)

func TestApplyModelAuthHeaders_JSON(t *testing.T) {
	header := http.Header{}
	model := &types.Model{
		ExternalModelInfo: types.ExternalModelInfo{
			AuthHead: `{"Authorization":"Bearer provider-token","X-Test":"ok"}`,
		},
	}

	require.NoError(t, applyModelAuthHeaders(header, model))
	require.Equal(t, "Bearer provider-token", header.Get("Authorization"))
	require.Equal(t, "ok", header.Get("X-Test"))
}

func TestApplyModelAuthHeaders_BearerString(t *testing.T) {
	header := http.Header{}
	model := &types.Model{
		ExternalModelInfo: types.ExternalModelInfo{
			AuthHead: "Bearer provider-token",
		},
	}

	require.NoError(t, applyModelAuthHeaders(header, model))
	require.Equal(t, "Bearer provider-token", header.Get("Authorization"))
}

func TestApplyModelAuthHeaders_Invalid(t *testing.T) {
	header := http.Header{}
	model := &types.Model{
		ExternalModelInfo: types.ExternalModelInfo{
			AuthHead: "not-json",
		},
	}

	require.Error(t, applyModelAuthHeaders(header, model))
	require.Empty(t, header.Get("Authorization"))
}
