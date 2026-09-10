package handler

import (
	"io"
	"mime/multipart"
	"net/url"
	"strings"
)

const maxImageEditMultipartMemory = 128 << 20

func hasMultipartFile(form *multipart.Form, key string) bool {
	if form == nil {
		return false
	}
	return len(form.File[key]) > 0
}

func imageEditResponseFormat(form *multipart.Form) string {
	if form != nil {
		if values := form.Value["response_format"]; len(values) > 0 && strings.TrimSpace(values[0]) != "" {
			return strings.TrimSpace(values[0])
		}
	}
	return "b64_json"
}

func rebuildImageEditMultipartBody(form *multipart.Form, modelName string) (io.ReadCloser, string, error) {
	return rewriteMultipartModelStreamWithOptions(form, modelName, multipartRewriteOptions{
		defaultFields: map[string]string{
			"response_format": "b64_json",
		},
		normalizeFields: map[string]func(string) string{
			"response_format": func(value string) string {
				if strings.TrimSpace(value) == "" {
					return "b64_json"
				}
				return value
			},
		},
	})
}

func imageEditProxyPath(endpoint string) string {
	if endpoint == "" {
		return ""
	}
	uri, err := url.ParseRequestURI(endpoint)
	if err != nil {
		return ""
	}
	if uri.Path == "" {
		return "/"
	}
	return uri.Path
}
