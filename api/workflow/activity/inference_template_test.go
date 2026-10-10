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
	sceneOptions := []string{"rag", "code-completion", "rewrite", "summarization", "other"}

	for _, engine := range []string{"nvidia-vllm", "ascend-vllm", "nvidia-sglang"} {
		t.Run(engine, func(t *testing.T) {
			path := filepath.Join("..", "..", "..", "configs", "inference", engine+".json")
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

			// The enforce-eager x spec-decode mutual-exclusion hint (issue
			// #1489) exists in both vllm templates and points at a parameter
			// that really exists in the same template; sglang has no
			// enforce-eager at all.
			if engine == "nvidia-sglang" {
				_, ok := byName["enforce-eager"]
				require.False(t, ok, "sglang template must not carry enforce-eager")
				return
			}
			eager, ok := byName["enforce-eager"]
			require.True(t, ok, "missing enforce-eager in %s", engine)
			require.Equal(t, []string{"spec-decode-method"}, eager.ConflictsWith)
			for _, conflict := range eager.ConflictsWith {
				_, ok := byName[conflict]
				require.True(t, ok, "conflicts_with target %q missing in %s", conflict, engine)
			}
		})
	}
}
