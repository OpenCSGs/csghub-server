package types

import (
	"encoding/json"
	"net/http"
	"strings"
)

func ApplyRequestAuthHeaders(header http.Header, authHeadStr string) error {
	if authHeadStr == "" {
		return nil
	}

	var authMap map[string]string
	if err := json.Unmarshal([]byte(authHeadStr), &authMap); err != nil {
		authHead := strings.TrimSpace(authHeadStr)
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
