package common

import "encoding/json"

func JsonStrToMap(jsonStr string) (map[string]interface{}, error) {
	var resMap map[string]interface{}
	if len(jsonStr) == 0 {
		return map[string]interface{}{}, nil
	}
	err := json.Unmarshal([]byte(jsonStr), &resMap)
	return resMap, err
}

func MapToJsonStr(data map[string]interface{}) (string, error) {
	jsonStr, err := json.Marshal(data)
	return string(jsonStr), err

}
