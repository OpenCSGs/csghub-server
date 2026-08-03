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
		"pass_at_3": 55,
		"total_input_tokens": 12000,
		"total_output_tokens": 3400,
		"total_tokens": 15400
	}`)

	summary, err := ParseClawEvalSummary(raw)
	require.NoError(t, err)
	require.Equal(t, 129, summary.Tasks)
	require.Equal(t, 3, summary.TrialsPerTask)
	require.Equal(t, 3, summary.Errored)
	require.InDelta(t, 0.72, summary.AvgScore, 0.0001)
	require.Equal(t, 40, summary.PassHatK)
	require.Equal(t, 55, summary.PassAtK)
	require.Equal(t, 12000, summary.InputTokens)
	require.Equal(t, 3400, summary.OutputTokens)
	require.Equal(t, 15400, summary.TotalTokens)

	encoded, err := json.Marshal(summary)
	require.NoError(t, err)
	require.JSONEq(t, `{
		"tasks": 129,
		"trials_per_task": 3,
		"errored": 3,
		"avg_score": 0.72,
		"pass_hat_3": 40,
		"pass_at_3": 55,
		"input_tokens": 12000,
		"output_tokens": 3400,
		"total_tokens": 15400
	}`, string(encoded))
}
