package sample

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/textproto"

	"opencsg.com/csghub-server/aigateway/types"
)

const (
	imageEditsRoute         = "/images/edits"
	transcriptionsRoute     = "/audio/transcriptions"
	voiceUploadRoute        = "/audio/voices"
	sampleWAVSampleRate     = 16000
	sampleWAVSampleCount    = sampleWAVSampleRate
	sampleWAVHeaderSize     = 44
	sampleWAVBytesPerSample = 2
)

type fileSpec struct {
	fieldName   string
	filename    string
	contentType string
	data        []byte
}

var tinyPNG = []byte{
	0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a,
	0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52,
	0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
	0x08, 0x04, 0x00, 0x00, 0x00, 0xb5, 0x1c, 0x0c,
	0x02, 0x00, 0x00, 0x00, 0x0b, 0x49, 0x44, 0x41,
	0x54, 0x78, 0xda, 0x63, 0xfc, 0xff, 0x1f, 0x00,
	0x03, 0x03, 0x02, 0x00, 0xef, 0xbf, 0x6b, 0xe5,
	0x00, 0x00, 0x00, 0x00, 0x49, 0x45, 0x4e, 0x44,
	0xae, 0x42, 0x60, 0x82,
}

// Keep the audio sample long enough for windowed ASR feature extractors.
// A header-only or single-sample WAV can decode successfully but fail before inference.
var sampleWAV = buildSampleWAV()

func buildSampleWAV() []byte {
	dataSize := sampleWAVSampleCount * sampleWAVBytesPerSample
	wav := make([]byte, sampleWAVHeaderSize+dataSize)
	copy(wav[0:4], "RIFF")
	binary.LittleEndian.PutUint32(wav[4:8], uint32(len(wav)-8))
	copy(wav[8:12], "WAVE")
	copy(wav[12:16], "fmt ")
	binary.LittleEndian.PutUint32(wav[16:20], 16)
	binary.LittleEndian.PutUint16(wav[20:22], 1)
	binary.LittleEndian.PutUint16(wav[22:24], 1)
	binary.LittleEndian.PutUint32(wav[24:28], sampleWAVSampleRate)
	binary.LittleEndian.PutUint32(wav[28:32], sampleWAVSampleRate*sampleWAVBytesPerSample)
	binary.LittleEndian.PutUint16(wav[32:34], sampleWAVBytesPerSample)
	binary.LittleEndian.PutUint16(wav[34:36], 16)
	copy(wav[36:40], "data")
	binary.LittleEndian.PutUint32(wav[40:44], uint32(dataSize))

	for index := 0; index < sampleWAVSampleCount; index++ {
		amplitude := int16(2048)
		if (index/20)%2 == 1 {
			amplitude = -amplitude
		}
		binary.LittleEndian.PutUint16(wav[sampleWAVHeaderSize+index*sampleWAVBytesPerSample:], uint16(amplitude))
	}
	return wav
}

func multipartRequest(input types.SampleInput, fields map[string]string, files []fileSpec) (*types.SampleRequest, error) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for key, value := range fields {
		if err := writer.WriteField(key, value); err != nil {
			return nil, fmt.Errorf("write multipart sample field %q: %w", key, err)
		}
	}
	for _, file := range files {
		headers := make(textproto.MIMEHeader)
		headers.Set("Content-Disposition", fmt.Sprintf(`form-data; name=%q; filename=%q`, file.fieldName, file.filename))
		headers.Set("Content-Type", file.contentType)
		part, err := writer.CreatePart(headers)
		if err != nil {
			return nil, fmt.Errorf("create multipart sample file %q: %w", file.fieldName, err)
		}
		if _, err := part.Write(file.data); err != nil {
			return nil, fmt.Errorf("write multipart sample file %q: %w", file.fieldName, err)
		}
	}
	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("close multipart sample: %w", err)
	}
	headers := cloneSampleHeaders(input.Headers)
	headers.Set("Content-Type", writer.FormDataContentType())
	return &types.SampleRequest{
		Method:   http.MethodPost,
		Endpoint: input.Endpoint,
		Headers:  headers,
		Body:     body.Bytes(),
	}, nil
}

func imageEditsRequest(input types.SampleInput) (*types.SampleRequest, error) {
	return multipartRequest(input, map[string]string{
		"model":           input.Model,
		"prompt":          sampleText(input),
		"response_format": "b64_json",
	}, []fileSpec{{fieldName: "image", filename: "sample.png", contentType: "image/png", data: tinyPNG}})
}

func transcriptionsRequest(input types.SampleInput) (*types.SampleRequest, error) {
	return multipartRequest(input, map[string]string{
		"model": input.Model,
	}, []fileSpec{{fieldName: "file", filename: "sample.wav", contentType: "audio/wav", data: sampleWAV}})
}

func voiceUploadRequest(input types.SampleInput) (*types.SampleRequest, error) {
	return multipartRequest(input, map[string]string{
		"model":   input.Model,
		"name":    "sample-voice",
		"consent": "consent-id",
	}, []fileSpec{{fieldName: "audio_sample", filename: "sample.wav", contentType: "audio/wav", data: sampleWAV}})
}
