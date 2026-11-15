package analytics

import "testing"

type captureOnlyPublisher struct{}

func (captureOnlyPublisher) Capture(Event) error { return nil }

func TestAvailableRequiresExplicitTransportStatus(t *testing.T) {
	if Available(nil) || Available(noOpPublisher{}) || Available(captureOnlyPublisher{}) {
		t.Fatal("a missing or no-op transport must not permit persistent claims")
	}
}
