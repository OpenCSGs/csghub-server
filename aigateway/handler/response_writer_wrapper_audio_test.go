package handler

import (
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	audioadapter "opencsg.com/csghub-server/aigateway/component/adapter/audio"
	"opencsg.com/csghub-server/aigateway/token"
)

type failingAudioResponseWriter struct {
	header http.Header
	err    error
}

func (w *failingAudioResponseWriter) Header() http.Header {
	if w.header == nil {
		w.header = http.Header{}
	}
	return w.header
}

func (w *failingAudioResponseWriter) WriteHeader(int) {}

func (w *failingAudioResponseWriter) Write([]byte) (int, error) {
	return 0, w.err
}

func TestResponseWriterWrapperAudio_UsesResponseUsage(t *testing.T) {
	counter := token.NewAudioUsageCounter(nil)
	recorder := httptest.NewRecorder()
	w := NewResponseWriterWrapperAudio(recorder, counter, false, nil)

	_, err := w.Write([]byte(`{"text":"hello","usage":{"prompt_tokens":371,"completion_tokens":52,"total_tokens":423,"seconds":9.2}}`))
	require.NoError(t, err)

	usage, err := counter.Usage(context.Background())
	require.NoError(t, err)
	require.Equal(t, int64(423), usage.TotalTokens)
	require.Equal(t, int64(371), usage.PromptTokens)
	require.Equal(t, int64(52), usage.CompletionTokens)
	require.Equal(t, 9.2, usage.Duration)
}

func TestResponseWriterWrapperAudio_UsesResponseUsageFromChunkedJSON(t *testing.T) {
	counter := token.NewAudioUsageCounter(nil)
	recorder := httptest.NewRecorder()
	w := NewResponseWriterWrapperAudio(recorder, counter, false, nil)

	_, err := w.Write([]byte(`{"text":"hello","usage":`))
	require.NoError(t, err)
	_, err = w.Write([]byte(`{"prompt_tokens":371,"completion_tokens":52,"total_tokens":423}}`))
	require.NoError(t, err)

	usage, err := counter.Usage(context.Background())
	require.NoError(t, err)
	require.Equal(t, int64(423), usage.TotalTokens)
	require.Equal(t, int64(371), usage.PromptTokens)
	require.Equal(t, int64(52), usage.CompletionTokens)
}

func TestResponseWriterWrapperAudio_CapturesDurationFromUsage(t *testing.T) {
	tests := []struct {
		name string
		body string
		want float64
	}{
		{
			name: "seconds",
			body: `{"text":"hello","usage":{"seconds":9.2}}`,
			want: 9.2,
		},
		{
			name: "top_level_duration",
			body: `{"text":"hello","duration":12.5}`,
			want: 12.5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			counter := token.NewAudioUsageCounter(nil)
			recorder := httptest.NewRecorder()
			w := NewResponseWriterWrapperAudio(recorder, counter, false, nil)

			_, err := w.Write([]byte(tt.body))
			require.NoError(t, err)

			duration, ok := w.DurationSeconds()
			require.True(t, ok)
			require.Equal(t, tt.want, duration)

			usage, err := counter.Usage(context.Background())
			require.NoError(t, err)
			require.Equal(t, tt.want, usage.Duration)
		})
	}
}

func TestResponseWriterWrapperAudio_CapturesFunASRVerboseJSONDuration(t *testing.T) {
	counter := token.NewAudioUsageCounter(nil)
	recorder := httptest.NewRecorder()
	w := NewResponseWriterWrapperAudio(recorder, counter, false, nil)

	_, err := w.Write([]byte(`{"text":"hello","segments":[{"start":0,"end":1.23,"text":"hello","speaker":null}],"language":"auto","duration":1.23,"model":"local"}`))
	require.NoError(t, err)

	duration, ok := w.DurationSeconds()
	require.True(t, ok)
	require.Equal(t, 1.23, duration)

	usage, err := counter.Usage(context.Background())
	require.NoError(t, err)
	require.Equal(t, "hello", usage.CompletionDesc)
	require.Equal(t, 1.23, usage.Duration)
}

func TestResponseWriterWrapperAudio_SkipsMissingDuration(t *testing.T) {
	counter := token.NewAudioUsageCounter(nil)
	recorder := httptest.NewRecorder()
	w := NewResponseWriterWrapperAudio(recorder, counter, false, nil)

	_, err := w.Write([]byte(`{"text":"hello","usage":{"prompt_tokens":1,"total_tokens":1}}`))
	require.NoError(t, err)

	_, ok := w.DurationSeconds()
	require.False(t, ok)
}

func TestAudioResponseWriter_NonStreamCapturesDurationFromHeader(t *testing.T) {
	counter := token.NewAudioUsageCounter(nil)
	recorder := httptest.NewRecorder()
	recorder.Header().Set("Audio-Duration-Seconds", "9.2")
	recorder.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w := NewResponseWriterWrapperAudio(recorder, counter, false, audioadapter.NewFunASRAdapter())
	w.WriteHeader(http.StatusOK)

	_, err := w.Write([]byte("hello"))
	require.NoError(t, err)

	require.Equal(t, "hello", recorder.Body.String())
	require.Equal(t, "text/plain; charset=utf-8", recorder.Header().Get("Content-Type"))
	duration, ok := w.DurationSeconds()
	require.True(t, ok)
	require.Equal(t, 9.2, duration)
	usage, err := counter.Usage(context.Background())
	require.NoError(t, err)
	require.Equal(t, 9.2, usage.Duration)
	require.Equal(t, "hello", usage.CompletionDesc)
}

func TestAudioResponseWriter_NonStreamHeaderDurationWinsOverBody(t *testing.T) {
	counter := token.NewAudioUsageCounter(nil)
	recorder := httptest.NewRecorder()
	recorder.Header().Set("Audio-Duration-Seconds", "9.2")
	w := NewResponseWriterWrapperAudio(recorder, counter, false, audioadapter.NewFunASRAdapter())
	w.WriteHeader(http.StatusOK)

	_, err := w.Write([]byte(`{"text":"hello","segments":[{"start":0,"end":1.2,"text":"hello"}],"duration":12.5}`))
	require.NoError(t, err)

	require.JSONEq(t, `{"text":"hello","segments":[{"start":0,"end":1.2,"text":"hello"}],"duration":12.5}`, recorder.Body.String())
	duration, ok := w.DurationSeconds()
	require.True(t, ok)
	require.Equal(t, 9.2, duration)
}

func TestFunASRAudioResponseWriter_NonStreamVerboseJSONPassthroughCapturesDuration(t *testing.T) {
	counter := token.NewAudioUsageCounter(nil)
	recorder := httptest.NewRecorder()
	w := NewResponseWriterWrapperAudio(recorder, counter, false, audioadapter.NewFunASRAdapter())
	body := `{"text":"hello","segments":[],"duration":12.5}`
	w.WriteHeader(http.StatusOK)

	_, err := w.Write([]byte(body))
	require.NoError(t, err)

	require.JSONEq(t, body, recorder.Body.String())
	duration, ok := w.DurationSeconds()
	require.True(t, ok)
	require.Equal(t, 12.5, duration)
}

func TestAudioResponseWriter_OpenAICompatibleAdapterPassesThroughGzipBody(t *testing.T) {
	counter := token.NewAudioUsageCounter(nil)
	recorder := httptest.NewRecorder()
	recorder.Header().Set("Content-Encoding", "gzip")
	w := NewResponseWriterWrapperAudio(recorder, counter, false, audioadapter.NewOpenAICompatibleAdapter())
	w.WriteHeader(http.StatusOK)

	var body bytes.Buffer
	gzipWriter := gzip.NewWriter(&body)
	_, err := gzipWriter.Write([]byte(`{"text":"hello"}`))
	require.NoError(t, err)
	require.NoError(t, gzipWriter.Close())

	_, err = w.Write(body.Bytes())
	require.NoError(t, err)

	require.Equal(t, body.Bytes(), recorder.Body.Bytes())
	require.Equal(t, "gzip", recorder.Header().Get("Content-Encoding"))
}

func newTestNDJSONWriter(t *testing.T) (*token.AudioUsageCounter, *httptest.ResponseRecorder, audioResponseWriter) {
	t.Helper()
	counter := token.NewAudioUsageCounter(nil)
	recorder := httptest.NewRecorder()
	recorder.Header().Set("Content-Type", "application/x-ndjson")
	w := NewResponseWriterWrapperAudio(recorder, counter, true, nil)
	w.WriteHeader(http.StatusOK)
	return counter, recorder, w
}

func TestNewResponseWriterWrapperAudio_FactoryStream(t *testing.T) {
	counter := token.NewAudioUsageCounter(nil)
	recorder := httptest.NewRecorder()
	w := NewResponseWriterWrapperAudio(recorder, counter, true, nil)
	_, ok := w.(*streamAudioResponseWriter)
	require.True(t, ok, "expected *streamAudioResponseWriter when useStream=true")
}

func TestNewResponseWriterWrapperAudio_FactoryNonStream(t *testing.T) {
	counter := token.NewAudioUsageCounter(nil)
	recorder := httptest.NewRecorder()
	w := NewResponseWriterWrapperAudio(recorder, counter, false, nil)
	_, ok := w.(*nonStreamAudioResponseWriter)
	require.True(t, ok, "expected *nonStreamAudioResponseWriter when useStream=false")
}

func TestStreamAudioResponseWriter_SingleNDJSONLine(t *testing.T) {
	counter, recorder, w := newTestNDJSONWriter(t)

	_, err := w.Write([]byte(`{"text":"hello world"}` + "\n"))
	require.NoError(t, err)

	require.Equal(t, `{"text":"hello world"}`+"\n", recorder.Body.String())

	usage, err := counter.Usage(context.Background())
	require.NoError(t, err)
	require.Equal(t, "hello world", usage.CompletionDesc)
}

func TestStreamAudioResponseWriter_MultipleNDJSONLines(t *testing.T) {
	counter, _, w := newTestNDJSONWriter(t)

	_, err := w.Write([]byte(`{"text":"hello"}` + "\n" + `{"text":" world"}` + "\n"))
	require.NoError(t, err)

	usage, err := counter.Usage(context.Background())
	require.NoError(t, err)
	require.Equal(t, "hello world", usage.CompletionDesc)
}

func TestStreamAudioResponseWriter_PartialLineAcrossWrites(t *testing.T) {
	counter, recorder, w := newTestNDJSONWriter(t)

	_, err := w.Write([]byte(`{"text":"hel`))
	require.NoError(t, err)
	require.Equal(t, "", recorder.Body.String())

	_, err = w.Write([]byte(`lo"}` + "\n"))
	require.NoError(t, err)

	require.Equal(t, `{"text":"hello"}`+"\n", recorder.Body.String())

	usage, err := counter.Usage(context.Background())
	require.NoError(t, err)
	require.Equal(t, "hello", usage.CompletionDesc)
}

func TestStreamAudioResponseWriter_AccumulatesText(t *testing.T) {
	counter, _, w := newTestNDJSONWriter(t)

	_, _ = w.Write([]byte(`{"text":"first"}` + "\n"))
	_, _ = w.Write([]byte(`{"text":"second"}` + "\n"))
	_, _ = w.Write([]byte(`{"text":"third"}` + "\n"))

	usage, err := counter.Usage(context.Background())
	require.NoError(t, err)
	require.Equal(t, "firstsecondthird", usage.CompletionDesc)
}

func TestStreamAudioResponseWriter_DurationFromChunk(t *testing.T) {
	counter, _, w := newTestNDJSONWriter(t)

	_, err := w.Write([]byte(`{"text":"hello","duration":12.5}` + "\n"))
	require.NoError(t, err)

	duration, ok := w.DurationSeconds()
	require.True(t, ok)
	require.Equal(t, 12.5, duration)

	usage, err := counter.Usage(context.Background())
	require.NoError(t, err)
	require.Equal(t, 12.5, usage.Duration)
}

func TestStreamAudioResponseWriter_CapturesDurationFromHeader(t *testing.T) {
	counter := token.NewAudioUsageCounter(nil)
	recorder := httptest.NewRecorder()
	recorder.Header().Set("Content-Type", "application/x-ndjson")
	recorder.Header().Set("Audio-Duration-Seconds", "9.2")
	w := NewResponseWriterWrapperAudio(recorder, counter, true, audioadapter.NewFunASRAdapter())
	w.WriteHeader(http.StatusOK)

	_, err := w.Write([]byte(`{"text":"hello"}` + "\n" + `{"text":"","duration":12.5}` + "\n"))
	require.NoError(t, err)

	require.Equal(t, `{"text":"hello"}`+"\n"+`{"text":"","duration":12.5}`+"\n", recorder.Body.String())
	duration, ok := w.DurationSeconds()
	require.True(t, ok)
	require.Equal(t, 9.2, duration)
	usage, err := counter.Usage(context.Background())
	require.NoError(t, err)
	require.Equal(t, "hello", usage.CompletionDesc)
	require.Equal(t, 9.2, usage.Duration)
}

func TestStreamAudioResponseWriter_DoesNotInventFallbackDuration(t *testing.T) {
	counter, _, w := newTestNDJSONWriter(t)

	_, err := w.Write([]byte(`{"text":"hello"}` + "\n"))
	require.NoError(t, err)

	usage, err := counter.Usage(context.Background())
	require.NoError(t, err)
	require.Zero(t, usage.Duration)

	_, ok := w.DurationSeconds()
	require.False(t, ok)
}

func TestStreamAudioResponseWriter_MalformedJSON(t *testing.T) {
	_, recorder, w := newTestNDJSONWriter(t)

	_, err := w.Write([]byte("not-valid-json\n"))
	require.NoError(t, err)

	require.Equal(t, "not-valid-json\n", recorder.Body.String())
}

func TestStreamAudioResponseWriter_WriteHeaderDeletesContentLength(t *testing.T) {
	counter := token.NewAudioUsageCounter(nil)
	recorder := httptest.NewRecorder()
	recorder.Header().Set("Content-Length", "100")
	w := NewResponseWriterWrapperAudio(recorder, counter, true, nil)

	w.WriteHeader(http.StatusOK)

	require.Empty(t, recorder.Header().Get("Content-Length"))
}

func TestStreamAudioResponseWriter_ErrorPassthrough(t *testing.T) {
	counter := token.NewAudioUsageCounter(nil)
	recorder := httptest.NewRecorder()
	w := NewResponseWriterWrapperAudio(recorder, counter, true, nil)

	w.WriteHeader(http.StatusBadGateway)
	_, err := w.Write([]byte(`{"error":"upstream failure"}`))
	require.NoError(t, err)

	require.Equal(t, `{"error":"upstream failure"}`, recorder.Body.String())
}

func TestStreamAudioResponseWriter_ReturnsZeroOnWriteError(t *testing.T) {
	writeErr := errors.New("write failed")
	internalWriter := &failingAudioResponseWriter{err: writeErr}
	internalWriter.Header().Set("Content-Type", "application/x-ndjson")
	w := NewResponseWriterWrapperAudio(internalWriter, token.NewAudioUsageCounter(nil), true, nil)
	w.WriteHeader(http.StatusOK)

	n, err := w.Write([]byte(`{"text":"hello"}` + "\n"))
	require.ErrorIs(t, err, writeErr)
	require.Zero(t, n)
}

func TestStreamAudioResponseWriter_CRLFNormalization(t *testing.T) {
	counter, _, w := newTestNDJSONWriter(t)

	_, err := w.Write([]byte("{\"text\":\"hello\"}\r\n"))
	require.NoError(t, err)

	usage, err := counter.Usage(context.Background())
	require.NoError(t, err)
	require.Equal(t, "hello", usage.CompletionDesc)
}

func TestStreamAudioResponseWriter_SkipsEmptyLines(t *testing.T) {
	_, recorder, w := newTestNDJSONWriter(t)

	_, err := w.Write([]byte("\n\n{\"text\":\"hello\"}\n\n"))
	require.NoError(t, err)

	require.Equal(t, "{\"text\":\"hello\"}\n", recorder.Body.String())
}

func TestStreamAudioResponseWriter_StatusCodeNilSafety(t *testing.T) {
	var w *streamAudioResponseWriter
	require.Equal(t, http.StatusOK, w.StatusCode())
	_, ok := w.DurationSeconds()
	require.False(t, ok)
}

func TestStreamAudioResponseWriter_SSEDecoder(t *testing.T) {
	counter := token.NewAudioUsageCounter(nil)
	recorder := httptest.NewRecorder()
	recorder.Header().Set("Content-Type", "text/event-stream")
	w := NewResponseWriterWrapperAudio(recorder, counter, true, nil)

	// WriteHeader triggers SSE decoder selection.
	w.WriteHeader(http.StatusOK)

	_, err := w.Write([]byte("data: {\"text\":\"hello from SSE\"}\n\n"))
	require.NoError(t, err)

	usage, err := counter.Usage(context.Background())
	require.NoError(t, err)
	require.Equal(t, "hello from SSE", usage.CompletionDesc)
	require.Equal(t, "data: {\"text\":\"hello from SSE\"}\n\n", recorder.Body.String())
}

func TestStreamAudioResponseWriter_SSEMultipleEvents(t *testing.T) {
	counter := token.NewAudioUsageCounter(nil)
	recorder := httptest.NewRecorder()
	recorder.Header().Set("Content-Type", "text/event-stream")
	w := NewResponseWriterWrapperAudio(recorder, counter, true, nil)
	w.WriteHeader(http.StatusOK)

	_, err := w.Write([]byte("data: {\"text\":\"first\"}\n\ndata: {\"text\":\"second\"}\n\n"))
	require.NoError(t, err)

	usage, err := counter.Usage(context.Background())
	require.NoError(t, err)
	require.Equal(t, "firstsecond", usage.CompletionDesc)
}

func TestStreamAudioResponseWriter_SSEDoneEvent(t *testing.T) {
	counter := token.NewAudioUsageCounter(nil)
	recorder := httptest.NewRecorder()
	recorder.Header().Set("Content-Type", "text/event-stream")
	w := NewResponseWriterWrapperAudio(recorder, counter, true, nil)
	w.WriteHeader(http.StatusOK)

	_, err := w.Write([]byte("data: {\"text\":\"ok\"}\n\ndata: [DONE]\n\n"))
	require.NoError(t, err)

	require.Equal(t, "data: {\"text\":\"ok\"}\n\ndata: [DONE]\n\n", recorder.Body.String())
	usage, err := counter.Usage(context.Background())
	require.NoError(t, err)
	require.Equal(t, "ok", usage.CompletionDesc)
}

func TestStreamAudioResponseWriter_UnknownContentTypePassthrough(t *testing.T) {
	counter := token.NewAudioUsageCounter(nil)
	recorder := httptest.NewRecorder()
	// No recognized Content-Type — passthrough raw without parsing.
	w := NewResponseWriterWrapperAudio(recorder, counter, true, nil)
	w.WriteHeader(http.StatusOK)

	_, err := w.Write([]byte(`{"text":"not-parsed"}` + "\n"))
	require.NoError(t, err)

	// Raw bytes forwarded as-is.
	require.Equal(t, "{\"text\":\"not-parsed\"}\n", recorder.Body.String())

	// No usage captured since decoder was nil.
	_, err = counter.Usage(context.Background())
	require.Error(t, err)
}
