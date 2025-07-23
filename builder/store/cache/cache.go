package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisClient interface {
	FlushAll(ctx context.Context) error
	ZAdd(ctx context.Context, key string, z redis.Z) error
	BZPopMax(ctx context.Context, key string) (*redis.ZWithKey, error)
	Set(ctx context.Context, key string, value string) error
	SetEx(ctx context.Context, key string, value string, expiration time.Duration) error
	Get(ctx context.Context, key string) (string, error)
	Del(ctx context.Context, keys ...string) error
	SAdd(ctx context.Context, key string, members ...interface{}) error
	SIsMember(ctx context.Context, key string, member interface{}) (bool, error)
	SMembers(ctx context.Context, key string) ([]string, error)
	SRem(ctx context.Context, key string, members ...interface{}) error
	SCard(ctx context.Context, key string) (int64, error)
	ZRevRange(ctx context.Context, key string, start, stop int64) ([]string, error)
	RunWhileLocked(ctx context.Context, resourceName string, expiration time.Duration, fn func(ctx context.Context) error) error
	WaitLockToRun(ctx context.Context, resourceName string, expiration time.Duration, fn func(ctx context.Context) error) error
	HSet(ctx context.Context, key string, field string, value interface{}) error
	HGetAll(ctx context.Context, key string) (map[string]string, error)
	HDel(ctx context.Context, key string, fields ...string) error
	ZRem(ctx context.Context, key string, value string) error
	RPush(ctx context.Context, key string, values ...interface{}) error
	LPopCount(ctx context.Context, key string, count int) ([]string, error)
	LPop(ctx context.Context, key string) (string, error)
	LPush(ctx context.Context, key string, values ...interface{}) error
}

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

func NewCache(ctx context.Context, cfg RedisConfig) (cache RedisClient, err error) {
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
	client := redis.NewClient(&redis.Options{
		Addr:     cfg.Addr,
		Username: cfg.Username,
		Password: cfg.Password,
		DB:       cfg.DB,
	})
	err = client.Ping(ctx).Err()
	if err != nil {
		err = fmt.Errorf("pinging Redis: %w", err)
		return
	}
	cache = &Cache{
		core:              client,
		releaseLockScript: redis.NewScript(releaseLockScript),
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

func (c *Cache) BZPopMax(ctx context.Context, key string) (*redis.ZWithKey, error) {
	return c.core.BZPopMax(ctx, time.Second*10, key).Result()
}

func (c *Cache) Set(ctx context.Context, key string, value string) error {
	return c.core.Set(ctx, key, value, 0).Err()
}

func (c *Cache) SetEx(ctx context.Context, key string, value string, expiration time.Duration) error {
	return c.core.SetEx(ctx, key, value, expiration).Err()
}

func (c *Cache) Get(ctx context.Context, key string) (string, error) {
	return c.core.Get(ctx, key).Result()
}

func (c *Cache) Del(ctx context.Context, keys ...string) error {
	return c.core.Del(ctx, keys...).Err()
}

func (c *Cache) SAdd(ctx context.Context, key string, members ...interface{}) error {
	return c.core.SAdd(ctx, key, members...).Err()
}

func (c *Cache) SIsMember(ctx context.Context, key string, member interface{}) (bool, error) {
	return c.core.SIsMember(ctx, key, member).Result()
}

func (c *Cache) SMembers(ctx context.Context, key string) ([]string, error) {
	return c.core.SMembers(ctx, key).Result()
}

func (c *Cache) SCard(ctx context.Context, key string) (int64, error) {
	return c.core.SCard(ctx, key).Result()
}

func (c *Cache) ZRevRange(ctx context.Context, key string, start, stop int64) ([]string, error) {
	return c.core.ZRevRange(ctx, key, start, stop).Result()
}

func (c *Cache) HSet(ctx context.Context, key string, field string, value interface{}) error {
	return c.core.HSet(ctx, key, field, value).Err()
}

func (c *Cache) HGetAll(ctx context.Context, key string) (map[string]string, error) {
	return c.core.HGetAll(ctx, key).Result()
}

func (c *Cache) HDel(ctx context.Context, key string, fields ...string) error {
	return c.core.HDel(ctx, key, fields...).Err()
}

func (c *Cache) ZRem(ctx context.Context, key string, value string) error {
	return c.core.ZRem(ctx, key, value).Err()
}

func (c *Cache) RPush(ctx context.Context, key string, values ...interface{}) error {
	return c.core.RPush(ctx, key, values...).Err()
}

func (c *Cache) LPush(ctx context.Context, key string, values ...interface{}) error {
	return c.core.LPush(ctx, key, values...).Err()
}

func (c *Cache) LPopCount(ctx context.Context, key string, count int) ([]string, error) {
	return c.core.LPopCount(ctx, key, count).Result()
}

func (c *Cache) LPop(ctx context.Context, key string) (string, error) {
	return c.core.LPop(ctx, key).Result()
}

func (c *Cache) SRem(ctx context.Context, key string, values ...interface{}) error {
	return c.core.SRem(ctx, key, values...).Err()
}
