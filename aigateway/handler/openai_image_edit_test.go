package handler

import (
	"bytes"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/aigateway/types"
	"opencsg.com/csghub-server/builder/rpc"
	"opencsg.com/csghub-server/common/errorx"
	commontypes "opencsg.com/csghub-server/common/types"
)

func TestRebuildImageEditMultipartBody(t *testing.T) {
	var original bytes.Buffer
	writer := multipart.NewWriter(&original)
	require.NoError(t, writer.WriteField("model", "public-model"))
	require.NoError(t, writer.WriteField("prompt", "make it brighter"))
	part, err := writer.CreateFormFile("image", "input.png")
	require.NoError(t, err)
	_, err = part.Write([]byte("png-bytes"))
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	req, err := http.NewRequest(http.MethodPost, "/v1/images/edits", &original)
	require.NoError(t, err)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	require.NoError(t, req.ParseMultipartForm(maxImageEditMultipartMemory))

	bodyReader, contentType, err := rebuildImageEditMultipartBody(req.MultipartForm, "downstream-model")
	require.NoError(t, err)
	defer bodyReader.Close()
	require.Contains(t, contentType, "multipart/form-data")
	body, err := io.ReadAll(bodyReader)
	require.NoError(t, err)

	rebuilt, err := http.NewRequest(http.MethodPost, "/v1/images/edits", bytes.NewReader(body))
	require.NoError(t, err)
	rebuilt.Header.Set("Content-Type", contentType)
	require.NoError(t, rebuilt.ParseMultipartForm(maxImageEditMultipartMemory))

	require.Equal(t, "downstream-model", rebuilt.FormValue("model"))
	require.Equal(t, "make it brighter", rebuilt.FormValue("prompt"))
	require.Equal(t, "b64_json", rebuilt.FormValue("response_format"))
	files := rebuilt.MultipartForm.File["image"]
	require.Len(t, files, 1)
	file, err := files[0].Open()
	require.NoError(t, err)
	defer file.Close()
	data, err := io.ReadAll(file)
	require.NoError(t, err)
	require.Equal(t, "png-bytes", string(data))
}

func TestImageEditProxyPath(t *testing.T) {
	require.Equal(t, "", imageEditProxyPath(""))
	require.Equal(t, "/", imageEditProxyPath("https://svc.example.com"))
	require.Equal(t, "/v1/images/edits", imageEditProxyPath("https://api.example.com/v1/images/edits"))
	require.Equal(t, "", imageEditProxyPath(strings.Repeat("%", 3)))
}

func TestOpenAIHandler_EditImageValidation(t *testing.T) {
	t.Run("invalid multipart body", func(t *testing.T) {
		tester, c, w := setupTest(t)
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/edits", strings.NewReader("not multipart"))
		c.Request.Header.Set("Content-Type", "multipart/form-data; boundary=missing")

		tester.handler.EditImage(c)

		require.Equal(t, http.StatusBadRequest, w.Code)
		require.Contains(t, w.Body.String(), "invalid_request_error")
	})

	t.Run("missing model or prompt", func(t *testing.T) {
		tester, c, w := setupTest(t)
		c.Request = newMultipartImageEditRequest(t, "", "make it brighter", "image-bytes", nil)

		tester.handler.EditImage(c)

		require.Equal(t, http.StatusBadRequest, w.Code)
		require.Contains(t, w.Body.String(), "Model and prompt cannot be empty")
	})

	t.Run("missing image", func(t *testing.T) {
		tester, c, w := setupTest(t)
		c.Request = newMultipartImageEditRequest(t, "model1", "make it brighter", "", nil)

		tester.handler.EditImage(c)

		require.Equal(t, http.StatusBadRequest, w.Code)
		require.Contains(t, w.Body.String(), "Image is required")
	})
}

func TestOpenAIHandler_EditImagePreProxyErrors(t *testing.T) {
	t.Run("model not found", func(t *testing.T) {
		tester, c, w := setupTest(t)
		c.Request = newMultipartImageEditRequest(t, "missing-model", "make it brighter", "image-bytes", nil)
		tester.mocks.openAIComp.EXPECT().GetModelByID(mock.Anything, "testuser", "missing-model").Return(nil, nil).Once()

		tester.handler.EditImage(c)

		require.Equal(t, http.StatusBadRequest, w.Code)
		require.Contains(t, w.Body.String(), "model_not_found")
	})

	t.Run("insufficient balance", func(t *testing.T) {
		tester, c, w := setupTest(t)
		c.Request = newMultipartImageEditRequest(t, "model1", "make it brighter", "image-bytes", nil)
		tester.mocks.openAIComp.EXPECT().GetModelByID(mock.Anything, "testuser", "model1").Return(imageEditTestModel("model1", "backend-model"), nil).Once()
		tester.mocks.openAIComp.EXPECT().CheckBalance(mock.Anything, "testuuid").Return(errorx.ErrInsufficientBalance).Once()

		tester.handler.EditImage(c)

		require.Equal(t, http.StatusForbidden, w.Code)
		require.Contains(t, w.Body.String(), "ACT-ERR-0")
	})

	t.Run("sensitive prompt", func(t *testing.T) {
		tester, c, w := setupTest(t)
		c.Request = newMultipartImageEditRequest(t, "model1", "sensitive prompt", "image-bytes", nil)
		tester.mocks.openAIComp.EXPECT().GetModelByID(mock.Anything, "testuser", "model1").Return(imageEditTestModel("model1", "backend-model"), nil).Once()
		tester.mocks.openAIComp.EXPECT().CheckBalance(mock.Anything, "testuuid").Return(nil).Once()
		tester.mocks.moderationComp.EXPECT().CheckImagePrompts(mock.Anything, "sensitive prompt", "testuuid").Return(&rpc.CheckResult{IsSensitive: true}, nil).Once()

		tester.handler.EditImage(c)

		require.Equal(t, http.StatusBadRequest, w.Code)
		require.Contains(t, w.Body.String(), "content_policy_violation")
	})

	t.Run("moderation error", func(t *testing.T) {
		tester, c, w := setupTest(t)
		c.Request = newMultipartImageEditRequest(t, "model1", "make it brighter", "image-bytes", nil)
		tester.mocks.openAIComp.EXPECT().GetModelByID(mock.Anything, "testuser", "model1").Return(imageEditTestModel("model1", "backend-model"), nil).Once()
		tester.mocks.openAIComp.EXPECT().CheckBalance(mock.Anything, "testuuid").Return(nil).Once()
		tester.mocks.moderationComp.EXPECT().CheckImagePrompts(mock.Anything, "make it brighter", "testuuid").Return(nil, errors.New("moderation unavailable")).Once()

		tester.handler.EditImage(c)

		require.Equal(t, http.StatusInternalServerError, w.Code)
		require.Contains(t, w.Body.String(), "moderation_error")
	})

	t.Run("skip balance model", func(t *testing.T) {
		tester, c, w := setupTest(t)
		c.Request = newMultipartImageEditRequest(t, "guard-model", "make it brighter", "image-bytes", nil)
		model := imageEditTestModel("guard-model", "backend-model")
		model.Metadata = map[string]any{
			types.MetaTaskKey: []interface{}{types.MetaTaskValGuard},
		}
		tester.mocks.openAIComp.EXPECT().GetModelByID(mock.Anything, "testuser", "guard-model").Return(model, nil).Once()
		tester.mocks.moderationComp.EXPECT().CheckImagePrompts(mock.Anything, "make it brighter", "testuuid").Return(nil, errors.New("moderation unavailable")).Once()

		tester.handler.EditImage(c)

		require.Equal(t, http.StatusInternalServerError, w.Code)
		require.Contains(t, w.Body.String(), "moderation_error")
	})
}

func TestOpenAIHandler_EditImageUpstreamError(t *testing.T) {
	tester, c, w := setupTest(t)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1/images/edits", r.URL.Path)
		require.Equal(t, http.MethodPost, r.Method)
		require.Contains(t, r.Header.Get("Content-Type"), "multipart/form-data")
		require.NoError(t, r.ParseMultipartForm(maxImageEditMultipartMemory))
		require.Equal(t, "backend-model", r.FormValue("model"))
		require.Equal(t, "make it brighter", r.FormValue("prompt"))

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"created":0,"data":[]}`))
	}))
	defer upstream.Close()

	c.Request = newMultipartImageEditRequest(t, "model1", "make it brighter", "image-bytes", map[string]string{
		"size":            "1024x1024",
		"response_format": "b64_json",
		"output_format":   "png",
	})
	tester.mocks.openAIComp.EXPECT().GetModelByID(mock.Anything, "testuser", "model1").Return(imageEditTestModelWithEndpoint("model1", "backend-model", upstream.URL+"/v1/images/edits"), nil).Once()
	tester.mocks.openAIComp.EXPECT().CheckBalance(mock.Anything, "testuuid").Return(nil).Once()
	tester.mocks.moderationComp.EXPECT().CheckImagePrompts(mock.Anything, "make it brighter", "testuuid").Return(&rpc.CheckResult{IsSensitive: false}, nil).Once()

	tester.handler.EditImage(c)

	require.Equal(t, http.StatusInternalServerError, w.Code)
	time.Sleep(20 * time.Millisecond)
}

func newMultipartImageEditRequest(t *testing.T, model, prompt, imageContent string, fields map[string]string) *http.Request {
	t.Helper()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if model != "" {
		require.NoError(t, writer.WriteField("model", model))
	}
	if prompt != "" {
		require.NoError(t, writer.WriteField("prompt", prompt))
	}
	for key, value := range fields {
		require.NoError(t, writer.WriteField(key, value))
	}
	if imageContent != "" {
		part, err := writer.CreateFormFile("image", "input.png")
		require.NoError(t, err)
		_, err = part.Write([]byte(imageContent))
		require.NoError(t, err)
	}
	require.NoError(t, writer.Close())

	req := httptest.NewRequest(http.MethodPost, "/v1/images/edits", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req
}

func imageEditTestModel(id, upstreamModelName string) *types.Model {
	return imageEditTestModelWithEndpoint(id, upstreamModelName, "https://api.example.com/v1/images/edits")
}

func imageEditTestModelWithEndpoint(id, upstreamModelName, endpoint string) *types.Model {
	return &types.Model{
		BaseModel: types.BaseModel{
			ID:      id,
			Object:  "model",
			OwnedBy: "testuser",
			Task:    "image-to-image",
		},
		Endpoint: endpoint,
		Upstreams: []commontypes.UpstreamConfig{
			{URL: endpoint, Enabled: true, ModelName: upstreamModelName},
		},
	}
}
