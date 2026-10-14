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
