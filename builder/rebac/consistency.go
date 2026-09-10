package rebac

// Consistency represents an authorization read consistency level independent of a Provider.
type Consistency uint8

const (
	// ConsistencyDefault uses the Provider's default consistency strategy.
	ConsistencyDefault Consistency = iota
	// ConsistencyMinimizeLatency allows the Provider to prioritize lower latency.
	ConsistencyMinimizeLatency
	// ConsistencyHigher asks the Provider to prioritize the newest relationship data.
	ConsistencyHigher
)

// Valid reports whether the consistency value is supported by the public contract.
func (c Consistency) Valid() bool {
	return c <= ConsistencyHigher
}

// String returns the stable consistency name used by logs and adapters.
func (c Consistency) String() string {
	switch c {
	case ConsistencyDefault:
		return "default"
	case ConsistencyMinimizeLatency:
		return "minimize_latency"
	case ConsistencyHigher:
		return "higher"
	default:
		return "unknown"
	}
}
