package wrapper

import (
	"bufio"
	"bytes"
	"encoding/json"
	"net"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/aigateway/component/adapter/ocr"
	"opencsg.com/csghub-server/aigateway/token"
	"opencsg.com/csghub-server/aigateway/types"
)

// OCR buffers the upstream OCR runtime response, then normalizes it into the
// AIGateway OCR response shape in Finalize. Upstream error responses (status
// >= 400) are passed through unchanged.
type OCR struct {
	internalWritter gin.ResponseWriter
	adapter         ocr.Adapter
	counter         *token.OCRUsageCounter
	opts            *ocr.ResponseOptions
	buffer          bytes.Buffer
	statusCode      int
	contentType     string
	headerWritten   bool
	response        *types.OCRResponse
}

func NewOCR(
	w gin.ResponseWriter,
	adapter ocr.Adapter,
	counter *token.OCRUsageCounter,
	opts *ocr.ResponseOptions,
) *OCR {
	return &OCR{
		internalWritter: w,
		adapter:         adapter,
		counter:         counter,
		opts:            opts,
		statusCode:      http.StatusOK,
	}
}

func (w *OCR) Header() http.Header {
	return w.internalWritter.Header()
}

func (w *OCR) WriteHeader(statusCode int) {
	w.statusCode = statusCode
	w.contentType = w.internalWritter.Header().Get("Content-Type")
}

func (w *OCR) Write(data []byte) (int, error) {
	return w.buffer.Write(data)
}

func (w *OCR) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return w.internalWritter.Hijack()
}

func (w *OCR) CloseNotify() <-chan bool {
	return w.internalWritter.CloseNotify()
}

func (w *OCR) Flush() {
	w.internalWritter.Flush()
}

func (w *OCR) Finalize() error {
	if w.headerWritten {
		return nil
	}
	w.headerWritten = true

	body := w.buffer.Bytes()
	if w.statusCode >= http.StatusBadRequest {
		// Pass upstream errors through unchanged.
		h := w.internalWritter.Header()
		if w.contentType != "" {
			h.Set("Content-Type", w.contentType)
		}
		h.Set("Content-Length", strconv.Itoa(len(body)))
		w.internalWritter.WriteHeader(w.statusCode)
		_, err := w.internalWritter.Write(body)
		return err
	}

	ocrResp, err := w.adapter.TransformResponse(body, w.opts)
	if err != nil {
		w.statusCode = http.StatusInternalServerError
		w.internalWritter.Header().Set("Content-Type", "application/json")
		w.internalWritter.WriteHeader(http.StatusInternalServerError)
		return json.NewEncoder(w.internalWritter).Encode(gin.H{"error": types.Error{
			Code:    "upstream_response_error",
			Message: err.Error(),
			Type:    "internal_error",
		}})
	}

	if w.counter != nil {
		w.counter.OCRResponse(ocrResp)
	}
	w.response = ocrResp

	out, err := json.Marshal(ocrResp)
	if err != nil {
		w.statusCode = http.StatusInternalServerError
		w.internalWritter.Header().Set("Content-Type", "application/json")
		w.internalWritter.WriteHeader(http.StatusInternalServerError)
		return err
	}

	h := w.internalWritter.Header()
	h.Set("Content-Type", "application/json")
	h.Del("Content-Encoding")
	h.Set("Content-Length", strconv.Itoa(len(out)))
	w.internalWritter.WriteHeader(w.statusCode)
	_, err = w.internalWritter.Write(out)
	return err
}

func (w *OCR) StatusCode() int {
	if w == nil || w.statusCode == 0 {
		return http.StatusOK
	}
	return w.statusCode
}

func (w *OCR) Response() *types.OCRResponse {
	if w == nil {
		return nil
	}
	return w.response
}
