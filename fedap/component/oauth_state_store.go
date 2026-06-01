//go:build saas

package component

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	storecache "opencsg.com/csghub-server/builder/store/cache"
)

const (
	oauthStateKeyPrefix = "fedap:oauth:state:"
)

type oauthStateStore interface {
	Save(ctx context.Context, state string, session *authSession, ttl time.Duration) error
	Load(ctx context.Context, state string) (*authSession, bool, error)
	LoadAndDelete(ctx context.Context, state string) (*authSession, bool, error)
}

type redisOAuthStateStore struct {
	redis storecache.RedisClient
}

func newRedisOAuthStateStore(redis storecache.RedisClient) oauthStateStore {
	return &redisOAuthStateStore{redis: redis}
}

func (s *redisOAuthStateStore) Save(ctx context.Context, state string, session *authSession, ttl time.Duration) error {
	payload, err := json.Marshal(session)
	if err != nil {
		return fmt.Errorf("marshal oauth state session: %w", err)
	}
	key := oauthStateKeyPrefix + state
	if err := s.redis.SetEx(ctx, key, string(payload), ttl); err != nil {
		return fmt.Errorf("save oauth state session: %w", err)
	}
	return nil
}

func (s *redisOAuthStateStore) Load(ctx context.Context, state string) (*authSession, bool, error) {
	payload, err := s.redis.Get(ctx, oauthStateKeyPrefix+state)
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("load oauth state session: %w", err)
	}

	var session authSession
	if err := json.Unmarshal([]byte(payload), &session); err != nil {
		return nil, false, fmt.Errorf("unmarshal oauth state session: %w", err)
	}

	return &session, true, nil
}

func (s *redisOAuthStateStore) LoadAndDelete(ctx context.Context, state string) (*authSession, bool, error) {
	payload, err := s.redis.GetDel(ctx, oauthStateKeyPrefix+state)
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("pop oauth state session: %w", err)
	}

	var session authSession
	if err := json.Unmarshal([]byte(payload), &session); err != nil {
		return nil, false, fmt.Errorf("unmarshal oauth state session: %w", err)
	}

	return &session, true, nil
}
