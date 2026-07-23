package token

import (
	"context"
	"fmt"

	"opencsg.com/csghub-server/aigateway/types"
	commontypes "opencsg.com/csghub-server/common/types"
)

var _ Counter = (*OCRUsageCounter)(nil)

// OCRUsageCounter derives OCR usage from the normalized OCR response.
// Pages are the billing unit; recognized text must never be recorded here.
type OCRUsageCounter struct {
	usage  *Usage
	images int64
	pages  int64
}

func NewOCRUsageCounter() *OCRUsageCounter {
	return &OCRUsageCounter{images: 1, pages: 1}
}

func (c *OCRUsageCounter) SetRequestDetails(images, pages int64) {
	if images > 0 {
		c.images = images
	}
	if pages > 0 {
		c.pages = pages
	}
}

func (c *OCRUsageCounter) OCRResponse(resp *types.OCRResponse) {
	if resp == nil {
		return
	}
	pages := int64(resp.Usage.Pages)
	if pages <= 0 {
		pages = c.pages
	}
	images := int64(resp.Usage.Images)
	if images <= 0 {
		images = c.images
	}
	c.usage = &Usage{
		DataType:       string(commontypes.DataTypeOCR),
		CompletionRC:   pages,
		CompletionDesc: fmt.Sprintf("pages=%d,images=%d", pages, images),
	}
}

func (c *OCRUsageCounter) Usage(_ context.Context) (*Usage, error) {
	if c.usage == nil {
		return nil, fmt.Errorf("no usage data available")
	}
	return c.usage, nil
}
