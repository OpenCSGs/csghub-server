package sample

import (
	"bytes"
	"encoding/binary"
	"io"
	"mime"
	"mime/multipart"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMultipartFactories(t *testing.T) {
	tests := []struct {
		name, fileField, filename, contentType string
		factory                                requestFactory
		fields                                 map[string]string
		fileData                               []byte
	}{
		{name: "image edit", factory: imageEditsRequest, fields: map[string]string{"model": "model", "prompt": "hi", "response_format": "b64_json"}, fileField: "image", filename: "sample.png", contentType: "image/png", fileData: tinyPNG},
		{name: "transcription", factory: transcriptionsRequest, fields: map[string]string{"model": "model"}, fileField: "file", filename: "sample.wav", contentType: "audio/wav", fileData: sampleWAV},
		{name: "voice upload", factory: voiceUploadRequest, fields: map[string]string{"model": "model", "name": "sample-voice", "consent": "consent-id"}, fileField: "audio_sample", filename: "sample.wav", contentType: "audio/wav", fileData: sampleWAV},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request, err := test.factory(sampleInput("https://api.example.com/v1/test", "model"))
			require.NoError(t, err)
			require.Equal(t, "Bearer test-token", request.Headers.Get("Authorization"))
			mediaType, parameters, err := mime.ParseMediaType(request.Headers.Get("Content-Type"))
			require.NoError(t, err)
			require.Equal(t, "multipart/form-data", mediaType)
			reader := multipart.NewReader(bytes.NewReader(request.Body), parameters["boundary"])
			seenFields := map[string]string{}
			for {
				part, err := reader.NextPart()
				if err == io.EOF {
					break
				}
				require.NoError(t, err)
				data, err := io.ReadAll(part)
				require.NoError(t, err)
				if part.FormName() == test.fileField {
					require.Equal(t, test.filename, part.FileName())
					require.Equal(t, test.contentType, part.Header.Get("Content-Type"))
					require.Equal(t, test.fileData, data)
					continue
				}
				seenFields[part.FormName()] = string(data)
			}
			require.Equal(t, test.fields, seenFields)
		})
	}
}

func TestSampleWAVIsLongEnoughForASRFeatureExtraction(t *testing.T) {
	require.Equal(t, "RIFF", string(sampleWAV[0:4]))
	require.Equal(t, "WAVE", string(sampleWAV[8:12]))
	require.Equal(t, uint16(1), binary.LittleEndian.Uint16(sampleWAV[20:22]))
	require.Equal(t, uint16(1), binary.LittleEndian.Uint16(sampleWAV[22:24]))
	require.Equal(t, uint32(sampleWAVSampleRate), binary.LittleEndian.Uint32(sampleWAV[24:28]))
	require.Equal(t, uint16(16), binary.LittleEndian.Uint16(sampleWAV[34:36]))
	require.Equal(t, uint32(sampleWAVSampleCount*sampleWAVBytesPerSample), binary.LittleEndian.Uint32(sampleWAV[40:44]))
	require.Len(t, sampleWAV, sampleWAVHeaderSize+sampleWAVSampleCount*sampleWAVBytesPerSample)
	require.NotEqual(t, make([]byte, len(sampleWAV)-sampleWAVHeaderSize), sampleWAV[sampleWAVHeaderSize:])
}

func TestMultipartBuildsHaveIndependentBodies(t *testing.T) {
	input := sampleInput("https://api.example.com/v1/images/edits", "model")
	first, err := imageEditsRequest(input)
	require.NoError(t, err)
	second, err := imageEditsRequest(input)
	require.NoError(t, err)
	require.NotEqual(t, first.Headers.Get("Content-Type"), second.Headers.Get("Content-Type"))
	require.NotEqual(t, first.Body, second.Body)
}

func TestMultipartL7PreservesHeaders(t *testing.T) {
	request, err := modelsL7Request(imageEditsRoute)(sampleInput("https://api.example.com/v1/images/edits", "model"))
	require.NoError(t, err)
	require.Equal(t, "Bearer test-token", request.Headers.Get("Authorization"))
}
