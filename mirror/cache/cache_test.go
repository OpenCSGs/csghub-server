package cache

import (
	"context"
	"fmt"
	"testing"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	mockcache "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/store/cache"
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

func TestDeleteRepoSyncCache(t *testing.T) {
	ctx := context.Background()
	redisClient := mockcache.NewMockRedisClient(t)
	c := &cacheImpl{redis: redisClient}
	uploads := make(map[string]string, 205)
	for i := 0; i < 205; i++ {
		oid := fmt.Sprintf("oid-%03d", i)
		uploads[oid] = "upload-id"
	}

	redisClient.EXPECT().
		HGetAll(ctx, "lfssyncer:repo:1234:partsize:64:uploads").
		Return(uploads, nil)
	redisClient.EXPECT().
		Pipelined(ctx, mock.Anything).
		RunAndReturn(func(ctx context.Context, fn func(redis.Pipeliner) error) ([]redis.Cmder, error) {
			rdb := redis.NewClient(&redis.Options{Addr: "127.0.0.1:0"})
			defer rdb.Close()
			pipe := rdb.Pipeline()

			err := fn(pipe)
			require.NoError(t, err)
			require.Equal(t, 3, pipe.Len())
			return nil, nil
		})

	err := c.DeleteRepoSyncCache(ctx, 1234, "64")
	require.NoError(t, err)
}
