package cache

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestStore_SaveAndLoad(t *testing.T) {
	s := New[string, int](0)
	defer s.Close()

	s.Save("a", 1, 0)
	v, ok := s.Load("a")
	assert.True(t, ok)
	assert.Equal(t, 1, v)
}

func TestStore_LoadMissing(t *testing.T) {
	s := New[string, int](0)
	defer s.Close()

	_, ok := s.Load("missing")
	assert.False(t, ok)
}

func TestStore_LoadAndDelete(t *testing.T) {
	s := New[string, string](0)
	defer s.Close()

	s.Save("state", "session-data", 0)

	v, ok := s.LoadAndDelete("state")
	assert.True(t, ok)
	assert.Equal(t, "session-data", v)

	_, ok = s.LoadAndDelete("state")
	assert.False(t, ok, "second LoadAndDelete should return false")
}

func TestStore_Delete(t *testing.T) {
	s := New[string, int](0)
	defer s.Close()

	s.Save("k", 42, 0)
	s.Delete("k")

	_, ok := s.Load("k")
	assert.False(t, ok)
}

func TestStore_TTLExpiration(t *testing.T) {
	s := New[string, int](0)
	defer s.Close()

	s.Save("short", 1, 50*time.Millisecond)

	v, ok := s.Load("short")
	assert.True(t, ok)
	assert.Equal(t, 1, v)

	time.Sleep(60 * time.Millisecond)

	_, ok = s.Load("short")
	assert.False(t, ok, "expired item should not be returned by Load")

	_, ok = s.LoadAndDelete("short")
	assert.False(t, ok, "expired item should not be returned by LoadAndDelete")
}

func TestStore_NoTTLNeverExpires(t *testing.T) {
	s := New[string, int](0)
	defer s.Close()

	s.Save("permanent", 99, 0)

	time.Sleep(50 * time.Millisecond)

	v, ok := s.Load("permanent")
	assert.True(t, ok)
	assert.Equal(t, 99, v)
}

func TestStore_CleanupRemovesExpired(t *testing.T) {
	s := New[string, int](0)
	defer s.Close()

	s.Save("expired", 1, 10*time.Millisecond)
	s.Save("permanent", 2, 0)

	time.Sleep(20 * time.Millisecond)
	s.cleanup()

	assert.Equal(t, 1, s.Len(), "cleanup should remove expired item, keep permanent")
	_, ok := s.Load("permanent")
	assert.True(t, ok)
}

func TestStore_CleanupLoop(t *testing.T) {
	s := New[string, int](50 * time.Millisecond)
	defer s.Close()

	s.Save("temp", 1, 10*time.Millisecond)

	time.Sleep(120 * time.Millisecond)

	assert.Equal(t, 0, s.Len(), "cleanup loop should have removed expired item")
}

func TestStore_Len(t *testing.T) {
	s := New[int, string](0)
	defer s.Close()

	assert.Equal(t, 0, s.Len())
	s.Save(1, "a", 0)
	s.Save(2, "b", 0)
	assert.Equal(t, 2, s.Len())
}

func TestStore_Overwrite(t *testing.T) {
	s := New[string, int](0)
	defer s.Close()

	s.Save("k", 1, 0)
	s.Save("k", 2, 0)

	v, ok := s.Load("k")
	assert.True(t, ok)
	assert.Equal(t, 2, v)
}

func TestStore_IntKey(t *testing.T) {
	s := New[int, string](0)
	defer s.Close()

	s.Save(42, "hello", 0)
	v, ok := s.Load(42)
	assert.True(t, ok)
	assert.Equal(t, "hello", v)
}

func TestStore_StructValue(t *testing.T) {
	type session struct {
		userID   string
		verifier string
	}
	s := New[string, *session](0)
	defer s.Close()

	sess := &session{userID: "u1", verifier: "v1"}
	s.Save("state-abc", sess, 10*time.Minute)

	got, ok := s.LoadAndDelete("state-abc")
	assert.True(t, ok)
	assert.Equal(t, "u1", got.userID)
	assert.Equal(t, "v1", got.verifier)
}
