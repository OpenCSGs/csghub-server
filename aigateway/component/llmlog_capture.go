package component

import (
	aitypes "opencsg.com/csghub-server/aigateway/types"
	commontypes "opencsg.com/csghub-server/common/types"
)

type LLMLogRecorder interface {
	Completion(aitypes.ChatCompletion)
	AppendCompletionChunk(aitypes.ChatCompletionChunk)
	Record() (*commontypes.LLMLogRecord, error)
	// Messages returns the normalized input and output messages.
	Messages() (input, output []commontypes.LLMLogMessage)
	TraceInfo() commontypes.LLMLogTraceInfo
}
