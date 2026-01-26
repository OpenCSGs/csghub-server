package memory

import (
	"fmt"
	"strings"
	"time"

	"opencsg.com/csghub-server/builder/rpc"
)

type AdapterFactory func(endpoint, basePath string, opts ...rpc.RequestOption) Adapter

var registry = map[string]AdapterFactory{}

func RegisterAdapter(name string, factory AdapterFactory) {
	key := strings.ToLower(strings.TrimSpace(name))
	if key == "" {
		return
	}
	registry[key] = factory
}

func NewAdapter(name, endpoint, basePath string, opts ...rpc.RequestOption) (Adapter, error) {
	key := strings.ToLower(strings.TrimSpace(name))
	if key == "" {
		key = "memmachine"
	}
	factory, ok := registry[key]
	if !ok {
		return nil, fmt.Errorf("unsupported memory backend: %s", name)
	}
	return factory(endpoint, basePath, opts...), nil
}

type TimeoutSetter interface {
	WithTimeout(timeout time.Duration)
}

type RetrySetter interface {
	WithRetry(attempts uint)
}

type DelaySetter interface {
	WithDelay(delay time.Duration)
}
