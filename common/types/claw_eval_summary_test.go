package types

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseClawEvalSummary(t *testing.T) {
	raw := []byte(`{
		"tasks": 129,
		"trials_per_task": 3,
		"errored": 3,
		"avg_score": 0.72,
		"pass_hat_3": 40,
		"pass_at_3": 55
	}`)

	summary, err := ParseClawEvalSummary(raw)
	require.NoError(t, err)
	require.Equal(t, 129, summary.Tasks)
	require.Equal(t, 3, summary.TrialsPerTask)
	require.Equal(t, 3, summary.Errored)
	require.InDelta(t, 0.72, summary.AvgScore, 0.0001)
	require.Equal(t, 40, summary.PassHatK)
	require.Equal(t, 55, summary.PassAtK)

	encoded, err := json.Marshal(summary)
	require.NoError(t, err)
	require.JSONEq(t, string(raw), string(encoded))
}
