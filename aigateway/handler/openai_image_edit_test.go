package handler

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
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
