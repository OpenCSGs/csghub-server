package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	aitypes "opencsg.com/csghub-server/aigateway/types"
)

// fakePiiMasker records how the chat Extract hook gated the masker. The
// real rewriting behavior is covered by the component tests; this fake
// isolates the transport gating (deployment switch + header opt-in).
type fakePiiMasker struct {
	enabled bool
	calls   int
}

func (f *fakePiiMasker) Enabled() bool { return f.enabled }

func (f *fakePiiMasker) MaskChatRequest(_ context.Context, _ *aitypes.ChatCompletionRequest) bool {
	f.calls++
	return true
}

func chatExtractTestContext(t *testing.T, header string) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	body := `{"model":"gpt-x","messages":[{"role":"user","content":"call 13911112222"}]}`
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	if header != "" {
		c.Request.Header.Set(PIICheckHeader, header)
	}
	return c, w
}

func TestChatExtractPIIMaskGating(t *testing.T) {
	cases := []struct {
		desc        string
		enabled     bool
		header      string
		wantInvoked bool
	}{
		{desc: "enabled and opted in", enabled: true, header: "enable", wantInvoked: true},
		{desc: "enabled and opted in with true", enabled: true, header: "true", wantInvoked: true},
		{desc: "enabled without header", enabled: true, header: "", wantInvoked: false},
		{desc: "enabled with other header value", enabled: true, header: "no", wantInvoked: false},
		{desc: "disabled with header", enabled: false, header: "enable", wantInvoked: false},
	}
	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			masker := &fakePiiMasker{enabled: tc.enabled}
			h := &chatPipelineHandler{handler: &OpenAIHandlerImpl{piiMasker: masker}}
			c, w := chatExtractTestContext(t, tc.header)

			meta, err := h.Extract(c)
			require.NoError(t, err)
			require.NotNil(t, meta)
			assert.Equal(t, http.StatusOK, w.Code)

			if tc.wantInvoked {
				assert.Equal(t, 1, masker.calls, "masker should be invoked exactly once")
			} else {
				assert.Zero(t, masker.calls, "masker should not be invoked")
			}
			// The opt-in header is gateway control metadata and must never
			// reach the upstream proxy.
			assert.Empty(t, c.Request.Header.Get(PIICheckHeader), "opt-in header must be stripped")
			// The metadata carries whatever the (possibly rewritten) parsed
			// body holds; parsing must always succeed.
			pb, ok := meta.ParsedBody.(*chatParsedBody)
			require.True(t, ok)
			assert.Equal(t, "gpt-x", pb.Req.Model)
		})
	}
}

func TestChatExtractWithoutMasker(t *testing.T) {
	h := &chatPipelineHandler{handler: &OpenAIHandlerImpl{}}
	c, _ := chatExtractTestContext(t, "enable")
	meta, err := h.Extract(c)
	require.NoError(t, err)
	assert.NotNil(t, meta)
}
