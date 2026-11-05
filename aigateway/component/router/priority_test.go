package router

import (
	"math"
	"testing"

	commontypes "opencsg.com/csghub-server/common/types"
)

func upstreamByURL(url string) commontypes.UpstreamConfig {
	return commontypes.UpstreamConfig{URL: url, ModelName: "m", Enabled: true}
}

func TestEligibleUpstreamsForPriority(t *testing.T) {
	low := upstreamByURL("http://low")
	mid := upstreamByURL("http://mid")
	mid.Priority = 50
	high := upstreamByURL("http://high")
	high.Priority = 100

	t.Run("low priority user is confined to the low channel", func(t *testing.T) {
		got := EligibleUpstreamsForPriority([]commontypes.UpstreamConfig{high, mid, low}, 0)
		if len(got) != 1 || got[0].URL != low.URL {
			t.Fatalf("want only the low upstream, got %+v", got)
		}
	})

	t.Run("intermediate tier admits up to its level, high first", func(t *testing.T) {
		got := EligibleUpstreamsForPriority([]commontypes.UpstreamConfig{low, high, mid}, 50)
		if len(got) != 2 || got[0].URL != mid.URL || got[1].URL != low.URL {
			t.Fatalf("want [mid low], got %+v", got)
		}
	})

	t.Run("unrestricted user keeps every upstream sorted high first", func(t *testing.T) {
		got := EligibleUpstreamsForPriority([]commontypes.UpstreamConfig{low, high, mid}, math.MaxInt)
		if len(got) != 3 {
			t.Fatalf("want all upstreams, got %+v", got)
		}
		for i, want := range []string{high.URL, mid.URL, low.URL} {
			if got[i].URL != want {
				t.Fatalf("position %d: want %s, got %s", i, want, got[i].URL)
			}
		}
	})

	t.Run("equal priorities keep the incoming order", func(t *testing.T) {
		a := upstreamByURL("http://a")
		b := upstreamByURL("http://b")
		got := EligibleUpstreamsForPriority([]commontypes.UpstreamConfig{b, a}, 0)
		if len(got) != 2 || got[0].URL != b.URL || got[1].URL != a.URL {
			t.Fatalf("want stable [b a], got %+v", got)
		}
	})

	t.Run("no eligible upstream yields empty slice", func(t *testing.T) {
		got := EligibleUpstreamsForPriority([]commontypes.UpstreamConfig{high}, 0)
		if len(got) != 0 {
			t.Fatalf("want empty, got %+v", got)
		}
	})
}

func TestPreferTopPriorityTier(t *testing.T) {
	low := upstreamByURL("http://low")
	highA := upstreamByURL("http://high-a")
	highA.Priority = 100
	highB := upstreamByURL("http://high-b")
	highB.Priority = 100

	t.Run("keeps only the max priority tier", func(t *testing.T) {
		got := TopPriorityTier([]commontypes.UpstreamConfig{low, highA, highB})
		if len(got) != 2 || got[0].URL != highA.URL || got[1].URL != highB.URL {
			t.Fatalf("want the two high upstreams, got %+v", got)
		}
	})

	t.Run("all same tier returns list unchanged", func(t *testing.T) {
		got := TopPriorityTier([]commontypes.UpstreamConfig{low})
		if len(got) != 1 || got[0].URL != low.URL {
			t.Fatalf("want unchanged list, got %+v", got)
		}
	})
}

func TestPickUpstreamPrefersHighPriorityTierWithSessionAffinity(t *testing.T) {
	r := NewSessionRouter()
	low := upstreamByURL("http://low")
	high := upstreamByURL("http://high")
	high.Priority = 100
	policy := commontypes.RoutingPolicy{Strategy: RoutingStrategySessionHash, HashReplicas: 64}

	// Same session key always lands on the high-priority upstream while it is
	// a candidate, regardless of where the hash would fall in a mixed ring.
	for i := 0; i < 5; i++ {
		got, err := r.PickUpstream("model", "session-1", []commontypes.UpstreamConfig{low, high}, policy)
		if err != nil {
			t.Fatal(err)
		}
		if got.URL != high.URL {
			t.Fatalf("iteration %d: want high upstream, got %s", i, got.URL)
		}
	}

	// With the high tier gone the same session falls back to the low tier.
	got, err := r.PickUpstream("model", "session-1", []commontypes.UpstreamConfig{low}, policy)
	if err != nil {
		t.Fatal(err)
	}
	if got.URL != low.URL {
		t.Fatalf("want low upstream fallback, got %s", got.URL)
	}
}
