//go:build !ee && !saas

package component

import (
	aitypes "opencsg.com/csghub-server/aigateway/types"
	commontypes "opencsg.com/csghub-server/common/types"
)

type logCaptureImpl struct {
}

func NewLLMLogRecorder(_, _, _ string, _ commontypes.LLMLogRequest, _ map[string]any) (LLMLogRecorder, error) {
	return &logCaptureImpl{}, nil
}

func (c *logCaptureImpl) Completion(_ aitypes.ChatCompletion) {
}

func (c *logCaptureImpl) AppendCompletionChunk(_ aitypes.ChatCompletionChunk) {
}

func (c *logCaptureImpl) Record() (*commontypes.LLMLogRecord, error) {
	return nil, nil
}
