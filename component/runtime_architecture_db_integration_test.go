package component

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/tests"
	"opencsg.com/csghub-server/common/types"
)

// Integration test: sync the inference configs into a real database and read
// runtime_frameworks back, so the merge is verified where the deploy path
// consumes it and not only at the call site.
func TestRuntimeFrameworkSyncPersistsMergedEngineArgsPerImage(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.Background()
	chdirToServerCommand(t)

	c := &runtimeArchitectureComponentImpl{
		runtimeFrameworksStore: database.NewRuntimeFrameworksStoreWithDB(db),
		runtimeArchStore:       database.NewRuntimeArchitecturesStoreWithDB(db),
	}
	require.NoError(t, c.UpdateRuntimeFrameworkByType(ctx, types.InferenceType))

	engineConfig := readInferenceEngineConfig(t, "vllm.json")
	sharedNames := engineArgNameList(engineConfig.EngineArgs)
	require.NotEmpty(t, sharedNames)

	// vllm.json declares four images, two of which accept async-scheduling and
	// two of which accept guided-decoding-backend and swap-space. Each row must
	// hold the shared args followed by the args of its own image only.
	for _, tc := range []struct {
		image       string
		computeType string
		imageArgs   []string
	}{
		{"opencsghq/vllm:v0.28.0", "gpu", []string{"async-scheduling", "spec-decode-method", "spec-num-tokens", "spec-draft-model", "spec-decode-scene"}},
		{"opencsghq/vllm-cpu:v0.24.0", "cpu", []string{"async-scheduling"}},
		{"opencsghq/vllm:v0.9.2-cu118", "gpu", []string{"guided-decoding-backend", "swap-space"}},
		{"opencsghq/vllm:v0.8.5-dtk25.04", "dcu", []string{"guided-decoding-backend", "swap-space"}},
	} {
		t.Run(tc.image, func(t *testing.T) {
			frame, err := c.runtimeFrameworksStore.FindByFrameImageAndComputeType(ctx, tc.image, tc.computeType)
			require.NoError(t, err)
			require.Equal(t, engineConfig.EngineName, frame.FrameName)

			stored := engineArgNames(t, frame.EngineArgs)
			require.Equal(t, append(append([]string{}, sharedNames...), tc.imageArgs...), stored)
			// Flags no configured version accepts must not reach the database.
			require.NotContains(t, stored, "limit-mm-per-prompt")
			t.Logf("runtime_frameworks row: frame_name=%s frame_version=%s frame_image=%s compute_type=%s engine_args=%s",
				frame.FrameName, frame.FrameVersion, frame.FrameImage, frame.ComputeType, frame.EngineArgs)
		})
	}

	// The two groups of images end up with different stored args, which is what
	// separates this from the single shared template.
	newer, err := c.runtimeFrameworksStore.FindByFrameImageAndComputeType(ctx, "opencsghq/vllm:v0.28.0", "gpu")
	require.NoError(t, err)
	older, err := c.runtimeFrameworksStore.FindByFrameImageAndComputeType(ctx, "opencsghq/vllm:v0.9.2-cu118", "gpu")
	require.NoError(t, err)
	require.NotEqual(t, newer.EngineArgs, older.EngineArgs)
}

func readInferenceEngineConfig(t *testing.T, fileName string) types.EngineConfig {
	t.Helper()
	configFiles, err := getJsonfiles("inference")
	require.NoError(t, err)

	var engineConfig types.EngineConfig
	found := false
	for _, path := range configFiles {
		if filepath.Base(path) != fileName {
			continue
		}
		content, err := os.ReadFile(path)
		require.NoError(t, err)
		require.NoError(t, json.Unmarshal(content, &engineConfig))
		found = true
	}
	require.Truef(t, found, "%s not found in configs/inference", fileName)
	return engineConfig
}
