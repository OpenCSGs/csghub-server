package anthropic

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/aigateway/types"
)

type bodyRetentionProxy struct {
	t       *testing.T
	request *types.AnthropicMessagesRequest
}

func (p *bodyRetentionProxy) ServeProxy(_ *gin.Context, _, _ string, writer types.HTTPResponseWriter) {
	require.Empty(p.t, p.request.RawBody, "routing bytes must be released before upstream streaming")
	writer.WriteHeader(200)
	_, err := writer.Write([]byte(`{"id":"msg_test","type":"message","role":"assistant","content":[],"model":"test-model","stop_reason":"end_turn","usage":{"input_tokens":1,"output_tokens":1}}`))
	require.NoError(p.t, err)
}
func TestExecuteReleasesRoutingSnapshotBeforeProxy(t *testing.T) {
	planner := &fakePlanner{target: makeMessagesTarget("http://unused.invalid/v1/messages", "")}
	h := makeTestHandlerWithPlanner(planner)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/v1/messages", strings.NewReader(validMessagesBody("auto")))
	c.Request.Header.Set("Content-Type", "application/json")
	meta, err := h.Extract(c)
	require.NoError(t, err)
	request := meta.ParsedBody.(*types.AnthropicMessagesRequest)
	require.NotEmpty(t, request.RawBody)
	p, err := planner.Plan(c, meta)
	require.NoError(t, err)
	h.ProxyExecutor = &bodyRetentionProxy{t: t, request: request}
	require.NoError(t, h.Execute(c, meta, p))
}
