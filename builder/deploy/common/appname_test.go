package common

import (
	"strings"
	"testing"
)

func TestUniqueSpaceAppName(t *testing.T) {
	namespace := "leida-test-20240327"
	spaceName := "leida_test-space"
	spaceID := int64(1)

	spaceAppName := UniqueSpaceAppName(namespace, spaceName, spaceID)
	if spaceAppName != "u-leida-test-20240327-leida-test-space-1" {
		t.Fatal("space app name wrong")
	}

	spaceID, err := ParseUniqueSpaceAppName(spaceAppName)
	if err != nil {
		t.Fatal(err)
	}

	if spaceID != 1 {
		t.Fatal("spaceID wrong")
	}

	host := "u-leida-test-20240327-leida-test-space-1.space-stg.opencsg.com"
	domainParts := strings.SplitN(host, ".", 2)
	spaceAppName = domainParts[0]
	spaceID, err = ParseUniqueSpaceAppName(spaceAppName)
	if err != nil {
		t.Fatal(err)
	}

	if spaceID != 1 {
		t.Fatal("spaceID wrong from host")
	}
}
