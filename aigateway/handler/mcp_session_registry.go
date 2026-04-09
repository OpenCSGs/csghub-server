//go:build ee || saas

package handler

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"
	"opencsg.com/csghub-server/builder/store/cache"
)

const (
	mcpSessionKeyPrefix  = "mcp:session:"
	mcpInstanceKeyPrefix = "mcp:instance:"
	defaultSessionTTL    = 30 * time.Minute
)

// MCPSessionRegistry tracks which AIGateway instance owns each MCP session.
type MCPSessionRegistry interface {
	Register(ctx context.Context, sessionID, instanceAddr string) error
	Lookup(ctx context.Context, sessionID string) (instanceAddr string, err error)
	Delete(ctx context.Context, sessionID string) error
	DeleteByInstance(ctx context.Context, instanceAddr string) error
}

type redisMCPSessionRegistry struct {
	redis cache.RedisClient
	ttl   time.Duration
}

func NewMCPSessionRegistry(redis cache.RedisClient) MCPSessionRegistry {
	return &redisMCPSessionRegistry{
		redis: redis,
		ttl:   defaultSessionTTL,
	}
}

func (r *redisMCPSessionRegistry) Register(ctx context.Context, sessionID, instanceAddr string) error {
	sessionKey := mcpSessionKeyPrefix + sessionID
	instanceKey := mcpInstanceKeyPrefix + instanceAddr

	_, err := r.redis.Pipelined(ctx, func(pipe redis.Pipeliner) error {
		pipe.SetEx(ctx, sessionKey, instanceAddr, r.ttl)
		pipe.SAdd(ctx, instanceKey, sessionID)
		pipe.Expire(ctx, instanceKey, 2*r.ttl)
		return nil
	})
	if err != nil {
		return fmt.Errorf("register mcp session %s: %w", sessionID, err)
	}
	return nil
}

func (r *redisMCPSessionRegistry) Lookup(ctx context.Context, sessionID string) (string, error) {
	sessionKey := mcpSessionKeyPrefix + sessionID
	addr, err := r.redis.Get(ctx, sessionKey)
	if err != nil {
		return "", fmt.Errorf("lookup mcp session %s: %w", sessionID, err)
	}
	// Refresh TTL on access
	_ = r.redis.Expire(ctx, sessionKey, r.ttl)
	return addr, nil
}

func (r *redisMCPSessionRegistry) Delete(ctx context.Context, sessionID string) error {
	sessionKey := mcpSessionKeyPrefix + sessionID
	addr, err := r.redis.Get(ctx, sessionKey)
	if err == nil && addr != "" {
		instanceKey := mcpInstanceKeyPrefix + addr
		_, err = r.redis.Pipelined(ctx, func(pipe redis.Pipeliner) error {
			pipe.SRem(ctx, instanceKey, sessionID)
			pipe.Del(ctx, sessionKey)
			return nil
		})
		return err
	}
	return r.redis.Del(ctx, sessionKey)
}

func (r *redisMCPSessionRegistry) DeleteByInstance(ctx context.Context, instanceAddr string) error {
	instanceKey := mcpInstanceKeyPrefix + instanceAddr
	sessionIDs, err := r.redis.SMembers(ctx, instanceKey)
	if err != nil {
		slog.WarnContext(ctx, "failed to list sessions for instance cleanup", "instance", instanceAddr, "error", err)
		return err
	}
	_, err = r.redis.Pipelined(ctx, func(pipe redis.Pipeliner) error {
		for _, sid := range sessionIDs {
			pipe.Del(ctx, mcpSessionKeyPrefix+sid)
		}
		pipe.Del(ctx, instanceKey)
		return nil
	})
	return err
}
