package handler

import (
	"fmt"
	"io"
	"mime/multipart"
	"net/textproto"
	"strings"
)

func rewriteMultipartModelStream(form *multipart.Form, modelName string) (io.ReadCloser, string) {
	body, contentType, _ := rewriteMultipartModelStreamWithOptions(form, modelName, multipartRewriteOptions{})
	return body, contentType
}

type multipartRewriteOptions struct {
	defaultFields   map[string]string
	normalizeFields map[string]func(string) string
}

func rewriteMultipartModelStreamWithOptions(form *multipart.Form, modelName string, options multipartRewriteOptions) (io.ReadCloser, string, error) {
	if form == nil {
		return nil, "", fmt.Errorf("multipart form is empty")
	}

	reader, writer := io.Pipe()
	multipartWriter := multipart.NewWriter(writer)
	contentType := multipartWriter.FormDataContentType()

	go func() {
		err := writeMultipartModel(form, modelName, multipartWriter, options)
		if closeErr := multipartWriter.Close(); err == nil {
			err = closeErr
		}
		if err != nil {
			_ = writer.CloseWithError(err)
			return
		}
		_ = writer.Close()
	}()

	return reader, contentType, nil
}

func firstMultipartValue(form *multipart.Form, key string) string {
	if form == nil || len(form.Value[key]) == 0 {
		return ""
	}
	return form.Value[key][0]
}

func writeMultipartModel(form *multipart.Form, modelName string, writer *multipart.Writer, options multipartRewriteOptions) error {
	wroteFields := map[string]bool{}
	for key, vals := range form.Value {
		if key == "model" {
			continue
		}
		for _, val := range vals {
			if normalize := options.normalizeFields[key]; normalize != nil {
				val = normalize(val)
			}
			if err := writer.WriteField(key, val); err != nil {
				return err
			}
		}
		wroteFields[key] = true
	}
	if err := writer.WriteField("model", modelName); err != nil {
		return err
	}
	for key, val := range options.defaultFields {
		if wroteFields[key] {
			continue
		}
		if normalize := options.normalizeFields[key]; normalize != nil {
			val = normalize(val)
		}
		if err := writer.WriteField(key, val); err != nil {
			return err
		}
	}

	for fieldName, files := range form.File {
		for _, fileHeader := range files {
			if err := copyMultipartFile(writer, fieldName, fileHeader); err != nil {
				return err
			}
		}
	}
	return nil
}

func copyMultipartFile(writer *multipart.Writer, fieldName string, fileHeader *multipart.FileHeader) error {
	file, err := fileHeader.Open()
	if err != nil {
		return err
	}
	defer file.Close()

	header := textproto.MIMEHeader{}
	header.Set("Content-Disposition", fmt.Sprintf(`form-data; name="%s"; filename="%s"`, escapeMultipartValue(fieldName), escapeMultipartValue(fileHeader.Filename)))
	contentType := fileHeader.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	header.Set("Content-Type", contentType)
	part, err := writer.CreatePart(header)
	if err != nil {
		return err
	}
	_, err = io.Copy(part, file)
	return err
}

func escapeMultipartValue(value string) string {
	return strings.NewReplacer("\\", "\\\\", `"`, "\\\"").Replace(value)
}
