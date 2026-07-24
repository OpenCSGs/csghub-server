package wrapper

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/aigateway/component/adapter/ocr"
	"opencsg.com/csghub-server/aigateway/token"
	"opencsg.com/csghub-server/aigateway/types"
)

const ocrUpstreamOKBody = `{
  "logId": "log-1",
  "errorCode": 0,
  "errorMsg": "Success",
  "result": {
    "ocrResults": [
      {"prunedResult": {"rec_texts": ["hello"], "rec_scores": [0.99], "rec_polys": [[[1,2],[50,2],[50,20],[1,20]]]}}
    ]
  }
}`

func newOCRTestGinWriter(recorder *httptest.ResponseRecorder) gin.ResponseWriter {
	w, _ := gin.CreateTestContext(recorder)
	return w.Writer
}

func TestOCRWrapper_TransformOnSuccess(t *testing.T) {
	recorder := httptest.NewRecorder()
	ginWriter := newOCRTestGinWriter(recorder)
	counter := token.NewOCRUsageCounter()

	w := NewOCR(ginWriter, ocr.NewPaddleXAdapter(), counter, &ocr.ResponseOptions{ModelID: "owner/m:1"})

	ginWriter.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, err := w.Write([]byte(ocrUpstreamOKBody))
	require.NoError(t, err)
	require.NoError(t, w.Finalize())

	assert.Equal(t, http.StatusOK, recorder.Code)

	var resp types.OCRResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
	assert.Equal(t, types.OCRResponseObject, resp.Object)
	assert.Equal(t, "owner/m:1", resp.Model)
	assert.Equal(t, "hello", resp.Text)
	require.Len(t, resp.Pages, 1)
	assert.Nil(t, resp.RawResult)

	usage, err := counter.Usage(t.Context())
	require.NoError(t, err)
	assert.EqualValues(t, 1, usage.CompletionRC)
	assert.Equal(t, "ocr", usage.DataType)

	assert.Equal(t, http.StatusOK, w.StatusCode())
	assert.NotNil(t, w.Response())
}

func TestOCRWrapper_PassthroughOnUpstreamError(t *testing.T) {
	recorder := httptest.NewRecorder()
	ginWriter := newOCRTestGinWriter(recorder)
	counter := token.NewOCRUsageCounter()

	w := NewOCR(ginWriter, ocr.NewPaddleXAdapter(), counter, &ocr.ResponseOptions{})

	errBody := `{"error": "upstream exploded"}`
	ginWriter.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusInternalServerError)
	_, err := w.Write([]byte(errBody))
	require.NoError(t, err)
	require.NoError(t, w.Finalize())

	assert.Equal(t, http.StatusInternalServerError, recorder.Code)
	assert.Equal(t, errBody, recorder.Body.String())

	// no usage recorded on failure
	_, err = counter.Usage(t.Context())
	require.Error(t, err)
}

func TestOCRWrapper_TransformFailure(t *testing.T) {
	recorder := httptest.NewRecorder()
	ginWriter := newOCRTestGinWriter(recorder)

	w := NewOCR(ginWriter, ocr.NewPaddleXAdapter(), token.NewOCRUsageCounter(), &ocr.ResponseOptions{})

	ginWriter.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, err := w.Write([]byte(`not-json`))
	require.NoError(t, err)
	require.NoError(t, w.Finalize())

	assert.Equal(t, http.StatusInternalServerError, recorder.Code)

	var errResp map[string]types.Error
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &errResp))
	assert.Equal(t, "upstream_response_error", errResp["error"].Code)
}

func TestOCRWrapper_UpstreamErrorCodeInBody(t *testing.T) {
	recorder := httptest.NewRecorder()
	ginWriter := newOCRTestGinWriter(recorder)

	w := NewOCR(ginWriter, ocr.NewPaddleXAdapter(), token.NewOCRUsageCounter(), &ocr.ResponseOptions{})

	ginWriter.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, err := w.Write([]byte(`{"errorCode": 101, "errorMsg": "bad image"}`))
	require.NoError(t, err)
	require.NoError(t, w.Finalize())

	// upstream returned 200 but an OCR error envelope -> treated as transform failure
	assert.Equal(t, http.StatusInternalServerError, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "upstream_response_error")
}

func TestOCRWrapper_FinalizeIdempotent(t *testing.T) {
	recorder := httptest.NewRecorder()
	ginWriter := newOCRTestGinWriter(recorder)

	w := NewOCR(ginWriter, ocr.NewPaddleXAdapter(), token.NewOCRUsageCounter(), &ocr.ResponseOptions{})

	ginWriter.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, err := w.Write([]byte(ocrUpstreamOKBody))
	require.NoError(t, err)
	require.NoError(t, w.Finalize())
	firstLen := recorder.Body.Len()
	require.NoError(t, w.Finalize())
	assert.Equal(t, firstLen, recorder.Body.Len())
}
