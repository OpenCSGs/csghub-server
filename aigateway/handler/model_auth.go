package handler

import (
	"encoding/json"
	"net/http"
	"strings"

	"opencsg.com/csghub-server/aigateway/types"
)

func applyModelAuthHeaders(header http.Header, model *types.Model) error {
	if model.AuthHead == "" {
		return nil
	}
	var authMap map[string]string
	if err := json.Unmarshal([]byte(model.AuthHead), &authMap); err != nil {
		authHead := strings.TrimSpace(model.AuthHead)
		if strings.HasPrefix(strings.ToLower(authHead), "bearer ") {
			header.Set("Authorization", authHead)
			return nil
		}
		return err
	}
	for authKey, authVal := range authMap {
		header.Set(authKey, authVal)
	}
	return nil
}
