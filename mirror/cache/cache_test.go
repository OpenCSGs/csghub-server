package cache

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	mockcache "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/store/cache"
	storecache "opencsg.com/csghub-server/builder/store/cache"
)

func Test_lfsProgressCacheKey(t *testing.T) {
	type args struct {
		repoID   int64
		partSize string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "test",
			args: args{
				repoID:   1234,
				partSize: "partSize",
			},
			want: "lfssyncer:repo:1234:partsize:partSize:progress",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := lfsProgressCacheKey(tt.args.repoID, tt.args.partSize); got != tt.want {
				t.Errorf("lfsProgressCacheKey() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_lfsPartCacheKey(t *testing.T) {
	type args struct {
		repoID   int64
		partSize string
		oid      string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "test",
			args: args{
				repoID:   1234,
				partSize: "partSize",
				oid:      "oid",
			},
			want: "lfssyncer:repo:1234:partsize:partSize:parts:oid",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := lfsPartCacheKey(tt.args.repoID, tt.args.partSize, tt.args.oid); got != tt.want {
				t.Errorf("lfsPartCacheKey() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_lfsUploadIDCacheKey(t *testing.T) {
	type args struct {
		repoID   int64
		partSize string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "test",
			args: args{
				repoID:   1234,
				partSize: "partSize",
			},
			want: "lfssyncer:repo:1234:partsize:partSize:uploads",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := lfsUploadIDCacheKey(tt.args.repoID, tt.args.partSize); got != tt.want {
				t.Errorf("lfsUploadIDCacheKey() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDeleteRepoSyncCacheAllPartSizes(t *testing.T) {
	ctx := context.Background()
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	c := &cacheImpl{redis: storecache.NewCacheWithClient(ctx, client)}
	for _, repoID := range []int64{1234, 12345} {
		for _, partSize := range []string{"100", "104857600", "107374183"} {
			require.NoError(t, c.CacheUploadID(ctx, repoID, "oid", partSize, "upload"))
			require.NoError(t, c.CacheLfsSyncFileProgress(ctx, repoID, "oid", partSize, 65))
			require.NoError(t, c.CacheLfsSyncAddPart(ctx, repoID, "oid", partSize, 1))
			for i := 0; i < 205; i++ {
				require.NoError(t, c.CacheLfsSyncAddPart(ctx, repoID, fmt.Sprint("orphan-", i), partSize, 1))
			}
		}
	}
	require.NoError(t, c.DeleteRepoSyncCache(ctx, 1234))
	for _, key := range server.Keys() {
		require.NotContains(t, key, "repo:1234:")
	}
	require.Len(t, server.Keys(), 3*208)
	require.NoError(t, c.DeleteRepoSyncCache(ctx, 1234))
}

func TestDeleteRepoSyncCachePaginatesAndBatches(t *testing.T) {
	ctx := context.Background()
	client := mockcache.NewMockRedisClient(t)
	c := &cacheImpl{redis: client}
	keys := make([]string, 205)
	for i := range keys {
		keys[i] = fmt.Sprintf("key-%d", i)
	}
	args := make([]any, len(keys))
	for i, key := range keys {
		args[i] = key
	}
	client.EXPECT().Scan(ctx, uint64(0), "lfssyncer:repo:1234:partsize:*", int64(100)).Return(keys, uint64(42), nil).Once()
	client.EXPECT().Del(ctx, args[:100]...).Return(nil).Once()
	client.EXPECT().Del(ctx, args[100:200]...).Return(nil).Once()
	client.EXPECT().Del(ctx, args[200:]...).Return(nil).Once()
	client.EXPECT().Scan(ctx, uint64(42), "lfssyncer:repo:1234:partsize:*", int64(100)).Return([]string{}, uint64(7), nil).Once()
	client.EXPECT().Scan(ctx, uint64(7), "lfssyncer:repo:1234:partsize:*", int64(100)).Return([]string{"last"}, uint64(0), nil).Once()
	client.EXPECT().Del(ctx, "last").Return(nil).Once()
	require.NoError(t, c.DeleteRepoSyncCache(ctx, 1234))
}

func TestDeleteRepoSyncCacheErrors(t *testing.T) {
	for _, phase := range []string{"scan", "delete"} {
		t.Run(phase, func(t *testing.T) {
			ctx := context.Background()
			client := mockcache.NewMockRedisClient(t)
			c := &cacheImpl{redis: client}
			failure := errors.New("redis unavailable")
			if phase == "scan" {
				client.EXPECT().Scan(ctx, uint64(0), mock.Anything, int64(100)).Return(nil, uint64(0), failure).Once()
			} else {
				client.EXPECT().Scan(ctx, uint64(0), mock.Anything, int64(100)).Return([]string{"key"}, uint64(42), nil).Once()
				client.EXPECT().Del(ctx, "key").Return(failure).Once()
			}
			require.ErrorIs(t, c.DeleteRepoSyncCache(ctx, 1234), failure)
		})
	}
}
