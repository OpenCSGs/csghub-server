package cache

import (
	"context"
	"fmt"
	"strconv"

	"github.com/redis/go-redis/v9"
	"opencsg.com/csghub-server/builder/store/cache"
	"opencsg.com/csghub-server/common/config"
)

const (
	// LfsSyncKeyPrefix is the Redis key prefix for LFS sync cache entries.
	LfsSyncKeyPrefix = "lfssyncer"
	// LfsRunningTaskKey stores the mapping between legacy worker IDs and mirror IDs.
	LfsRunningTaskKey = "lfs-running-task"
	// repoSyncCacheDeleteBatchSize limits the number of Redis keys deleted per DEL call.
	repoSyncCacheDeleteBatchSize = 100
)

// Cache defines Redis-backed cache operations used by mirror LFS synchronization.
type Cache interface {
	// CacheUploadID stores the multipart upload ID for one LFS object.
	CacheUploadID(ctx context.Context, repoID int64, oid, partSize string, uploadID string) error
	// GetUploadID returns the multipart upload ID for one LFS object.
	GetUploadID(ctx context.Context, repoID int64, oid, partSize string) (string, error)
	// DeleteUploadID removes the multipart upload ID for one LFS object.
	DeleteUploadID(ctx context.Context, repoID int64, oid, partSize string) error

	// CacheLfsSyncAddPart marks one multipart part as uploaded.
	CacheLfsSyncAddPart(ctx context.Context, repoID int64, oid, partSize string, partNumber int) error
	// IsLfsPartSynced reports whether one multipart part has already been uploaded.
	IsLfsPartSynced(ctx context.Context, repoID int64, oid, partSize string, partNumber int) (bool, error)
	// LfsPartSyncedCount returns the number of uploaded multipart parts for one LFS object.
	LfsPartSyncedCount(ctx context.Context, repoID int64, oid, partSize string) (int64, error)
	// DeleteLfsPartCache removes all uploaded part markers for one LFS object.
	DeleteLfsPartCache(ctx context.Context, repoID int64, oid, partSize string) error
	// DeleteSpecificLfsPartCache removes one uploaded part marker for one LFS object.
	DeleteSpecificLfsPartCache(ctx context.Context, repoID int64, oid, partSize string, partNumber int) error

	// CacheLfsSyncFileProgress stores the upload progress percentage for one LFS object.
	CacheLfsSyncFileProgress(ctx context.Context, repoID int64, oid, partSize string, progress int) error
	// GetLfsSyncFileProgress returns the upload progress percentage for one LFS object.
	GetLfsSyncFileProgress(ctx context.Context, repoID int64, oid, partSize string) (int, error)

	// CacheRunningTask stores the mirror task currently handled by one legacy worker.
	CacheRunningTask(ctx context.Context, workID int, mirrorID int64) error
	// RemoveRunningTask removes the running task marker for one legacy worker.
	RemoveRunningTask(ctx context.Context, workID int) error
	// DeleteLfsSyncFileCache removes upload, progress, and part cache for one LFS file.
	DeleteLfsSyncFileCache(ctx context.Context, repoID int64, oid, partSize string) error
	// DeleteRepoSyncCache removes all LFS sync cache for a repository under one part size.
	DeleteRepoSyncCache(ctx context.Context, repoID int64, partSize string) error
}

// cacheImpl implements Cache with Redis.
type cacheImpl struct {
	redis cache.RedisClient
}

// NewCache creates a Redis-backed mirror LFS cache.
func NewCache(ctx context.Context, config *config.Config) (Cache, error) {
	rdb, err := cache.NewCache(ctx, cache.RedisConfig{
		Addr:     config.Redis.Endpoint,
		Username: config.Redis.User,
		Password: config.Redis.Password,
	})
	if err != nil {
		return nil, fmt.Errorf("initializing redis: %w", err)
	}
	return &cacheImpl{
		redis: rdb,
	}, nil
}

// DeleteLfsSyncFileCache removes upload, progress, and part cache for one LFS file.
func (c *cacheImpl) DeleteLfsSyncFileCache(ctx context.Context, repoID int64, oid, partSize string) error {
	_, err := c.redis.Pipelined(ctx, func(pipe redis.Pipeliner) error {
		pipe.HDel(ctx, lfsUploadIDCacheKey(repoID, partSize), oid)
		pipe.HDel(ctx, lfsProgressCacheKey(repoID, partSize), oid)
		pipe.Del(ctx, lfsPartCacheKey(repoID, partSize, oid))
		return nil
	})
	return err
}

// DeleteRepoSyncCache removes all LFS sync cache for a repository under one part size.
func (c *cacheImpl) DeleteRepoSyncCache(ctx context.Context, repoID int64, partSize string) error {
	uploadIDKey := lfsUploadIDCacheKey(repoID, partSize)
	progressKey := lfsProgressCacheKey(repoID, partSize)
	uploads, err := c.redis.HGetAll(ctx, uploadIDKey)
	if err != nil {
		return err
	}

	keys := make([]string, 0, len(uploads)+2)
	for oid := range uploads {
		keys = append(keys, lfsPartCacheKey(repoID, partSize, oid))
	}
	keys = append(keys, uploadIDKey, progressKey)

	_, err = c.redis.Pipelined(ctx, func(pipe redis.Pipeliner) error {
		for start := 0; start < len(keys); start += repoSyncCacheDeleteBatchSize {
			end := start + repoSyncCacheDeleteBatchSize
			if end > len(keys) {
				end = len(keys)
			}
			pipe.Del(ctx, keys[start:end]...)
		}
		return nil
	})
	return err
}

// CacheUploadID stores the multipart upload ID for one LFS object.
func (c *cacheImpl) CacheUploadID(ctx context.Context, repoID int64, oid, partSize string, uploadID string) error {
	return c.redis.HSet(ctx, lfsUploadIDCacheKey(repoID, partSize), oid, uploadID)
}

// GetUploadID returns the multipart upload ID for one LFS object.
func (c *cacheImpl) GetUploadID(ctx context.Context, repoID int64, oid, partSize string) (string, error) {
	return c.redis.HGet(ctx, lfsUploadIDCacheKey(repoID, partSize), oid)
}

// DeleteUploadID removes the multipart upload ID for one LFS object.
func (c *cacheImpl) DeleteUploadID(ctx context.Context, repoID int64, oid, partSize string) error {
	return c.redis.HDel(ctx, lfsUploadIDCacheKey(repoID, partSize), oid)
}

// CacheLfsSyncAddPart marks one multipart part as uploaded.
func (c *cacheImpl) CacheLfsSyncAddPart(ctx context.Context, repoID int64, oid, partSize string, partNumber int) error {
	key := lfsPartCacheKey(repoID, partSize, oid)
	err := c.redis.HSet(ctx, key, strconv.Itoa(partNumber), 1)
	if err != nil {
		return fmt.Errorf("failed to add lfs part number to hash: %w", err)
	}
	return nil
}

// IsLfsPartSynced reports whether one multipart part has already been uploaded.
func (c *cacheImpl) IsLfsPartSynced(ctx context.Context, repoID int64, oid, partSize string, partNumber int) (bool, error) {
	key := lfsPartCacheKey(repoID, partSize, oid)
	return c.redis.HExists(ctx, key, strconv.Itoa(partNumber))
}

// LfsPartSyncedCount returns the number of uploaded multipart parts for one LFS object.
func (c *cacheImpl) LfsPartSyncedCount(ctx context.Context, repoID int64, oid, partSize string) (int64, error) {
	key := lfsPartCacheKey(repoID, partSize, oid)
	return c.redis.HLen(ctx, key)
}

// DeleteLfsPartCache removes all uploaded part markers for one LFS object.
func (c *cacheImpl) DeleteLfsPartCache(ctx context.Context, repoID int64, oid, partSize string) error {
	key := lfsPartCacheKey(repoID, partSize, oid)
	return c.redis.Del(ctx, key)
}

// CacheLfsSyncFileProgress stores the upload progress percentage for one LFS object.
func (c *cacheImpl) CacheLfsSyncFileProgress(ctx context.Context, repoID int64, oid, partSize string, progress int) error {
	key := lfsProgressCacheKey(repoID, partSize)
	strProgress := strconv.Itoa(progress)
	err := c.redis.HSet(ctx, key, oid, strProgress)
	if err != nil {
		return fmt.Errorf("failed to set lfs progress: %w", err)
	}
	return nil
}

// GetLfsSyncFileProgress returns the upload progress percentage for one LFS object.
func (c *cacheImpl) GetLfsSyncFileProgress(ctx context.Context, repoID int64, oid, partSize string) (int, error) {
	key := lfsProgressCacheKey(repoID, partSize)
	strProgress, err := c.redis.HGet(ctx, key, oid)
	if err != nil {
		return 0, fmt.Errorf("failed to get lfs progress: %w", err)
	}
	return strconv.Atoi(strProgress)
}

// DeleteSpecificLfsPartCache removes one uploaded part marker for one LFS object.
func (c *cacheImpl) DeleteSpecificLfsPartCache(ctx context.Context, repoID int64, oid, partSize string, partNumber int) error {
	key := lfsPartCacheKey(repoID, partSize, oid)
	return c.redis.HDel(ctx, key, strconv.Itoa(partNumber))
}

// lfsProgressCacheKey returns the Redis hash key that stores LFS object progress by oid.
func lfsProgressCacheKey(repoID int64, partSize string) string {
	return fmt.Sprintf("%s:repo:%d:partsize:%s:progress", LfsSyncKeyPrefix, repoID, partSize)
}

// lfsPartCacheKey returns the Redis hash key that stores uploaded part markers for one LFS object.
func lfsPartCacheKey(repoID int64, partSize, oid string) string {
	return fmt.Sprintf("%s:repo:%d:partsize:%s:parts:%s", LfsSyncKeyPrefix, repoID, partSize, oid)
}

// lfsUploadIDCacheKey returns the Redis hash key that stores multipart upload IDs by oid.
func lfsUploadIDCacheKey(repoID int64, partSize string) string {
	return fmt.Sprintf("%s:repo:%d:partsize:%s:uploads", LfsSyncKeyPrefix, repoID, partSize)
}

// CacheRunningTask stores the mirror task currently handled by one legacy worker.
func (c *cacheImpl) CacheRunningTask(ctx context.Context, workID int, mirrorID int64) error {
	return c.redis.HSet(ctx, LfsRunningTaskKey, strconv.Itoa(workID), mirrorID)
}

// RemoveRunningTask removes the running task marker for one legacy worker.
func (c *cacheImpl) RemoveRunningTask(ctx context.Context, workID int) error {
	return c.redis.HDel(ctx, LfsRunningTaskKey, strconv.Itoa(workID))
}

// GetRunningTask returns all legacy worker-to-mirror running task mappings.
func (c *cacheImpl) GetRunningTask(ctx context.Context) (map[int]int64, error) {
	mapping, err := c.redis.HGetAll(ctx, LfsRunningTaskKey)
	if err != nil {
		return nil, err
	}
	result := make(map[int]int64)
	for k, v := range mapping {
		workID, err := strconv.Atoi(k)
		if err != nil {
			return nil, err
		}
		mirrorID, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return nil, err
		}
		result[workID] = mirrorID
	}
	return result, nil
}
