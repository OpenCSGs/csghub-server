package handler

import (
	"io"
	"mime/multipart"
	"strconv"
	"strings"

	"opencsg.com/csghub-server/aigateway/component/adapter/ocr"
)

const (
	maxOCRMultipartMemory = 32 << 20 // 32MB, matches EditImage
	maxOCRFileSize        = 20 << 20 // 20MB uploaded file limit
	// Bounds the whole request body (file + multipart overhead + other fields)
	// so oversized uploads are rejected before being read to disk.
	maxOCRRequestSize = maxOCRFileSize + 1<<20
)

var ocrAllowedContentTypes = map[string]struct{}{
	"image/png":       {},
	"image/jpeg":      {},
	"image/webp":      {},
	"image/bmp":       {},
	"image/tiff":      {},
	"application/pdf": {},
}

func ocrFileTypeForContentType(contentType string) (int, bool) {
	contentType = strings.ToLower(strings.TrimSpace(contentType))
	if contentType == "application/pdf" {
		return ocr.FileTypePDF, true
	}
	if _, allowed := ocrAllowedContentTypes[contentType]; allowed {
		return ocr.FileTypeImage, true
	}
	return 0, false
}

func optionalMultipartBool(form *multipart.Form, key string) *bool {
	raw := strings.TrimSpace(firstMultipartValue(form, key))
	if raw == "" {
		return nil
	}
	parsed, err := strconv.ParseBool(raw)
	if err != nil {
		return nil
	}
	return &parsed
}

func readOCRUpload(fileHeader *multipart.FileHeader) ([]byte, error) {
	f, err := fileHeader.Open()
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = f.Close()
	}()
	return io.ReadAll(f)
}
