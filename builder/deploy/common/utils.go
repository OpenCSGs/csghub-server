package common

import (
	"encoding/json"
	"strings"
)

func JsonStrToMap(jsonStr string) (map[string]string, error) {
	var resMap map[string]string
	if len(strings.Trim(jsonStr, " ")) == 0 {
		return map[string]string{}, nil
	}
	err := json.Unmarshal([]byte(jsonStr), &resMap)
	return resMap, err
}
