//go:build saas

package component

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	mockcache "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/store/cache"
)

func TestRedisOAuthStateStore_Save(t *testing.T) {
	ctx := context.Background()
	mockRedis := mockcache.NewMockRedisClient(t)
	store := newRedisOAuthStateStore(mockRedis)

	mockRedis.EXPECT().
		SetEx(ctx, oauthStateKeyPrefix+"state-1", `{"code_verifier":"verifier-1","user_uuid":"user-1","site_id":"site-1","callback":"https://biz.example.com/return"}`, 10*time.Minute).
		Return(nil).
		Once()

	err := store.Save(ctx, "state-1", &authSession{
		CodeVerifier: "verifier-1",
		UserUUID:     "user-1",
		SiteID:       "site-1",
		Callback:     "https://biz.example.com/return",
	}, 10*time.Minute)
	require.NoError(t, err)
}

func TestRedisOAuthStateStore_LoadAndDelete(t *testing.T) {
	t.Run("returns session when redis returns payload", func(t *testing.T) {
		ctx := context.Background()
		mockRedis := mockcache.NewMockRedisClient(t)
		store := newRedisOAuthStateStore(mockRedis)

		mockRedis.EXPECT().
			GetDel(ctx, oauthStateKeyPrefix+"state-1").
			Return(`{"code_verifier":"verifier-1","user_uuid":"user-1","site_id":"site-1","callback":"https://biz.example.com/return"}`, nil).
			Once()

		session, ok, err := store.LoadAndDelete(ctx, "state-1")
		require.NoError(t, err)
		require.True(t, ok)
		require.NotNil(t, session)
		assert.Equal(t, "verifier-1", session.CodeVerifier)
		assert.Equal(t, "user-1", session.UserUUID)
		assert.Equal(t, "site-1", session.SiteID)
		assert.Equal(t, "https://biz.example.com/return", session.Callback)
	})

	t.Run("returns missing when redis returns nil", func(t *testing.T) {
		ctx := context.Background()
		mockRedis := mockcache.NewMockRedisClient(t)
		store := newRedisOAuthStateStore(mockRedis)

		mockRedis.EXPECT().
			GetDel(ctx, oauthStateKeyPrefix+"missing").
			Return("", redis.Nil).
			Once()

		session, ok, err := store.LoadAndDelete(ctx, "missing")
		require.NoError(t, err)
		assert.False(t, ok)
		assert.Nil(t, session)
	})

	t.Run("returns error when redis getdel fails", func(t *testing.T) {
		ctx := context.Background()
		mockRedis := mockcache.NewMockRedisClient(t)
		store := newRedisOAuthStateStore(mockRedis)

		mockRedis.EXPECT().
			GetDel(ctx, oauthStateKeyPrefix+"state-1").
			Return("", errors.New("redis down")).
			Once()

		session, ok, err := store.LoadAndDelete(ctx, "state-1")
		require.Error(t, err)
		assert.False(t, ok)
		assert.Nil(t, session)
		assert.Contains(t, err.Error(), "pop oauth state session")
	})
}

func TestRedisOAuthStateStore_Load(t *testing.T) {
	t.Run("returns session when key exists", func(t *testing.T) {
		ctx := context.Background()
		mockRedis := mockcache.NewMockRedisClient(t)
		store := newRedisOAuthStateStore(mockRedis)

		mockRedis.EXPECT().
			Get(ctx, oauthStateKeyPrefix+"state-1").
			Return(`{"code_verifier":"verifier-1","user_uuid":"user-1","site_id":"site-1","callback":"https://biz.example.com/return"}`, nil).
			Once()

		session, ok, err := store.Load(ctx, "state-1")
		require.NoError(t, err)
		require.True(t, ok)
		require.NotNil(t, session)
		assert.Equal(t, "verifier-1", session.CodeVerifier)
		assert.Equal(t, "user-1", session.UserUUID)
		assert.Equal(t, "site-1", session.SiteID)
		assert.Equal(t, "https://biz.example.com/return", session.Callback)
	})

	t.Run("returns missing when key absent", func(t *testing.T) {
		ctx := context.Background()
		mockRedis := mockcache.NewMockRedisClient(t)
		store := newRedisOAuthStateStore(mockRedis)

		mockRedis.EXPECT().
			Get(ctx, oauthStateKeyPrefix+"missing").
			Return("", redis.Nil).
			Once()

		session, ok, err := store.Load(ctx, "missing")
		require.NoError(t, err)
		assert.False(t, ok)
		assert.Nil(t, session)
	})
}
