package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/aigateway/token"
)

func TestSpeechAudioResponseWriterCapturesDurationHeader(t *testing.T) {
	counter := token.NewAudioUsageCounter(nil)
	counter.Text("ocean waves")
	recorder := httptest.NewRecorder()
	recorder.Header().Set(speechAudioDurationHeader, "2.5")
	writer := NewResponseWriterWrapperSpeech(recorder, counter)

	writer.WriteHeader(http.StatusOK)
	_, err := writer.Write([]byte("RIFF-audio"))
	require.NoError(t, err)

	usage, err := counter.Usage(context.Background())
	require.NoError(t, err)
	require.Equal(t, 2.5, usage.Duration)
}

func TestSpeechBatchResponseWriterSumsDuration(t *testing.T) {
	counter := token.NewAudioUsageCounter(nil)
	recorder := httptest.NewRecorder()
	writer := NewResponseWriterWrapperSpeechBatch(recorder, counter)

	_, err := writer.Write([]byte(`{
		"results": [
			{"status":"success","usage":{"input_tokens":5,"total_tokens":5,"seconds":1.25}},
			{"status":"error","usage":{"input_tokens":9,"total_tokens":9,"seconds":9}},
			{"status":"success","usage":{"input_tokens":4,"total_tokens":4,"seconds":0.75}}
		]
	}`))
	require.NoError(t, err)

	usage, err := counter.Usage(context.Background())
	require.NoError(t, err)
	require.Equal(t, 2.0, usage.Duration)
	require.Equal(t, int64(9), usage.TotalTokens)
	require.Equal(t, int64(2), usage.CompletionRC)
}
