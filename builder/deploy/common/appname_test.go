package common

import (
	"strings"
	"testing"
)

func TestUniqueSpaceAppName(t *testing.T) {
	namespace := "aaaaaaaalesdfsdfida-tededswsddst-2024asdfsadfsefsdfsdfsdf0327"
	spaceName := "leasdfaida_tesdfsdfsdfst-spaasdasdascasdfasfase"
	spaceID := int64(123456)

	spaceAppName := UniqueSpaceAppName("u", namespace, spaceName, spaceID)

	if len(spaceAppName) > 63 {
		t.Fatal("space app name is too long")
	}

	if spaceAppName != "u-aaaaaaaalesdfsdfida-tededswsddst-2024asdfsadfsefsdfsdfsd-2n9c" {
		t.Fatal("space app name wrong")
	}

	spaceID, err := parseUniqueSpaceAppName(spaceAppName)
	if err != nil {
		t.Fatal(err)
	}

	if spaceID != 123456 {
		t.Fatal("spaceID wrong")
	}

	host := "u-leida-test-20240327-leida-test-space-1.space-stg.opencsg.com"
	domainParts := strings.SplitN(host, ".", 2)
	spaceAppName = domainParts[0]
	spaceID, err = parseUniqueSpaceAppName(spaceAppName)
	if err != nil {
		t.Fatal(err)
	}

	if spaceID != 1 {
		t.Fatal("spaceID wrong from host")
	}
}
