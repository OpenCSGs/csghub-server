package tests

import (
	"testing"

	"opencsg.com/csghub-server/tests/testinfra"
)

func TestIntegration_ServerStart(t *testing.T) {
	testinfra.StartTestServer(t)
}
