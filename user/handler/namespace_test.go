package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/common/types"
)

type namespaceComponentStub struct {
	getInfoByUUIDs func(ctx context.Context, uuids []string) ([]*types.Namespace, error)
}

func (s *namespaceComponentStub) GetInfo(context.Context, string) (*types.Namespace, error) {
	return nil, nil
}

func (s *namespaceComponentStub) GetInfoByUUID(context.Context, string) (*types.Namespace, error) {
	return nil, nil
}

func (s *namespaceComponentStub) GetInfoByUUIDs(ctx context.Context, uuids []string) ([]*types.Namespace, error) {
	return s.getInfoByUUIDs(ctx, uuids)
}

func TestNamespaceHandler_GetInfoByUUIDs(t *testing.T) {
	gin.SetMode(gin.TestMode)

	const namespaceUUID = "11111111-1111-4111-8111-111111111111"

	call := func(t *testing.T, body string, returned []*types.Namespace) (*httptest.ResponseRecorder, []string) {
		t.Helper()
		gin.SetMode(gin.TestMode)
		var gotUUIDs []string
		h := &NamespaceHandler{
			maxNamespaceLookupUUIDs: 100,
			c: &namespaceComponentStub{
				getInfoByUUIDs: func(_ context.Context, uuids []string) ([]*types.Namespace, error) {
					gotUUIDs = append([]string(nil), uuids...)
					return returned, nil
				},
			},
		}
		response := httptest.NewRecorder()
		ginCtx, _ := gin.CreateTestContext(response)
		ginCtx.Request = httptest.NewRequest(http.MethodPost, "/api/v1/namespaces/lookup", bytes.NewBufferString(body))
		ginCtx.Request.Header.Set("Content-Type", "application/json")
		h.GetInfoByUUIDs(ginCtx)
		return response, gotUUIDs
	}

	responseCode := func(t *testing.T, response *httptest.ResponseRecorder) string {
		t.Helper()
		var body httpbase.R
		require.NoError(t, json.Unmarshal(response.Body.Bytes(), &body))
		return body.Code
	}

	t.Run("looks up unique valid uuids", func(t *testing.T) {
		response, gotUUIDs := call(t, `["`+namespaceUUID+`"]`, []*types.Namespace{{Path: "user1", Type: "user", UUID: namespaceUUID}})
		require.Equal(t, http.StatusOK, response.Code)
		require.Equal(t, []string{namespaceUUID}, gotUUIDs)
	})

	t.Run("deduplicates repeated uuids", func(t *testing.T) {
		body := `["` + namespaceUUID + `","` + namespaceUUID + `"]`
		response, gotUUIDs := call(t, body, []*types.Namespace{{Path: "user1", Type: "user", UUID: namespaceUUID}})
		require.Equal(t, http.StatusOK, response.Code)
		require.Equal(t, []string{namespaceUUID}, gotUUIDs)
	})

	t.Run("rejects malformed json", func(t *testing.T) {
		response, _ := call(t, `["`+namespaceUUID+`"`, nil)
		require.Equal(t, http.StatusBadRequest, response.Code)
		require.Equal(t, "REQ-ERR-1", responseCode(t, response))
	})

	t.Run("rejects empty array", func(t *testing.T) {
		response, _ := call(t, `[]`, nil)
		require.Equal(t, http.StatusBadRequest, response.Code)
		require.Equal(t, "REQ-ERR-2", responseCode(t, response))
	})

	t.Run("rejects null body", func(t *testing.T) {
		response, _ := call(t, `null`, nil)
		require.Equal(t, http.StatusBadRequest, response.Code)
		require.Equal(t, "REQ-ERR-2", responseCode(t, response))
	})

	t.Run("rejects empty uuid entry", func(t *testing.T) {
		response, _ := call(t, `[""]`, nil)
		require.Equal(t, http.StatusBadRequest, response.Code)
		require.Equal(t, "REQ-ERR-6", responseCode(t, response))
	})

	t.Run("rejects null uuid entry", func(t *testing.T) {
		response, _ := call(t, `[null]`, nil)
		require.Equal(t, http.StatusBadRequest, response.Code)
		require.Equal(t, "REQ-ERR-6", responseCode(t, response))
	})

	t.Run("rejects invalid uuid format", func(t *testing.T) {
		response, _ := call(t, `["not-a-uuid"]`, nil)
		require.Equal(t, http.StatusBadRequest, response.Code)
		require.Equal(t, "REQ-ERR-6", responseCode(t, response))
	})

	t.Run("rejects more than max uuids", func(t *testing.T) {
		uuids := make([]string, 101)
		for i := range uuids {
			uuids[i] = namespaceUUID
		}
		response, _ := call(t, `["`+strings.Join(uuids, `","`)+`"]`, nil)
		require.Equal(t, http.StatusBadRequest, response.Code)
		require.Equal(t, "REQ-ERR-6", responseCode(t, response))
	})
}
