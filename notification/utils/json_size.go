package utils

import "encoding/json"

func JSONSize(v any) (int, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return 0, err
	}
	return len(b), nil
}
