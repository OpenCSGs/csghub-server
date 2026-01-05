package component

import (
	"testing"

	"github.com/stretchr/testify/require"
	v1 "knative.dev/serving/pkg/apis/serving/v1"
)

func TestTrafficFixAlgorithm(t *testing.T) {
	// Helper function to create a pointer to int64
	int64Ptr := func(i int64) *int64 {
		return &i
	}

	// Helper function to create a pointer to bool
	boolPtr := func(b bool) *bool {
		return &b
	}

	tests := []struct {
		name             string
		oldTraffic       []v1.TrafficTarget
		delRevisionName  string
		wantFixedTraffic []v1.TrafficTarget
		wantValid        bool
	}{
		{
			name: "Normal case: delete one revision, adjust other revisions proportionally",
			oldTraffic: []v1.TrafficTarget{
				{RevisionName: "rev1", Percent: int64Ptr(30)},
				{RevisionName: "rev2", Percent: int64Ptr(50)},
				{RevisionName: "rev3", Percent: int64Ptr(20)},
			},
			delRevisionName: "rev1",
			wantFixedTraffic: []v1.TrafficTarget{
				{RevisionName: "rev2", Percent: int64Ptr(71)},
				{RevisionName: "rev3", Percent: int64Ptr(29)},
			},
			wantValid: true,
		},
		{
			name: "Boundary case: delete all revisions, fallback to latest",
			oldTraffic: []v1.TrafficTarget{
				{RevisionName: "rev1", Percent: int64Ptr(100)},
			},
			delRevisionName: "rev1",
			wantFixedTraffic: []v1.TrafficTarget{
				{LatestRevision: boolPtr(true), Percent: int64Ptr(100)},
			},
			wantValid: true,
		},
		{
			name: "Boundary case: delete one revision, only one left",
			oldTraffic: []v1.TrafficTarget{
				{RevisionName: "rev1", Percent: int64Ptr(40)},
				{RevisionName: "rev2", Percent: int64Ptr(60)},
			},
			delRevisionName: "rev1",
			wantFixedTraffic: []v1.TrafficTarget{
				{RevisionName: "rev2", Percent: int64Ptr(100)},
			},
			wantValid: true,
		},
		{
			name: "Boundary case: original traffic sum not 100%, should be fixed",
			oldTraffic: []v1.TrafficTarget{
				{RevisionName: "rev1", Percent: int64Ptr(30)},
				{RevisionName: "rev2", Percent: int64Ptr(40)},
			},
			delRevisionName: "",
			wantFixedTraffic: []v1.TrafficTarget{
				{RevisionName: "rev1", Percent: int64Ptr(42)},
				{RevisionName: "rev2", Percent: int64Ptr(58)},
			},
			wantValid: true,
		},
		{
			name: "Boundary case: nil percent should be handled as 0",
			oldTraffic: []v1.TrafficTarget{
				{RevisionName: "rev1", Percent: nil},
				{RevisionName: "rev2", Percent: int64Ptr(100)},
			},
			delRevisionName: "",
			wantFixedTraffic: []v1.TrafficTarget{
				{RevisionName: "rev1", Percent: int64Ptr(0)},
				{RevisionName: "rev2", Percent: int64Ptr(100)},
			},
			wantValid: true,
		},
		{
			name: "Boundary case: empty revision name should be skipped",
			oldTraffic: []v1.TrafficTarget{
				{RevisionName: "", Percent: int64Ptr(50)},
				{RevisionName: "rev1", Percent: int64Ptr(50)},
			},
			delRevisionName: "",
			wantFixedTraffic: []v1.TrafficTarget{
				{RevisionName: "rev1", Percent: int64Ptr(100)},
			},
			wantValid: true,
		},
		{
			name: "Case: delete non-existent revision",
			oldTraffic: []v1.TrafficTarget{
				{RevisionName: "rev1", Percent: int64Ptr(50)},
				{RevisionName: "rev2", Percent: int64Ptr(50)},
			},
			delRevisionName: "non-existent",
			wantFixedTraffic: []v1.TrafficTarget{
				{RevisionName: "rev1", Percent: int64Ptr(50)},
				{RevisionName: "rev2", Percent: int64Ptr(50)},
			},
			wantValid: true,
		},
		{
			name: "Case: multiple revisions with zero percent",
			oldTraffic: []v1.TrafficTarget{
				{RevisionName: "rev1", Percent: int64Ptr(0)},
				{RevisionName: "rev2", Percent: int64Ptr(0)},
				{RevisionName: "rev3", Percent: int64Ptr(100)},
			},
			delRevisionName: "rev3",
			wantFixedTraffic: []v1.TrafficTarget{
				{RevisionName: "rev1", Percent: int64Ptr(0)},
				{RevisionName: "rev2", Percent: int64Ptr(100)},
			},
			wantValid: true,
		},
		{
			name: "Case: traffic sum zero after deletion",
			oldTraffic: []v1.TrafficTarget{
				{RevisionName: "rev1", Percent: int64Ptr(0)},
				{RevisionName: "rev2", Percent: int64Ptr(0)},
			},
			delRevisionName: "rev1",
			wantFixedTraffic: []v1.TrafficTarget{
				{RevisionName: "rev2", Percent: int64Ptr(100)},
			},
			wantValid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Act
			fixedTraffic, isValid := TrafficFixAlgorithm(tt.oldTraffic, tt.delRevisionName)

			// Assert validity
			require.Equal(t, tt.wantValid, isValid)

			// Assert traffic targets if valid
			if tt.wantValid {
				// Check number of traffic targets
				require.Equal(t, len(tt.wantFixedTraffic), len(fixedTraffic))

				// Check each traffic target
				for i, wantTarget := range tt.wantFixedTraffic {
					gotTarget := fixedTraffic[i]

					// Check RevisionName
					require.Equal(t, wantTarget.RevisionName, gotTarget.RevisionName)

					// Check LatestRevision
					if wantTarget.LatestRevision != nil {
						require.NotNil(t, gotTarget.LatestRevision)
						require.Equal(t, *wantTarget.LatestRevision, *gotTarget.LatestRevision)
					} else {
						require.Nil(t, gotTarget.LatestRevision)
					}

					// Check Percent
					require.NotNil(t, gotTarget.Percent)
					require.Equal(t, *wantTarget.Percent, *gotTarget.Percent)
				}

				// Verify sum of traffic is 100%
				var totalPercent int64
				for _, target := range fixedTraffic {
					totalPercent += *target.Percent
				}
				require.Equal(t, int64(100), totalPercent)
			}
		})
	}
}
