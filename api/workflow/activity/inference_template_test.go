package activity

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/common/types"
)

// The spec-decode virtual parameters are a contract with the deployment form:
// options render dropdowns and virtual params are composed server-side instead
// of being passed through as flags. Nothing else parses these template files,
// so guard the shape here (issue #1489).
func TestInferenceTemplatesSpecDecodeContract(t *testing.T) {
	// ascend-vllm lives under configs/inference/ee and is covered by the EE/SaaS test.
	for _, tc := range []struct {
		engine string
		// conflictArgs are the flags that must declare the spec-decode-method
		// mutual-exclusion hint in this template.
		conflictArgs []string
	}{
		{engine: "nvidia-vllm", conflictArgs: []string{"enforce-eager", "async-scheduling"}},
		{engine: "nvidia-sglang"},
	} {
		t.Run(tc.engine, func(t *testing.T) {
			path := filepath.Join("..", "..", "..", "configs", "inference", tc.engine+".json")
			assertInferenceTemplateSpecDecodeContract(t, tc.engine, path, tc.conflictArgs)
		})
	}
}

func assertInferenceTemplateSpecDecodeContract(t *testing.T, engine, path string, conflictArgs []string) {
	t.Helper()
	sceneOptions := []string{"rag", "code-completion", "rewrite", "summarization", "other"}

	data, err := os.ReadFile(path)
	require.NoError(t, err)

	var template struct {
		EngineArgs []types.EngineArg `json:"engine_args"`
	}
	require.NoError(t, json.Unmarshal(data, &template))

	byName := make(map[string]types.EngineArg, len(template.EngineArgs))
	for _, arg := range template.EngineArgs {
		if _, dup := byName[arg.Name]; dup {
			t.Fatalf("duplicate engine arg %q", arg.Name)
		}
		byName[arg.Name] = arg
	}

	for _, name := range []string{"spec-decode-method", "spec-num-tokens", "spec-draft-model", "spec-decode-scene"} {
		arg, ok := byName[name]
		require.True(t, ok, "missing %s in %s", name, engine)
		require.True(t, arg.Virtual, "%s must be virtual", name)
		require.Equal(t, "", arg.Value, "%s must default to empty", name)
	}

	scene := byName["spec-decode-scene"]
	require.Equal(t, sceneOptions, scene.Options)
	// Which scenes warrant a "suggest ngram" hint is owned by the
	// backend, so changing it never needs a frontend release.
	require.Equal(t,
		[]string{"rag", "code-completion", "rewrite", "summarization"},
		scene.RecommendNgram)

	// The mutual-exclusion hints (issues #1489, #1659) must only appear on
	// flags that really exist in the template and must point at a parameter
	// that also exists there.
	wantConflict := make(map[string]bool, len(conflictArgs))
	for _, name := range conflictArgs {
		wantConflict[name] = true
	}
	for _, name := range []string{"enforce-eager", "async-scheduling"} {
		arg, ok := byName[name]
		if !wantConflict[name] {
			require.False(t, ok, "%s template must not carry %s", engine, name)
			continue
		}
		require.True(t, ok, "missing %s in %s", name, engine)
		require.Equal(t, []string{"spec-decode-method"}, arg.ConflictsWith, "%s conflicts_with", name)
		for _, conflict := range arg.ConflictsWith {
			_, ok := byName[conflict]
			require.True(t, ok, "conflicts_with target %q missing in %s", conflict, engine)
		}
	}
}
