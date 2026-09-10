package handler

import (
	audioadapter "opencsg.com/csghub-server/aigateway/component/adapter/audio"
	"opencsg.com/csghub-server/aigateway/types"
)

func (h *OpenAIHandlerImpl) audioAdapter(model *types.Model) audioadapter.Adapter {
	if h != nil && h.audioRegistry != nil {
		if adapter := h.audioRegistry.GetAdapter(model); adapter != nil {
			return adapter
		}
	}
	return audioadapter.NewOpenAICompatibleAdapter()
}
