package types

import (
	"math"
	"testing"
)

func TestUserRoutingPriorityFromTags(t *testing.T) {
	tests := []struct {
		name string
		tags []string
		want int
	}{
		{
			name: "no tags is unrestricted",
			tags: nil,
			want: UnrestrictedUpstreamPriority,
		},
		{
			name: "unrelated tags are unrestricted",
			tags: []string{"paid", "beta_tester"},
			want: UnrestrictedUpstreamPriority,
		},
		{
			name: "low_priority tag is the low channel",
			tags: []string{"paid", UserTagLowPriority},
			want: 0,
		},
		{
			name: "low_priority with surrounding spaces",
			tags: []string{" low_priority "},
			want: 0,
		},
		{
			name: "numeric priority tag wins over low_priority",
			tags: []string{UserTagLowPriority, "priority:50"},
			want: 50,
		},
		{
			name: "highest numeric tag wins",
			tags: []string{"priority:10", "priority:50", "priority:30"},
			want: 50,
		},
		{
			name: "malformed numeric tag is ignored",
			tags: []string{"priority:abc", "priority:-5", UserTagLowPriority},
			want: 0,
		},
		{
			name: "only malformed tags is unrestricted",
			tags: []string{"priority:abc", "priority:", "priorityx:5"},
			want: UnrestrictedUpstreamPriority,
		},
		{
			name: "zero numeric tag equals low channel",
			tags: []string{"priority:0"},
			want: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := UserRoutingPriorityFromTags(tt.tags); got != tt.want {
				t.Fatalf("UserRoutingPriorityFromTags(%v) = %d, want %d", tt.tags, got, tt.want)
			}
		})
	}
}

func TestUnrestrictedUpstreamPriorityAdmitsEverything(t *testing.T) {
	if UnrestrictedUpstreamPriority <= math.MaxInt32 {
		t.Fatalf("unrestricted priority must exceed any realistic configured tier, got %d", UnrestrictedUpstreamPriority)
	}
}

func TestIsUserRoutingTagName(t *testing.T) {
	tests := []struct {
		name string
		tag  string
		want bool
	}{
		{name: "low_priority", tag: UserTagLowPriority, want: true},
		{name: "low_priority with spaces", tag: " low_priority ", want: false},
		{name: "numeric tier", tag: "priority:50", want: true},
		{name: "numeric tier with spaces", tag: " priority:50 ", want: false},
		{name: "unparsable tier is still protected", tag: "priority:abc", want: true},
		{name: "unrelated tag", tag: "paid", want: false},
		{name: "prefix elsewhere", tag: "high_priority", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsUserRoutingTagName(tt.tag); got != tt.want {
				t.Fatalf("IsUserRoutingTagName(%q) = %v, want %v", tt.tag, got, tt.want)
			}
		})
	}
}
