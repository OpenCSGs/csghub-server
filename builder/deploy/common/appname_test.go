package common

import (
	"testing"
)

func TestUniqueSpaceAppName(t *testing.T) {
	namespace := "user_name_1"
	spaceName := "space-name-1"
	spaceID := int64(1)

	spaceAppName := UniqueSpaceAppName(namespace, spaceName, spaceID)
	if spaceAppName != "u-user-name-1-space-name-1-1" {
		t.Fatal("space app name wrong")
	}

	spaceID, err := ParseUniqueSpaceAppName(spaceAppName)
	if err != nil {
		t.Fatal(err)
	}

	if spaceID != 1 {
		t.Fatal("spaceID wrong")
	}
}
