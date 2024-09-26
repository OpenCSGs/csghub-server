package cache

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
)

type RedisConfig struct {
	Addr     string `comment:"Redis address, e.g. localhost:6379"`
	Username string `comment:"optional, Redis username"`
	Password string `comment:"optional, Redis password"`
	DB       int    `comment:"optional, Redis DB"`
}

type Cache struct {
	core              *redis.Client
	releaseLockScript *redis.Script
}

func NewCache(ctx context.Context, cfg RedisConfig) (cache *Cache, err error) {
	const releaseLockScript = `
local value = redis.call("GET", KEYS[1])
if not value then
	return -1 -- not locked
end
if value == ARGV[1] then
	return redis.call("DEL",KEYS[1]) -- lock is successfully released
else
	return 0 -- lock does not belongs to us
end`
	cache = &Cache{
		core: redis.NewClient(&redis.Options{
			Addr:     cfg.Addr,
			Username: cfg.Username,
			Password: cfg.Password,
			DB:       cfg.DB,
		}),
		releaseLockScript: redis.NewScript(releaseLockScript),
	}
	err = cache.core.Ping(ctx).Err()
	if err != nil {
		err = fmt.Errorf("pinging Redis: %w", err)
		return
	}
	return
}

func (c *Cache) FlushAll(ctx context.Context) error {
	return c.core.FlushAll(ctx).Err()
}

func (c *Cache) ZAdd(ctx context.Context, key string, z redis.Z) error {
	_, err := c.core.ZAdd(ctx, key, z).Result()
	return err
}

func (c *Cache) ZPopMax(ctx context.Context, key string, count int64) ([]redis.Z, error) {
	return c.core.ZPopMax(ctx, key, count).Result()
}
