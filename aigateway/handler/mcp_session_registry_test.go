//go:build ee || saas

package handler

import (
	"context"
	"fmt"
	"testing"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/mock"
	mockcache "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/store/cache"
)

func TestRedisMCPSessionRegistry_RegisterAndLookup(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	mockRedis := mockcache.NewMockRedisClient(t)
	registry := NewMCPSessionRegistry(mockRedis)

	sessionID := "test-session-123"
	addr := "192.168.1.10:8094"

	mockRedis.EXPECT().Pipelined(ctx, mock.AnythingOfType("func(redis.Pipeliner) error")).
		Return([]redis.Cmder{}, nil)

	err := registry.Register(ctx, sessionID, addr)
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	mockRedis.EXPECT().Get(ctx, "mcp:session:"+sessionID).Return(addr, nil)
	mockRedis.EXPECT().Expire(ctx, "mcp:session:"+sessionID, defaultSessionTTL).Return(nil)

	gotAddr, err := registry.Lookup(ctx, sessionID)
	if err != nil {
		t.Fatalf("Lookup() error = %v", err)
	}
	if gotAddr != addr {
		t.Fatalf("Lookup() = %q, want %q", gotAddr, addr)
	}
}

func TestRedisMCPSessionRegistry_Delete(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	mockRedis := mockcache.NewMockRedisClient(t)
	registry := NewMCPSessionRegistry(mockRedis)

	sessionID := "test-session-456"
	addr := "192.168.1.10:8094"

	mockRedis.EXPECT().Get(ctx, "mcp:session:"+sessionID).Return(addr, nil)
	mockRedis.EXPECT().Pipelined(ctx, mock.AnythingOfType("func(redis.Pipeliner) error")).
		Return([]redis.Cmder{}, nil)

	err := registry.Delete(ctx, sessionID)
	if err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
}

func TestRedisMCPSessionRegistry_DeleteByInstance(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	mockRedis := mockcache.NewMockRedisClient(t)
	registry := NewMCPSessionRegistry(mockRedis)

	addr := "192.168.1.10:8094"
	sessions := []string{"s1", "s2", "s3"}

	mockRedis.EXPECT().SMembers(ctx, "mcp:instance:"+addr).Return(sessions, nil)
	mockRedis.EXPECT().Pipelined(ctx, mock.AnythingOfType("func(redis.Pipeliner) error")).
		Return([]redis.Cmder{}, nil)

	err := registry.DeleteByInstance(ctx, addr)
	if err != nil {
		t.Fatalf("DeleteByInstance() error = %v", err)
	}
}

func TestRedisMCPSessionRegistry_RegisterError(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	mockRedis := mockcache.NewMockRedisClient(t)
	registry := NewMCPSessionRegistry(mockRedis)

	mockRedis.EXPECT().Pipelined(ctx, mock.AnythingOfType("func(redis.Pipeliner) error")).
		Return(nil, fmt.Errorf("redis down"))

	err := registry.Register(ctx, "s1", "addr1")
	if err == nil {
		t.Fatal("expected error from Register")
	}
}

func TestRedisMCPSessionRegistry_LookupNotFound(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	mockRedis := mockcache.NewMockRedisClient(t)
	registry := NewMCPSessionRegistry(mockRedis)

	mockRedis.EXPECT().Get(ctx, "mcp:session:missing").Return("", fmt.Errorf("redis: nil"))

	_, err := registry.Lookup(ctx, "missing")
	if err == nil {
		t.Fatal("expected error from Lookup for missing session")
	}
}
