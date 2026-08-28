//go:build !ee && !saas

package analytics

// newPublisher intentionally ignores PostHog configuration in CE builds.
// Server-side product analytics is available only in EE and SaaS editions.
func newPublisher(Config) (Publisher, error) {
	return noOpPublisher{}, nil
}
