package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/aliyun/aliyun-oss-go-sdk/oss"
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
	CacheUploadID(ctx context.Context, repoPath, oid, partSize string, uploadID string) error
	GetUploadID(ctx context.Context, repoPath, oid, partSize string) (string, error)
	DeleteUploadID(ctx context.Context, repoPath, oid, partSize string) error

	CacheLfsSyncAddPart(ctx context.Context, repoPath, oid, partSize string, partNumber int) error
	IsLfsPartSynced(ctx context.Context, repoPath, oid, partSize string, partNumber int) (bool, error)
	LfsPartSyncedCount(ctx context.Context, repoPath, oid, partSize string) (int64, error)
	DeleteLfsPartCache(ctx context.Context, repoPath, oid, partSize string) error
	DeleteSpecificLfsPartCache(ctx context.Context, repoPath, oid, partSize string, partNumber int) error

	CacheLfsSyncFileProgress(ctx context.Context, repoPath, oid, partSize string, progress int) error
	DeleteLfsSyncFileProgress(ctx context.Context, repoPath, oid, partSize string) error
	GetLfsSyncFileProgress(ctx context.Context, repoPath, oid, partSize string) (int, error)

	CacheRunningTask(ctx context.Context, workID int, mirrorID int64) error
	GetRunningTask(ctx context.Context) (map[int]int64, error)
	RemoveRunningTask(ctx context.Context, workID int) error
	DeleteAllCache(ctx context.Context, repoPath, oid, partSize string) error
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

type OssMultipartUploadResult struct {
	Imur oss.InitiateMultipartUploadResult
}

func (o *OssMultipartUploadResult) MarshalBinary() ([]byte, error) {
	return json.Marshal(o)
}

func (o *OssMultipartUploadResult) UnmarshalBinary(data []byte) error {
	return json.Unmarshal(data, o)
}

func (c *cacheImpl) DeleteAllCache(ctx context.Context, repoPath, oid, partSize string) error {
	uploadIDkey := uploadIDCacheKey(repoPath, oid, partSize)
	lfsPartCacheKey := lfsPartCacheKey(repoPath, oid, partSize)
	lfsProgressCacheKey := lfsProgressCacheKey(repoPath, oid, partSize)
	return c.redis.Del(ctx, lfsProgressCacheKey, uploadIDkey, lfsPartCacheKey)
}

func (c *cacheImpl) CacheUploadID(ctx context.Context, repoPath, oid, partSize string, uploadID string) error {
	key := uploadIDCacheKey(repoPath, oid, partSize)
	return c.redis.Set(ctx, key, uploadID)
}

func (c *cacheImpl) GetUploadID(ctx context.Context, repoPath, oid, partSize string) (string, error) {
	key := uploadIDCacheKey(repoPath, oid, partSize)
	return c.redis.Get(ctx, key)
}

func (c *cacheImpl) DeleteUploadID(ctx context.Context, repoPath, oid, partSize string) error {
	key := uploadIDCacheKey(repoPath, oid, partSize)
	return c.redis.Del(ctx, key)
}

func (c *cacheImpl) CacheLfsSyncAddPart(ctx context.Context, repoPath, oid, partSize string, partNumber int) error {
	key := lfsPartCacheKey(repoPath, oid, partSize)
	err := c.redis.SAdd(ctx, key, partNumber)
	if err != nil {
		return fmt.Errorf("failed to add lfs part number to set:  %w", err)
	}
	return nil
}

func (c *cacheImpl) IsLfsPartSynced(ctx context.Context, repoPath, oid, partSize string, partNumber int) (bool, error) {
	key := lfsPartCacheKey(repoPath, oid, partSize)
	return c.redis.SIsMember(ctx, key, partNumber)
}

func (c *cacheImpl) LfsPartSyncedCount(ctx context.Context, repoPath, oid, partSize string) (int64, error) {
	key := lfsPartCacheKey(repoPath, oid, partSize)
	return c.redis.SCard(ctx, key)
}

func (c *cacheImpl) DeleteLfsPartCache(ctx context.Context, repoPath, oid, partSize string) error {
	key := lfsPartCacheKey(repoPath, oid, partSize)
	return c.redis.Del(ctx, key)
}

func (c *cacheImpl) CacheLfsSyncFileProgress(ctx context.Context, repoPath, oid, partSize string, progress int) error {
	key := lfsProgressCacheKey(repoPath, oid, partSize)
	strProgress := strconv.Itoa(progress)
	err := c.redis.Set(ctx, key, strProgress)
	if err != nil {
		return fmt.Errorf("failed to set lfs part number to set:   %w", err)
	}
	return nil
}

func (c *cacheImpl) DeleteLfsSyncFileProgress(ctx context.Context, repoPath, oid, partSize string) error {
	key := lfsProgressCacheKey(repoPath, oid, partSize)
	return c.redis.Del(ctx, key)
}

func (c *cacheImpl) GetLfsSyncFileProgress(ctx context.Context, repoPath, oid, partSize string) (int, error) {
	key := lfsProgressCacheKey(repoPath, oid, partSize)
	strProgress, err := c.redis.Get(ctx, key)
	if err != nil {
		return 0, fmt.Errorf("failed to get lfs part number:   %w", err)
	}
	return strconv.Atoi(strProgress)
}

func (c *cacheImpl) DeleteSpecificLfsPartCache(ctx context.Context, repoPath, oid, partSize string, partNumber int) error {
	key := lfsPartCacheKey(repoPath, oid, partSize)
	return c.redis.SRem(ctx, key, partNumber)
}
func lfsProgressCacheKey(repoPath, oid, partSize string) string {
	return fmt.Sprintf("%s-%s-%s-%s", LfsSyncProgressKeyPrefix, repoPath, oid, partSize)
}
func lfsPartCacheKey(repoPath, oid, partSize string) string {
	return fmt.Sprintf("%s-%s-%s-%s", LfsSyncKeyPrefix, repoPath, oid, partSize)
}

func uploadIDCacheKey(repoPath, oid, partSize string) string {
	return fmt.Sprintf("%s-%s-%s-%s", UploadIDKeyPrefix, repoPath, oid, partSize)
}

func (c *cacheImpl) CacheRunningTask(ctx context.Context, workID int, mirrorID int64) error {
	return c.redis.HSet(ctx, LfsRunningTaskKey, strconv.Itoa(workID), mirrorID)
}

func (c *cacheImpl) RemoveRunningTask(ctx context.Context, workID int) error {
	return c.redis.HDel(ctx, LfsRunningTaskKey, strconv.Itoa(workID))
}

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
