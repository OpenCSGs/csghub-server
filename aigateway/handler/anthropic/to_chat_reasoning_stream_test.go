package anthropic

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStreamReasoningFieldBecomesThinkingBlock(t *testing.T) {
	for _, field := range []string{"reasoning_content", "reasoning"} {
		t.Run(field, func(t *testing.T) {
			chunks := []string{
				chatChunk(fmt.Sprintf(`{%q:"think "}`, field), "null"),
				chatChunk(fmt.Sprintf(`{%q:"hard"}`, field), "null"),
				chatChunk(`{"content":"answer"}`, `"stop"`),
			}
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/event-stream")
				w.WriteHeader(200)
				for _, c := range chunks {
					fmt.Fprintf(w, "%s\n\n", c)
				}
				fmt.Fprint(w, "data: [DONE]\n\n")
			}))
			defer upstream.Close()

			planner := &fakePlanner{target: makeMessagesTarget(upstream.URL+"/v1/chat/completions", "")}
			handler := makeTestHandlerWithPlanner(planner)
			w := dispatchWithTarget(t, handler, fmt.Sprintf(toolCallsRequestBody, true), planner)
			require.Equal(t, 200, w.Code)

			thinking := ""
			for _, ev := range parseSSEEvents(t, w.Body.String()) {
				if ev.Event != "content_block_delta" {
					continue
				}
				d, _ := ev.Data["delta"].(map[string]any)
				if d["type"] == "thinking_delta" {
					thinking += d["thinking"].(string)
				}
			}
			assert.Equal(t, "think hard", thinking)
		})
	}
}
