//go:build !ee && !saas

package component

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func expectUpdateModelArchType(_ context.Context, _ *testRuntimeArchWithMocks) {}

func TestGetJsonfilesOmitsEEInferenceConfigs(t *testing.T) {
	chdirToServerCommand(t)

	files, err := getJsonfiles("inference")
	require.NoError(t, err)
	for _, file := range files {
		require.NotContains(t, filepath.ToSlash(file), "/inference/ee/")
	}
}
