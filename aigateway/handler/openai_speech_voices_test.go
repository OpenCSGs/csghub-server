package handler

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/aigateway/types"
	commontypes "opencsg.com/csghub-server/common/types"
)

func newVoicesTestModel(serverURL string) *types.Model {
	return &types.Model{
		BaseModel: types.BaseModel{
			ID:       "backend-model",
			Object:   "model",
			Metadata: map[string]any{},
			Task:     "text-to-speech",
		},
		Endpoint: serverURL + "/v1/audio/speech",
		Upstreams: []commontypes.UpstreamConfig{
			{URL: serverURL + "/v1/audio/speech", Enabled: true, ModelName: "backend-model"},
		},
	}
}

func newMultipartVoiceRequest(t *testing.T, method, model string) *http.Request {
	t.Helper()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if model != "" {
		require.NoError(t, writer.WriteField("model", model))
	}
	require.NoError(t, writer.WriteField("name", "my-voice"))
	require.NoError(t, writer.WriteField("consent", "consent-id"))
	part, err := writer.CreateFormFile("audio_sample", "sample.wav")
	require.NoError(t, err)
	_, err = part.Write([]byte("audio-bytes"))
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	req := httptest.NewRequest(method, "/v1/audio/voices", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req
}

func TestOpenAIHandler_ListVoices(t *testing.T) {
	t.Run("successful proxy", func(t *testing.T) {
		tester, c, w := setupTest(t)

		voicesJSON := `{"voices":[{"name":"vivian"}]}`
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, http.MethodGet, r.Method)
			require.Equal(t, "/v1/audio/voices", r.URL.Path)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(voicesJSON))
		}))
		defer server.Close()

		c.Request = httptest.NewRequest(http.MethodGet, "/v1/audio/voices?model=model1", nil)
		model := newVoicesTestModel(server.URL)
		tester.mocks.openAIComp.EXPECT().GetModelByID(mock.Anything, "testuser", "model1").Return(model, nil).Once()

		tester.handler.ListVoices(c)

		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		require.Equal(t, voicesJSON, w.Body.String())
	})

	t.Run("missing model", func(t *testing.T) {
		tester, c, w := setupTest(t)
		c.Request = httptest.NewRequest(http.MethodGet, "/v1/audio/voices", nil)

		tester.handler.ListVoices(c)

		require.Equal(t, http.StatusBadRequest, w.Code)
		require.Contains(t, w.Body.String(), "Model cannot be empty")
	})
}

func TestOpenAIHandler_UploadVoice(t *testing.T) {
	t.Run("owner can upload with rewritten model", func(t *testing.T) {
		tester, c, w := setupTest(t)

		var downstreamModel string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, http.MethodPost, r.Method)
			require.Equal(t, "/v1/audio/voices", r.URL.Path)
			require.NoError(t, r.ParseMultipartForm(32<<20))
			downstreamModel = r.FormValue("model")
			require.Equal(t, "my-voice", r.FormValue("name"))
			file, _, err := r.FormFile("audio_sample")
			require.NoError(t, err)
			content, err := io.ReadAll(file)
			require.NoError(t, err)
			require.Equal(t, "audio-bytes", string(content))
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"status":"success"}`))
		}))
		defer server.Close()

		c.Request = newMultipartVoiceRequest(t, http.MethodPost, "model1")
		model := newVoicesTestModel(server.URL)
		tester.mocks.openAIComp.EXPECT().GetModelByID(mock.Anything, "testuser", "model1").Return(model, nil).Once()
		tester.mocks.openAIComp.EXPECT().CanManageModel(mock.Anything, "testuser", "testuuid", model).Return(true, nil).Once()

		tester.handler.UploadVoice(c)

		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		require.Equal(t, "backend-model", downstreamModel)
	})

	t.Run("non-owner is forbidden", func(t *testing.T) {
		tester, c, w := setupTest(t)
		c.Request = newMultipartVoiceRequest(t, http.MethodPost, "model1")
		model := newVoicesTestModel("https://api.example.com")
		tester.mocks.openAIComp.EXPECT().GetModelByID(mock.Anything, "testuser", "model1").Return(model, nil).Once()
		tester.mocks.openAIComp.EXPECT().CanManageModel(mock.Anything, "testuser", "testuuid", model).Return(false, nil).Once()

		tester.handler.UploadVoice(c)

		require.Equal(t, http.StatusForbidden, w.Code)
		require.Contains(t, w.Body.String(), "insufficient_permissions")
	})

	t.Run("oversized upload rejected", func(t *testing.T) {
		tester, c, w := setupTest(t)

		var body bytes.Buffer
		writer := multipart.NewWriter(&body)
		require.NoError(t, writer.WriteField("model", "model1"))
		part, err := writer.CreateFormFile("audio_sample", "sample.wav")
		require.NoError(t, err)
		_, err = part.Write(bytes.Repeat([]byte("a"), maxVoiceUploadBytes+1))
		require.NoError(t, err)
		require.NoError(t, writer.Close())
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/audio/voices", &body)
		c.Request.Header.Set("Content-Type", writer.FormDataContentType())

		tester.handler.UploadVoice(c)

		require.Equal(t, http.StatusRequestEntityTooLarge, w.Code)
		require.Contains(t, w.Body.String(), "request body too large")
	})

	t.Run("missing audio sample", func(t *testing.T) {
		tester, c, w := setupTest(t)
		var body bytes.Buffer
		writer := multipart.NewWriter(&body)
		require.NoError(t, writer.WriteField("model", "model1"))
		require.NoError(t, writer.Close())
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/audio/voices", &body)
		c.Request.Header.Set("Content-Type", writer.FormDataContentType())

		tester.handler.UploadVoice(c)

		require.Equal(t, http.StatusBadRequest, w.Code)
		require.Contains(t, w.Body.String(), "audio_sample cannot be empty")
	})
}

func TestOpenAIHandler_UpdateVoice(t *testing.T) {
	t.Run("update proxies as POST to backend", func(t *testing.T) {
		tester, c, w := setupTest(t)

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// The backend voice upload endpoint overwrites by name and only
			// accepts POST.
			require.Equal(t, http.MethodPost, r.Method)
			require.Equal(t, "/v1/audio/voices", r.URL.Path)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"status":"success"}`))
		}))
		defer server.Close()

		c.Request = newMultipartVoiceRequest(t, http.MethodPut, "model1")
		model := newVoicesTestModel(server.URL)
		tester.mocks.openAIComp.EXPECT().GetModelByID(mock.Anything, "testuser", "model1").Return(model, nil).Once()
		tester.mocks.openAIComp.EXPECT().CanManageModel(mock.Anything, "testuser", "testuuid", model).Return(true, nil).Once()

		tester.handler.UpdateVoice(c)

		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	})
}

func TestOpenAIHandler_DeleteVoice(t *testing.T) {
	t.Run("owner can delete voice by name", func(t *testing.T) {
		tester, c, w := setupTest(t)

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, http.MethodDelete, r.Method)
			require.Equal(t, "/v1/audio/voices/my-voice", r.URL.Path)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"status":"success"}`))
		}))
		defer server.Close()

		c.Request = httptest.NewRequest(http.MethodDelete, "/v1/audio/voices/my-voice?model=model1", nil)
		c.Params = gin.Params{{Key: "name", Value: "my-voice"}}
		model := newVoicesTestModel(server.URL)
		tester.mocks.openAIComp.EXPECT().GetModelByID(mock.Anything, "testuser", "model1").Return(model, nil).Once()
		tester.mocks.openAIComp.EXPECT().CanManageModel(mock.Anything, "testuser", "testuuid", model).Return(true, nil).Once()

		tester.handler.DeleteVoice(c)

		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	})

	t.Run("non-owner is forbidden", func(t *testing.T) {
		tester, c, w := setupTest(t)
		c.Request = httptest.NewRequest(http.MethodDelete, "/v1/audio/voices/my-voice?model=model1", nil)
		c.Params = gin.Params{{Key: "name", Value: "my-voice"}}
		model := newVoicesTestModel("https://api.example.com")
		tester.mocks.openAIComp.EXPECT().GetModelByID(mock.Anything, "testuser", "model1").Return(model, nil).Once()
		tester.mocks.openAIComp.EXPECT().CanManageModel(mock.Anything, "testuser", "testuuid", model).Return(false, nil).Once()

		tester.handler.DeleteVoice(c)

		require.Equal(t, http.StatusForbidden, w.Code)
		require.Contains(t, w.Body.String(), "insufficient_permissions")
	})

	t.Run("missing voice name", func(t *testing.T) {
		tester, c, w := setupTest(t)
		c.Request = httptest.NewRequest(http.MethodDelete, "/v1/audio/voices/x?model=model1", nil)
		c.Params = gin.Params{{Key: "name", Value: " "}}

		tester.handler.DeleteVoice(c)

		require.Equal(t, http.StatusBadRequest, w.Code)
		require.Contains(t, w.Body.String(), "Voice name cannot be empty")
	})
}

func TestVoicesProxyPath(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/audio/voices", nil)

	require.Equal(t, "", voicesProxyPath(c, ""))
	require.Equal(t, "", voicesProxyPath(c, "https://host"))
	require.Equal(t, "", voicesProxyPath(c, "https://host/"))
	require.Equal(t, "/v1/audio/voices", voicesProxyPath(c, "https://host/v1/audio/speech"))
	require.Equal(t, "/custom/path", voicesProxyPath(c, "https://host/custom/path"))
}

func TestBatchSpeechRequest_JSONRoundTrip(t *testing.T) {
	body := `{"model":"m","items":[{"input":"hi","voice":"vivian"},{"input":"there"}],"response_format":"wav","non_streaming_mode":true}`
	var req BatchSpeechRequest
	require.NoError(t, json.Unmarshal([]byte(body), &req))
	require.Equal(t, "m", req.Model)
	require.Len(t, req.Items, 2)
	require.Equal(t, []string{"hi", "there"}, req.InputTexts())

	req.Model = "backend"
	out, err := json.Marshal(req)
	require.NoError(t, err)
	var m map[string]any
	require.NoError(t, json.Unmarshal(out, &m))
	require.Equal(t, "backend", m["model"])
	require.Equal(t, "wav", m["response_format"])
	require.Equal(t, true, m["non_streaming_mode"])
	items, ok := m["items"].([]any)
	require.True(t, ok)
	require.Len(t, items, 2)
	first, ok := items[0].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "vivian", first["voice"])
}
