package text2video

import (
	"bytes"
	"fmt"
	"io"
	"mime/multipart"
	"net/textproto"
	"strconv"
	"strings"
)

func parseOpenAIVideoSize(size string) (int, int, bool) {
	size = strings.TrimSpace(strings.ToLower(size))
	if size == "" {
		return 0, 0, false
	}
	parts := strings.SplitN(size, "x", 2)
	if len(parts) != 2 {
		return 0, 0, false
	}
	width, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil || width <= 0 {
		return 0, 0, false
	}
	height, err := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err != nil || height <= 0 {
		return 0, 0, false
	}
	return width, height, true
}

func buildMultipartBody(write func(writer *multipart.Writer) error) ([]byte, string, error) {
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	if err := write(writer); err != nil {
		_ = writer.Close()
		return nil, "", err
	}
	if err := writer.Close(); err != nil {
		return nil, "", err
	}
	return buf.Bytes(), writer.FormDataContentType(), nil
}

func writeMultipartValues(writer *multipart.Writer, form *multipart.Form, skipKeys map[string]struct{}) error {
	if form == nil {
		return nil
	}
	for key, values := range form.Value {
		if _, skip := skipKeys[key]; skip {
			continue
		}
		for _, value := range values {
			if err := writer.WriteField(key, value); err != nil {
				return err
			}
		}
	}
	for key, files := range form.File {
		if _, skip := skipKeys[key]; skip {
			continue
		}
		for _, fileHeader := range files {
			if err := copyMultipartFileField(writer, key, fileHeader); err != nil {
				return err
			}
		}
	}
	return nil
}

func copyMultipartFileField(writer *multipart.Writer, fieldName string, fileHeader *multipart.FileHeader) error {
	if fileHeader == nil {
		return fmt.Errorf("multipart file header is nil")
	}
	file, err := fileHeader.Open()
	if err != nil {
		return err
	}
	defer file.Close()

	header := make(textproto.MIMEHeader)
	header.Set("Content-Disposition", fmt.Sprintf(`form-data; name="%s"; filename="%s"`, escapeMultipartQuotes(fieldName), escapeMultipartQuotes(fileHeader.Filename)))
	if contentType := strings.TrimSpace(fileHeader.Header.Get("Content-Type")); contentType != "" {
		header.Set("Content-Type", contentType)
	}
	part, err := writer.CreatePart(header)
	if err != nil {
		return err
	}
	_, err = io.Copy(part, file)
	return err
}

func escapeMultipartQuotes(value string) string {
	return strings.ReplaceAll(value, `"`, `\"`)
}
