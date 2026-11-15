package decisions

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPatchDecisionRawModelSemanticPreservation(t *testing.T) {
	raw := []byte(`{"trace_id":"d-1","input":[{"role":"user"}],"questions":[{"type":"predicate","name":"x","custom":{"keep":true}}],"model":"client-model","extension":{"a":[1,2]},"big":12345678901234567890}`)

	patched, err := patchDecisionRawModel(raw, "resolved-model")
	require.NoError(t, err)

	// The model is replaced and every other field survives with its exact
	// value bytes (RawMessage), including unknown extensions and a large
	// integer that would lose precision through float64.
	var before, after map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(raw, &before))
	require.NoError(t, json.Unmarshal(patched, &after))
	require.Equal(t, `"resolved-model"`, string(after["model"]))
	delete(before, "model")
	delete(after, "model")
	require.Equal(t, before, after)
}

func TestPatchDecisionRawModelDuplicateKeysCollapse(t *testing.T) {
	// Duplicate top-level keys are invalid per the RFC's SHOULD-unique rule;
	// the map round trip keeps the last value and emits one key.
	patched, err := patchDecisionRawModel([]byte(`{"model":"first","input":"a","model":"second"}`), "resolved")
	require.NoError(t, err)

	var got map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(patched, &got))
	require.Equal(t, `"resolved"`, string(got["model"]))
	require.Equal(t, `"a"`, string(got["input"]))
}

func TestPatchDecisionRawModelEscapedModelName(t *testing.T) {
	patched, err := patchDecisionRawModel([]byte(`{"model":"client\"quoted","input":"x"}`), "resolved\"model")
	require.NoError(t, err)

	var got map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(patched, &got))
	require.Equal(t, `"resolved\"model"`, string(got["model"]))
	require.Equal(t, `"x"`, string(got["input"]))
}

func TestPatchDecisionRawModelNonObjectFails(t *testing.T) {
	// Validate rejects non-object bodies before this point; the patch must
	// still fail cleanly rather than corrupting the payload.
	_, err := patchDecisionRawModel([]byte(`[1,2]`), "resolved")
	require.Error(t, err)
}
