package types

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAdmissionPriorityFromSource(t *testing.T) {
	cases := []struct {
		name  string
		scope string
		want  AdmissionPriority
	}{
		{"scope high", "high", AdmissionPriorityHigh},
		{"scope low", "low", AdmissionPriorityLow},
		{"missing scope defaults to high", "", AdmissionPriorityHigh},
		{"unknown scope defaults to high", "critical", AdmissionPriorityHigh},
		{"case-insensitive low", " LOW ", AdmissionPriorityLow},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, AdmissionPriorityFromSource(tc.scope))
		})
	}
}

func TestResolveQueuePriority(t *testing.T) {
	tag := func(n int) *int { return &n }
	cases := []struct {
		name  string
		scope string
		tag   *int
		want  AdmissionPriority
	}{
		// No tag: the API key scope decides exactly as before.
		{"no tag, scope high", "high", nil, AdmissionPriorityHigh},
		{"no tag, scope low", "low", nil, AdmissionPriorityLow},
		{"no tag, scope unset", "", nil, AdmissionPriorityHigh},
		{"no tag, unknown scope", "critical", nil, AdmissionPriorityHigh},

		// priority:<n> promotes above every scope class, irrespective of the
		// API key scope. The mapping is 1:1 (-n): adjacent tiers stay one
		// stride apart and the clamp keeps every score an exact integer, so
		// same-tier FIFO keeps full ms resolution even at the top tier.
		{"tag 1 beats scope high", "high", tag(1), -1},
		{"tag 1 beats scope low", "low", tag(1), -1},
		{"higher tag is promoted first", "high", tag(10), -10},
		{"tag tier ignores the scope", "low", tag(5), -5},
		{"top tier stays within the exact-double bound", "high", tag(maxUserTagPriority), -AdmissionPriority(maxUserTagPriority)},
		{"oversized tag is clamped", "", tag(maxUserTagPriority * 10), -AdmissionPriority(maxUserTagPriority)},

		// low_priority (0) demotes to the bottom band regardless of scope.
		{"low_priority tag, scope high", "high", tag(0), AdmissionPriorityLowest},
		{"low_priority tag, scope low", "low", tag(0), AdmissionPriorityLowest},
		{"low_priority tag, scope unset", "", tag(0), AdmissionPriorityLowest},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ResolveQueuePriority(QueuePriorityInput{APIKeyScope: tc.scope, UserTag: tc.tag})
			require.Equal(t, tc.want, got)
		})
	}
}
