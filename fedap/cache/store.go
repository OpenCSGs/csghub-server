package cache

import (
	"sync"
	"time"
)

type entry[V any] struct {
	value     V
	createdAt time.Time
	ttl       time.Duration // zero means no expiration
}

func (e *entry[V]) expired(now time.Time) bool {
	return e.ttl > 0 && now.After(e.createdAt.Add(e.ttl))
}

// Store is a thread-safe in-memory key-value store with optional per-item TTL.
// Items stored with a positive TTL are automatically removed by a background
// goroutine. Items stored with zero TTL live until explicitly deleted.
type Store[K comparable, V any] struct {
	mu      sync.RWMutex
	items   map[K]*entry[V]
	closeCh chan struct{}
}

// New creates a Store and starts a background cleanup goroutine that sweeps
// expired items at the given interval. Pass 0 to disable background cleanup.
func New[K comparable, V any](cleanupInterval time.Duration) *Store[K, V] {
	s := &Store[K, V]{
		items:   make(map[K]*entry[V]),
		closeCh: make(chan struct{}),
	}
	if cleanupInterval > 0 {
		go s.cleanupLoop(cleanupInterval)
	}
	return s
}

// Save stores a value with a TTL. Pass 0 for ttl to store without expiration.
func (s *Store[K, V]) Save(key K, value V, ttl time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items[key] = &entry[V]{
		value:     value,
		createdAt: time.Now(),
		ttl:       ttl,
	}
}

// Load retrieves a value by key. Returns false if the key does not exist or has expired.
func (s *Store[K, V]) Load(key K) (V, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	e, ok := s.items[key]
	if !ok || e.expired(time.Now()) {
		var zero V
		return zero, false
	}
	return e.value, true
}

// LoadAndDelete retrieves and removes a value by key (one-time use).
// Returns false if the key does not exist or has expired.
func (s *Store[K, V]) LoadAndDelete(key K) (V, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.items[key]
	if !ok || e.expired(time.Now()) {
		var zero V
		return zero, false
	}
	delete(s.items, key)
	return e.value, true
}

// Delete removes a value by key.
func (s *Store[K, V]) Delete(key K) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.items, key)
}

// Len returns the number of items (including expired but not yet cleaned up).
func (s *Store[K, V]) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.items)
}

// Close stops the background cleanup goroutine.
func (s *Store[K, V]) Close() {
	close(s.closeCh)
}

func (s *Store[K, V]) cleanupLoop(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			s.cleanup()
		case <-s.closeCh:
			return
		}
	}
}

func (s *Store[K, V]) cleanup() {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	for key, e := range s.items {
		if e.expired(now) {
			delete(s.items, key)
		}
	}
}
