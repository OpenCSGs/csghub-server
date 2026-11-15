package analytics

import "sync"

type Config struct {
	Enabled      bool
	ProjectToken string
	APIHost      string
	Environment  string
}

type Event struct {
	Name          string
	DistinctID    string
	SessionID     string
	CorrelationID string
	InsertID      string
	Properties    map[string]any
}

type Publisher interface {
	Capture(Event) error
	Close() error
}

type noOpPublisher struct{}

// Available reports whether a publisher has an initialized event transport.
// It does not guarantee remote delivery. Publishers without explicit status
// are treated as unavailable for operations that persist delivery claims.
func Available(publisher interface{ Capture(Event) error }) bool {
	status, ok := publisher.(interface{ Available() bool })
	return ok && status.Available()
}

var (
	defaultMu        sync.RWMutex
	defaultPublisher Publisher = noOpPublisher{}
)

func New(cfg Config) (Publisher, error) {
	return newPublisher(cfg)
}

func Default() Publisher {
	defaultMu.RLock()
	defer defaultMu.RUnlock()
	return defaultPublisher
}

func Assign(publisher Publisher) {
	if publisher == nil {
		publisher = noOpPublisher{}
	}
	defaultMu.Lock()
	defaultPublisher = publisher
	defaultMu.Unlock()
}

func (noOpPublisher) Capture(Event) error { return nil }
func (noOpPublisher) Close() error        { return nil }
func (noOpPublisher) Available() bool     { return false }
