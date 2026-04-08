package wrapper

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/aigateway/component"
	"opencsg.com/csghub-server/aigateway/component/adapter/text2image"
	"opencsg.com/csghub-server/aigateway/token"
	"opencsg.com/csghub-server/aigateway/types"
	"opencsg.com/csghub-server/common/utils/common"
)

type ImageGeneration struct {
	internalWritter     gin.ResponseWriter
	adapter             text2image.T2IAdapter
	moderationComponent component.Moderation
	sensitiveDefaultImg string
	imageCounter        *token.ImageUsageCounter
	responseFormat      string
	storage             types.Storage
	bucket              string
	buffer              bytes.Buffer
	statusCode          int
	contentType         string
	headerWritten       bool
}

func NewImageGeneration(
	w gin.ResponseWriter,
	adapter text2image.T2IAdapter,
	moderationComponent component.Moderation,
	sensitiveDefaultImg string,
	imageCounter *token.ImageUsageCounter,
	responseFormat string,
	storage types.Storage,
	bucket string,
) *ImageGeneration {
	return &ImageGeneration{
		internalWritter:     w,
		adapter:             adapter,
		moderationComponent: moderationComponent,
		sensitiveDefaultImg: sensitiveDefaultImg,
		imageCounter:        imageCounter,
		responseFormat:      responseFormat,
		storage:             storage,
		bucket:              bucket,
		statusCode:          http.StatusOK,
	}
}

func (w *ImageGeneration) Header() http.Header {
	return w.internalWritter.Header()
}

func (w *ImageGeneration) WriteHeader(statusCode int) {
	w.statusCode = statusCode
	w.contentType = w.internalWritter.Header().Get("Content-Type")
}

func (w *ImageGeneration) Write(data []byte) (int, error) {
	return w.buffer.Write(data)
}

func (w *ImageGeneration) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return w.internalWritter.Hijack()
}

func (w *ImageGeneration) CloseNotify() <-chan bool {
	return w.internalWritter.CloseNotify()
}

func (w *ImageGeneration) Flush() {
	w.internalWritter.Flush()
}

func (w *ImageGeneration) Finalize() error {
	if w.headerWritten {
		return nil
	}
	w.headerWritten = true

	encodingHeader := w.internalWritter.Header().Get("Content-Encoding")
	var opts *types.TransformResponseOptions
	if w.responseFormat != "" || w.storage != nil {
		opts = &types.TransformResponseOptions{
			ResponseFormat: w.responseFormat,
			Storage:        w.storage,
			Bucket:         w.bucket,
		}
	}
	body, openaiResp, err := w.adapter.TransformResponse(context.Background(), w.buffer.Bytes(), w.contentType, encodingHeader, opts)
	if err != nil {
		w.internalWritter.Header().Set("Content-Type", "application/json")
		w.internalWritter.WriteHeader(http.StatusInternalServerError)
		return json.NewEncoder(w.internalWritter).Encode(gin.H{"error": types.Error{
			Code:    "transform_error",
			Message: err.Error(),
			Type:    "internal_error",
		}})
	}

	if w.imageCounter != nil {
		w.imageCounter.ImageResponse(openaiResp)
	}

	result, err := w.moderationComponent.CheckImage(context.Background(), *openaiResp)
	if err != nil {
		slog.Error("image moderation check failed", slog.Any("error", err))
	} else if result != nil && result.IsSensitive {
		if len(openaiResp.Data) > 0 {
			openaiResp.Data[0].URL = w.sensitiveDefaultImg
			openaiResp.Data[0].B64JSON = ""
		}
		body, err = common.MarshalJSONWithoutHTMLEscape(openaiResp)
		if err != nil {
			w.internalWritter.Header().Set("Content-Type", "application/json")
			w.internalWritter.WriteHeader(http.StatusInternalServerError)
			return err
		}
	}

	h := w.internalWritter.Header()
	h.Set("Content-Type", "application/json")
	h.Del("Content-Encoding")
	h.Set("Content-Length", strconv.Itoa(len(body)))
	w.internalWritter.WriteHeader(w.statusCode)
	_, err = w.internalWritter.Write(body)
	return err
}
