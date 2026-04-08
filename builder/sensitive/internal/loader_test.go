package internal

import (
	"testing"

	"opencsg.com/csghub-server/common/config"
)

// testObserver is a mock observer for testing
type testObserver struct {
	UpdateCalled bool
	LastData     *SensitiveWordData
	UpdateCount  int
}

func (to *testObserver) Update(data *SensitiveWordData) error {
	to.UpdateCalled = true
	to.UpdateCount++
	to.LastData = data
	return nil
}

func TestConfigLoader(t *testing.T) {
	cfg := &config.Config{}
	cfg.SensitiveCheck.DictDir = "./config.yaml"

	loader := NewConfigLoader(cfg)
	observer := &testObserver{}

	loader.Subscribe(observer)

	data, err := loader.Load()
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if data == nil {
		t.Fatalf("expected data, got nil")
	}

	if !observer.UpdateCalled {
		t.Fatalf("observer was not notified")
	}

	if observer.UpdateCount != 1 {
		t.Fatalf("expected 1 update, got %d", observer.UpdateCount)
	}
}

func TestMultipleObservers(t *testing.T) {
	cfg := &config.Config{}
	cfg.SensitiveCheck.DictDir = "./config.yaml"

	loader := NewConfigLoader(cfg)

	observer1 := &testObserver{}
	observer2 := &testObserver{}

	loader.Subscribe(observer1)
	loader.Subscribe(observer2)

	_, err := loader.Load()
	if err != nil {
		t.Fatalf("failed to load: %v", err)
	}

	if !observer1.UpdateCalled || !observer2.UpdateCalled {
		t.Fatalf("not all observers were notified")
	}

	if observer1.UpdateCount != 1 || observer2.UpdateCount != 1 {
		t.Fatalf("expected 1 update each, got %d and %d", observer1.UpdateCount, observer2.UpdateCount)
	}
}

func TestUnsubscribe(t *testing.T) {
	cfg := &config.Config{}
	cfg.SensitiveCheck.DictDir = "./config.yaml"

	loader := NewConfigLoader(cfg)
	observer := &testObserver{}

	loader.Subscribe(observer)
	loader.Unsubscribe(observer)

	_, err := loader.Load()
	if err != nil {
		t.Fatalf("failed to load: %v", err)
	}

	if observer.UpdateCalled {
		t.Fatalf("unsubscribed observer was still notified")
	}
}
