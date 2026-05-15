package handler

import (
	"net/http"

	"opencsg.com/csghub-server/aigateway/types"
)

func applyModelAuthHeaders(header http.Header, model *types.Model) error {
	return types.ApplyRequestAuthHeaders(header, model.AuthHead)
}
