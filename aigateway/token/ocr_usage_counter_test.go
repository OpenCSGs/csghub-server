package token

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/aigateway/types"
	commontypes "opencsg.com/csghub-server/common/types"
)

func TestOCRUsageCounter_UsageFromResponse(t *testing.T) {
	c := NewOCRUsageCounter()
	c.OCRResponse(&types.OCRResponse{
		Usage: types.OCRUsage{Pages: 3, Images: 1},
	})

	usage, err := c.Usage(context.Background())
	require.NoError(t, err)
	assert.Equal(t, string(commontypes.DataTypeOCR), usage.DataType)
	assert.EqualValues(t, 3, usage.CompletionRC)
	assert.Equal(t, "pages=3,images=1", usage.CompletionDesc)
	assert.Zero(t, usage.TotalTokens)
}

func TestOCRUsageCounter_FallbackToRequestDetails(t *testing.T) {
	c := NewOCRUsageCounter()
	c.SetRequestDetails(2, 5)
	c.OCRResponse(&types.OCRResponse{})

	usage, err := c.Usage(context.Background())
	require.NoError(t, err)
	assert.EqualValues(t, 5, usage.CompletionRC)
	assert.Equal(t, "pages=5,images=2", usage.CompletionDesc)
}

func TestOCRUsageCounter_ErrorBeforeResponse(t *testing.T) {
	c := NewOCRUsageCounter()
	_, err := c.Usage(context.Background())
	require.Error(t, err)
}

func TestOCRUsageCounter_NilResponse(t *testing.T) {
	c := NewOCRUsageCounter()
	c.OCRResponse(nil)
	_, err := c.Usage(context.Background())
	require.Error(t, err)
}
