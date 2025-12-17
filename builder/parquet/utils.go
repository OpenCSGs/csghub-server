package parquet

import (
	"fmt"
)

const (
	UUIDLength     = 16
	UUIDColumnType = "UUID"
)

func convertUUID(bytes []byte) string {
	if len(bytes) != 16 {
		return ""
	}
	uuid := fmt.Sprintf("%02x%02x%02x%02x-%02x%02x-%02x%02x-%02x%02x-%02x%02x%02x%02x%02x%02x",
		bytes[0], bytes[1], bytes[2], bytes[3],
		bytes[4], bytes[5], bytes[6], bytes[7],
		bytes[8], bytes[9], bytes[10], bytes[11],
		bytes[12], bytes[13], bytes[14], bytes[15])
	return uuid
}
