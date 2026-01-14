package cache

import (
	"context"
	"fmt"
	"strconv"

	"opencsg.com/csghub-server/common/config"
)

type LfsCacheImpl struct {
	cache RedisClient
}

type LfsCache interface {
	CacheLfsProgress(ctx context.Context, repoID int64, oid string, progress int) error
	GetLfsProgress(ctx context.Context, repoID int64, oid string) (int, error)
	DeleteLfsProgress(ctx context.Context, repoID int64, oid string) error
}

func NewLfsCache(config *config.Config) (LfsCache, error) {
	cacheClient, err := NewCache(context.Background(), RedisConfig{
		Addr:     config.Redis.Endpoint,
		Username: config.Redis.User,
		Password: config.Redis.Password,
	})
	if err != nil {
		return nil, err
	}
	return &LfsCacheImpl{
		cache: cacheClient,
	}, nil
}

func (l *LfsCacheImpl) CacheLfsProgress(ctx context.Context, repoID int64, oid string, progress int) error {
	key := lfsProgressCacheKey(repoID, oid)
	return l.cache.Set(ctx, key, strconv.Itoa(progress))
}
func (l *LfsCacheImpl) GetLfsProgress(ctx context.Context, repoID int64, oid string) (int, error) {
	key := lfsProgressCacheKey(repoID, oid)
	val, err := l.cache.Get(ctx, key)
	if err != nil {
		return 0, err
	}
	var progress int
	_, err = fmt.Sscanf(val, "%d", &progress)
	if err != nil {
		return 0, err
	}
	return progress, nil
}

func (l *LfsCacheImpl) DeleteLfsProgress(ctx context.Context, repoID int64, oid string) error {
	key := lfsProgressCacheKey(repoID, oid)
	return l.cache.Del(ctx, key)
}

func lfsProgressCacheKey(repoID int64, oid string) string {
	return fmt.Sprintf("xnet:migration:lfs:progress:%d:%s", repoID, oid)
}
