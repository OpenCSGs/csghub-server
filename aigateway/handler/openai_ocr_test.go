package handler

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/aigateway/token"
	"opencsg.com/csghub-server/aigateway/types"
	"opencsg.com/csghub-server/common/errorx"
	commontypes "opencsg.com/csghub-server/common/types"
)

const ocrTestUpstreamBody = `{
  "logId": "log-1",
  "errorCode": 0,
  "errorMsg": "Success",
  "result": {
    "ocrResults": [
      {
        "prunedResult": {
          "rec_texts": ["hello", "world"],
          "rec_scores": [0.99, 0.87],
          "rec_polys": [
            [[1, 2], [50, 2], [50, 20], [1, 20]],
            [[1, 30], [60, 30], [60, 48], [1, 48]]
          ]
        }
      }
    ]
  }
}`

func newMultipartOCRRequest(t *testing.T, model, fileContent, fileContentType string, fields map[string]string, fileCount int) *http.Request {
	t.Helper()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if model != "" {
		require.NoError(t, writer.WriteField("model", model))
	}
	for key, value := range fields {
		require.NoError(t, writer.WriteField(key, value))
	}
	for i := 0; i < fileCount; i++ {
		header := make(textproto.MIMEHeader)
		header.Set("Content-Disposition", `form-data; name="file"; filename="input.png"`)
		if fileContentType != "" {
			header.Set("Content-Type", fileContentType)
		}
		part, err := writer.CreatePart(header)
		require.NoError(t, err)
		_, err = part.Write([]byte(fileContent))
		require.NoError(t, err)
	}
	require.NoError(t, writer.Close())

	req := httptest.NewRequest(http.MethodPost, "/v1/ocr", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req
}

// withCancelableContext mimics a real server request: production requests
// carry a cancelable context, which makes the reverse proxy skip its
// CloseNotifier path (that path panics on httptest.ResponseRecorder).
func withCancelableContext(t *testing.T, req *http.Request) *http.Request {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	return req.WithContext(ctx)
}

func ocrTestModelWithEndpoint(id, upstreamModelName, endpoint string) *types.Model {
	return &types.Model{
		BaseModel: types.BaseModel{
			ID:      id,
			Object:  "model",
			OwnedBy: "testuser",
			Task:    string(commontypes.OpticalCharacterRecognition),
		},
		InternalModelInfo: types.InternalModelInfo{
			RuntimeFramework: "paddleocr",
		},
		Endpoint: endpoint,
		Upstreams: []commontypes.UpstreamConfig{
			{URL: endpoint, Enabled: true, ModelName: upstreamModelName},
		},
	}
}

func TestSupportsOCRTask(t *testing.T) {
	assert.False(t, supportsOCRTask(nil))
	assert.True(t, supportsOCRTask(&types.Model{
		BaseModel: types.BaseModel{Task: "optical-character-recognition"},
	}))
	assert.True(t, supportsOCRTask(&types.Model{
		BaseModel: types.BaseModel{Task: "text-generation, optical-character-recognition"},
	}))
	assert.True(t, supportsOCRTask(&types.Model{
		InternalModelInfo: types.InternalModelInfo{RuntimeFramework: "paddleocr"},
	}))
	assert.True(t, supportsOCRTask(&types.Model{
		ExternalModelInfo: types.ExternalModelInfo{Provider: "opencsg"},
	}))
	assert.False(t, supportsOCRTask(&types.Model{
		BaseModel:         types.BaseModel{Task: "text-generation"},
		InternalModelInfo: types.InternalModelInfo{RuntimeFramework: "vllm"},
	}))
}

func TestOpenAIHandler_OCRValidation(t *testing.T) {
	t.Run("invalid multipart body", func(t *testing.T) {
		tester, c, w := setupTest(t)
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/ocr", strings.NewReader("not multipart"))
		c.Request.Header.Set("Content-Type", "multipart/form-data; boundary=missing")

		tester.handler.OCR(c)

		require.Equal(t, http.StatusBadRequest, w.Code)
		require.Contains(t, w.Body.String(), "invalid_request_error")
	})

	t.Run("missing model", func(t *testing.T) {
		tester, c, w := setupTest(t)
		c.Request = newMultipartOCRRequest(t, "", "image-bytes", "image/png", nil, 1)

		tester.handler.OCR(c)

		require.Equal(t, http.StatusBadRequest, w.Code)
		require.Contains(t, w.Body.String(), "Model cannot be empty")
	})

	t.Run("missing file", func(t *testing.T) {
		tester, c, w := setupTest(t)
		c.Request = newMultipartOCRRequest(t, "model1", "", "", nil, 0)

		tester.handler.OCR(c)

		require.Equal(t, http.StatusBadRequest, w.Code)
		require.Contains(t, w.Body.String(), "File cannot be empty")
	})

	t.Run("multiple files", func(t *testing.T) {
		tester, c, w := setupTest(t)
		c.Request = newMultipartOCRRequest(t, "model1", "image-bytes", "image/png", nil, 2)

		tester.handler.OCR(c)

		require.Equal(t, http.StatusBadRequest, w.Code)
		require.Contains(t, w.Body.String(), "Only one file is allowed")
	})

	t.Run("pdf rejected", func(t *testing.T) {
		tester, c, w := setupTest(t)
		c.Request = newMultipartOCRRequest(t, "model1", "pdf-bytes", "application/pdf", nil, 1)

		tester.handler.OCR(c)

		require.Equal(t, http.StatusBadRequest, w.Code)
		require.Contains(t, w.Body.String(), "PDF input is not supported yet")
	})

	t.Run("unsupported content type", func(t *testing.T) {
		tester, c, w := setupTest(t)
		c.Request = newMultipartOCRRequest(t, "model1", "gif-bytes", "image/gif", nil, 1)

		tester.handler.OCR(c)

		require.Equal(t, http.StatusBadRequest, w.Code)
		require.Contains(t, w.Body.String(), "Unsupported file content type")
	})

	t.Run("oversized file rejected by request body limit", func(t *testing.T) {
		tester, c, w := setupTest(t)
		c.Request = newMultipartOCRRequest(t, "model1", strings.Repeat("a", int(maxOCRRequestSize)+1), "image/png", nil, 1)

		tester.handler.OCR(c)

		require.Equal(t, http.StatusBadRequest, w.Code)
		require.Contains(t, w.Body.String(), "invalid multipart form")
	})

	t.Run("oversized unrelated field rejected by request body limit", func(t *testing.T) {
		tester, c, w := setupTest(t)
		fields := map[string]string{"junk": strings.Repeat("a", int(maxOCRRequestSize))}
		c.Request = newMultipartOCRRequest(t, "model1", "image-bytes", "image/png", fields, 1)

		tester.handler.OCR(c)

		require.Equal(t, http.StatusBadRequest, w.Code)
		require.Contains(t, w.Body.String(), "invalid multipart form")
	})
}

func TestOpenAIHandler_OCRPreProxyErrors(t *testing.T) {
	t.Run("model not found", func(t *testing.T) {
		tester, c, w := setupTest(t)
		c.Request = newMultipartOCRRequest(t, "missing-model", "image-bytes", "image/png", nil, 1)
		tester.mocks.openAIComp.EXPECT().GetModelByID(mock.Anything, "testuser", "missing-model").Return(nil, nil).Once()

		tester.handler.OCR(c)

		require.Equal(t, http.StatusBadRequest, w.Code)
		require.Contains(t, w.Body.String(), "model_not_found")
	})

	t.Run("model is not OCR-capable", func(t *testing.T) {
		tester, c, w := setupTest(t)
		c.Request = newMultipartOCRRequest(t, "model1", "image-bytes", "image/png", nil, 1)
		chatModel := &types.Model{
			BaseModel:         types.BaseModel{ID: "model1", Object: "model", Task: "text-generation"},
			InternalModelInfo: types.InternalModelInfo{RuntimeFramework: "vllm"},
			Endpoint:          "https://api.example.com",
			Upstreams: []commontypes.UpstreamConfig{
				{URL: "https://api.example.com", Enabled: true, ModelName: "backend-model"},
			},
		}
		tester.mocks.openAIComp.EXPECT().GetModelByID(mock.Anything, "testuser", "model1").Return(chatModel, nil).Once()

		tester.handler.OCR(c)

		require.Equal(t, http.StatusBadRequest, w.Code)
		require.Contains(t, w.Body.String(), "unsupported_model")
	})

	t.Run("insufficient balance", func(t *testing.T) {
		tester, c, w := setupTest(t)
		c.Request = newMultipartOCRRequest(t, "model1", "image-bytes", "image/png", nil, 1)
		tester.mocks.openAIComp.EXPECT().GetModelByID(mock.Anything, "testuser", "model1").
			Return(ocrTestModelWithEndpoint("model1", "paddleocr-model", "https://api.example.com"), nil).Once()
		tester.mocks.openAIComp.EXPECT().CheckBalance(mock.Anything, "testuuid").Return(errorx.ErrInsufficientBalance).Once()

		tester.handler.OCR(c)

		require.Equal(t, http.StatusForbidden, w.Code)
	})
}

func TestOpenAIHandler_OCRUpstreamErrorPassthrough(t *testing.T) {
	tester, c, w := setupTest(t)
	var gotPath string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error": "runtime exploded"}`))
	}))
	defer upstream.Close()

	c.Request = withCancelableContext(t, newMultipartOCRRequest(t, "model1", "image-bytes", "image/png", nil, 1))
	tester.mocks.openAIComp.EXPECT().GetModelByID(mock.Anything, "testuser", "model1").
		Return(ocrTestModelWithEndpoint("model1", "paddleocr-model", upstream.URL), nil).Once()
	tester.mocks.openAIComp.EXPECT().CheckBalance(mock.Anything, "testuuid").Return(nil).Once()

	tester.handler.OCR(c)

	require.Equal(t, "/ocr", gotPath)
	require.Equal(t, http.StatusInternalServerError, w.Code)
	require.Contains(t, w.Body.String(), "runtime exploded")
}

func TestOpenAIHandler_OCRHappyPath(t *testing.T) {
	tester, c, w := setupTest(t)

	var (
		gotPath        string
		gotMethod      string
		gotContentType string
		gotBody        []byte
	)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotMethod = r.Method
		gotContentType = r.Header.Get("Content-Type")
		gotBody, _ = io.ReadAll(r.Body)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(ocrTestUpstreamBody))
	}))
	defer upstream.Close()

	c.Request = withCancelableContext(t, newMultipartOCRRequest(t, "model1", "image-bytes", "image/png", map[string]string{
		"use_doc_orientation_classify": "true",
	}, 1))
	model := ocrTestModelWithEndpoint("model1", "paddleocr-model", upstream.URL)
	tester.mocks.openAIComp.EXPECT().GetModelByID(mock.Anything, "testuser", "model1").Return(model, nil).Once()
	tester.mocks.openAIComp.EXPECT().CheckBalance(mock.Anything, "testuuid").Return(nil).Once()

	var wg sync.WaitGroup
	wg.Add(1)
	tester.mocks.openAIComp.EXPECT().
		RecordUsageFromTokenUsage(mock.Anything, "testuuid", model, "paddleocr-model", mock.MatchedBy(func(usage *token.Usage) bool {
			return usage != nil &&
				usage.DataType == string(commontypes.DataTypeOCR) &&
				usage.CompletionRC == 1
		}), mock.Anything).
		RunAndReturn(func(context.Context, string, *types.Model, string, *token.Usage, string) error {
			wg.Done()
			return nil
		}).Once()

	tester.handler.OCR(c)

	// upstream received a valid PaddleX request
	require.Equal(t, "/ocr", gotPath)
	require.Equal(t, http.MethodPost, gotMethod)
	require.Equal(t, "application/json", gotContentType)
	var payload map[string]any
	require.NoError(t, json.Unmarshal(gotBody, &payload))
	decoded, err := base64.StdEncoding.DecodeString(payload["file"].(string))
	require.NoError(t, err)
	require.Equal(t, "image-bytes", string(decoded))
	require.EqualValues(t, 1, payload["fileType"])
	require.Equal(t, true, payload["useDocOrientationClassify"])
	require.Equal(t, false, payload["visualize"])
	require.NotContains(t, payload, "useDocUnwarping")

	require.Equal(t, http.StatusOK, w.Code)

	var resp types.OCRResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, types.OCRResponseObject, resp.Object)
	assert.Equal(t, "model1", resp.Model)
	assert.Equal(t, "hello\nworld", resp.Text)
	require.Len(t, resp.Pages, 1)
	require.Len(t, resp.Pages[0].Lines, 2)
	assert.Equal(t, "hello", resp.Pages[0].Lines[0].Text)
	require.NotNil(t, resp.Pages[0].Lines[0].Score)
	assert.Equal(t, 1, resp.Usage.Pages)
	assert.Equal(t, 1, resp.Usage.Images)
	assert.Nil(t, resp.RawResult)

	wg.Wait()
}

func TestOpenAIHandler_OCRRawResponse(t *testing.T) {
	tester, c, w := setupTest(t)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(ocrTestUpstreamBody))
	}))
	defer upstream.Close()

	c.Request = withCancelableContext(t, newMultipartOCRRequest(t, "model1", "image-bytes", "image/png", map[string]string{
		"raw_response": "true",
	}, 1))
	model := ocrTestModelWithEndpoint("model1", "paddleocr-model", upstream.URL)
	tester.mocks.openAIComp.EXPECT().GetModelByID(mock.Anything, "testuser", "model1").Return(model, nil).Once()
	tester.mocks.openAIComp.EXPECT().CheckBalance(mock.Anything, "testuuid").Return(nil).Once()
	tester.mocks.openAIComp.EXPECT().
		RecordUsageFromTokenUsage(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil).Maybe()

	tester.handler.OCR(c)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"raw_result"`)
	assert.Contains(t, w.Body.String(), `"logId"`)
}
