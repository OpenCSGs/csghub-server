package parquet

import (
	"testing"
)

func Test_convertUUID(t *testing.T) {
	bytes := []byte{0x01, 0x23, 0x45, 0x67, 0x89, 0xab, 0xcd, 0xef, 0xfe, 0xdc, 0xba, 0x98, 0x76, 0x54, 0x32, 0x10}
	uuid := convertUUID(bytes)
	if uuid != "01234567-89ab-cdef-fedc-ba9876543210" {
		t.Errorf("ConvertUUID(%v) = %v; want %v", bytes, uuid, "01234567-89ab-cdef-fedc-ba9876543210")
	}
}
