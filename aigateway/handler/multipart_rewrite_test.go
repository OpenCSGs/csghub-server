package handler

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRewriteMultipartModelStream(t *testing.T) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	require.NoError(t, writer.WriteField("model", "public-model"))
	require.NoError(t, writer.WriteField("prompt", "meeting"))
	part, err := writer.CreateFormFile("file", `sample "quote".wav`)
	require.NoError(t, err)
	_, err = part.Write([]byte("audio-bytes"))
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	req := httptest.NewRequest("POST", "/v1/audio/transcriptions", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	form, err := req.MultipartReader()
	require.NoError(t, err)
	parsedForm, err := form.ReadForm(32 << 20)
	require.NoError(t, err)

	bodyReader, contentType := rewriteMultipartModelStream(parsedForm, "backend-model")
	rewrittenBody, err := io.ReadAll(bodyReader)
	require.NoError(t, err)

	req = httptest.NewRequest("POST", "/v1/audio/transcriptions", bytes.NewReader(rewrittenBody))
	req.Header.Set("Content-Type", contentType)
	require.NoError(t, req.ParseMultipartForm(32<<20))

	require.Equal(t, "backend-model", req.FormValue("model"))
	require.Equal(t, "meeting", req.FormValue("prompt"))
	file, header, err := req.FormFile("file")
	require.NoError(t, err)
	defer file.Close()
	data, err := io.ReadAll(file)
	require.NoError(t, err)
	require.Equal(t, "audio-bytes", string(data))
	require.Equal(t, `sample "quote".wav`, header.Filename)
}

func TestFirstMultipartValue(t *testing.T) {
	require.Empty(t, firstMultipartValue(nil, "model"))
	require.Empty(t, firstMultipartValue(&multipart.Form{}, "model"))
	require.Equal(t, "model1", firstMultipartValue(&multipart.Form{
		Value: map[string][]string{"model": {"model1", "model2"}},
	}, "model"))
}
