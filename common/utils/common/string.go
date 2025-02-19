package common

import (
	"crypto/md5"
	"encoding/hex"
)

// TruncString return a substring if the input string is larger than limit size, truncated string ends with "..."
func TruncString(s string, limit int) string {
	if len(s) <= limit {
		return s
	}

	s1 := []byte(s[:limit])
	s1[limit-1] = '.'
	s1[limit-2] = '.'
	s1[limit-3] = '.'
	return string(s1)
}

func MD5Hash(s string) string {
	hash := md5.New()
	hash.Write([]byte(s))
	hashBytes := hash.Sum(nil)

	return hex.EncodeToString(hashBytes)
}
