package cache

import (
	"context"
	"fmt"
	"strconv"

	"opencsg.com/csghub-server/builder/store/cache"
	"opencsg.com/csghub-server/common/config"
)

const (
	OssMultipartUploadKeyPrefix = "oss-multipart-upload"
	LfsSyncKeyPrefix            = "lfs-sync"
	LfsSyncProgressKeyPrefix    = "lfs-sync-progress"
	LfsRunningTaskKey           = "lfs-running-task"
	UploadIDKeyPrefix           = "upload-id"
)

type Cache interface {
	CacheUploadID(ctx context.Context, repoPath, oid string, uploadID string) error
	GetUploadID(ctx context.Context, repoPath, oid string) (string, error)
	DeleteUploadID(ctx context.Context, repoPath, oid string) error

	CacheLfsSyncAddPart(ctx context.Context, repoPath, oid string, partNumber int) error
	IsLfsPartSynced(ctx context.Context, repoPath, oid string, partNumber int) (bool, error)
	LfsPartSyncedCount(ctx context.Context, repoPath, oid string) (int64, error)
	DeleteLfsPartCache(ctx context.Context, repoPath, oid string) error
	DeleteSpecificLfsPartCache(ctx context.Context, repoPath, oid string, partNumber int) error

	CacheLfsSyncFileProgress(ctx context.Context, repoPath, oid string, progress int) error
	DeleteLfsSyncFileProgress(ctx context.Context, repoPath, oid string) error
	GetLfsSyncFileProgress(ctx context.Context, repoPath, oid string) (int, error)

	DeleteAllCache(ctx context.Context, repoPath, oid string) error
}

type cacheImpl struct {
	redis cache.RedisClient
}

func NewCache(ctx context.Context, config *config.Config) (Cache, error) {
	redis, err := cache.NewCache(ctx, cache.RedisConfig{
		Addr:     config.Redis.Endpoint,
		Username: config.Redis.User,
		Password: config.Redis.Password,
	})
	if err != nil {
		return nil, fmt.Errorf("initializing redis: %w", err)
	}
	return &cacheImpl{
		redis: redis,
	}, nil
}

func (c *cacheImpl) DeleteAllCache(ctx context.Context, repoPath, oid string) error {
	uploadIDkey := uploadIDCacheKey(repoPath, oid)
	lfsPartCacheKey := lfsPartCacheKey(repoPath, oid)
	lfsProgressCacheKey := lfsProgressCacheKey(repoPath, oid)
	return c.redis.Del(ctx, lfsProgressCacheKey, uploadIDkey, lfsPartCacheKey)
}

func (c *cacheImpl) CacheUploadID(ctx context.Context, repoPath, oid string, uploadID string) error {
	key := uploadIDCacheKey(repoPath, oid)
	return c.redis.Set(ctx, key, uploadID)
}

func (c *cacheImpl) GetUploadID(ctx context.Context, repoPath, oid string) (string, error) {
	key := uploadIDCacheKey(repoPath, oid)
	return c.redis.Get(ctx, key)
}

func (c *cacheImpl) DeleteUploadID(ctx context.Context, repoPath, oid string) error {
	key := uploadIDCacheKey(repoPath, oid)
	return c.redis.Del(ctx, key)
}

func (c *cacheImpl) DeleteImur(ctx context.Context, repoPath, oid string) error {
	key := imurKeyCacheKey(repoPath, oid)
	return c.redis.Del(ctx, key)
}

func (c *cacheImpl) CacheLfsSyncAddPart(ctx context.Context, repoPath, oid string, partNumber int) error {
	key := lfsPartCacheKey(repoPath, oid)
	err := c.redis.SAdd(ctx, key, partNumber)
	if err != nil {
		return fmt.Errorf("failed to add lfs part number to set:  %w", err)
	}
	return nil
}

func (c *cacheImpl) IsLfsPartSynced(ctx context.Context, repoPath, oid string, partNumber int) (bool, error) {
	key := lfsPartCacheKey(repoPath, oid)
	return c.redis.SIsMember(ctx, key, partNumber)
}

func (c *cacheImpl) LfsPartSyncedCount(ctx context.Context, repoPath, oid string) (int64, error) {
	key := lfsPartCacheKey(repoPath, oid)
	return c.redis.SCard(ctx, key)
}

func (c *cacheImpl) DeleteLfsPartCache(ctx context.Context, repoPath, oid string) error {
	key := lfsPartCacheKey(repoPath, oid)
	return c.redis.Del(ctx, key)
}

func (c *cacheImpl) CacheLfsSyncFileProgress(ctx context.Context, repoPath, oid string, progress int) error {
	key := lfsProgressCacheKey(repoPath, oid)
	strProgress := strconv.Itoa(progress)
	err := c.redis.Set(ctx, key, strProgress)
	if err != nil {
		return fmt.Errorf("failed to set lfs part number to set:   %w", err)
	}
	return nil
}

func (c *cacheImpl) DeleteLfsSyncFileProgress(ctx context.Context, repoPath, oid string) error {
	key := lfsProgressCacheKey(repoPath, oid)
	return c.redis.Del(ctx, key)
}

func (c *cacheImpl) GetLfsSyncFileProgress(ctx context.Context, repoPath, oid string) (int, error) {
	key := lfsProgressCacheKey(repoPath, oid)
	strProgress, err := c.redis.Get(ctx, key)
	if err != nil {
		return 0, fmt.Errorf("failed to get lfs part number:   %w", err)
	}
	return strconv.Atoi(strProgress)
}

func (c *cacheImpl) DeleteSpecificLfsPartCache(ctx context.Context, repoPath, oid string, partNumber int) error {
	key := lfsPartCacheKey(repoPath, oid)
	return c.redis.SRem(ctx, key, partNumber)
}
func lfsProgressCacheKey(repoPath, oid string) string {
	return fmt.Sprintf("%s-%s-%s", LfsSyncProgressKeyPrefix, repoPath, oid)
}
func lfsPartCacheKey(repoPath, oid string) string {
	return fmt.Sprintf("%s-%s-%s", LfsSyncKeyPrefix, repoPath, oid)
}

func imurKeyCacheKey(repoPath, oid string) string {
	return fmt.Sprintf("%s-%s-%s", OssMultipartUploadKeyPrefix, repoPath, oid)
}

func uploadIDCacheKey(repoPath, oid string) string {
	return fmt.Sprintf("%s-%s-%s", UploadIDKeyPrefix, repoPath, oid)
}
