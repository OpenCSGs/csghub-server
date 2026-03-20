package text2image

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"opencsg.com/csghub-server/aigateway/types"
)

func TestOpenAICompatibleAdapter_GetHeaders(t *testing.T) {
	adapter := NewOpenAICompatibleAdapter()
	model := &types.Model{}

	t.Run("returns Content-Type application/json", func(t *testing.T) {
		headers := adapter.GetHeaders(model, nil)
		assert.Equal(t, map[string]string{"Content-Type": "application/json"}, headers)
	})

	t.Run("accepts nil req", func(t *testing.T) {
		headers := adapter.GetHeaders(model, nil)
		assert.Len(t, headers, 1)
		assert.Equal(t, "application/json", headers["Content-Type"])
	})

	t.Run("accepts req with output_format", func(t *testing.T) {
		req := &types.ImageGenerationRequest{}
		req.OutputFormat = "webp"
		headers := adapter.GetHeaders(model, req)
		assert.Equal(t, map[string]string{"Content-Type": "application/json"}, headers)
	})
}
