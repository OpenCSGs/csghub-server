package cache

import (
	"context"

	"git-devops.opencsg.com/product/community/starhub-server/config"
	"github.com/google/wire"
)

// WireSet provides a wire set for this package.
var WireSet = wire.NewSet(
	ProvideRedisConfig,
	ProvideCache,
	ProvideModelCache,
	ProvideDatasetCache,
	ProvideUserCache,
	ProvideAccessTokenCache,
	ProvideSSHKeyCache,
	ProvideMemberCache,
	ProvideOrgCache,
	ProvideNamespaceCache,
	ProvideTagCache,
	ProvideRepoCache,
)

func ProvideRedisConfig(config *config.Config) RedisConfig {
	return RedisConfig{
		Addr:     config.Redis.Endpoint,
		Username: config.Redis.User,
		Password: config.Redis.Password,
	}

}

func ProvideCache(ctx context.Context, cfg RedisConfig) (*Cache, error) {
	return NewCache(ctx, cfg)
}

func ProvideModelCache(cache *Cache) *ModelCache {
	return NewModelCache(cache)
}

func ProvideDatasetCache(cache *Cache) *DatasetCache {
	return NewDatasetCache(cache)
}

func ProvideUserCache(cache *Cache) *UserCache {
	return NewUserCache(cache)
}

func ProvideAccessTokenCache(cache *Cache) *AccessTokenCache {
	return NewAccessTokenCache(cache)
}

func ProvideSSHKeyCache(cache *Cache) *SSHKeyCache {
	return NewSSHKeyCache(cache)
}

func ProvideMemberCache(cache *Cache) *MemberCache {
	return NewMemberCache(cache)
}

func ProvideOrgCache(cache *Cache) *OrgCache {
	return NewOrgCache(cache)
}

func ProvideNamespaceCache(cache *Cache) *NamespaceCache {
	return NewNamespaceCache(cache)
}

func ProvideTagCache(cache *Cache) *TagCache {
	return NewTagCache(cache)
}

func ProvideRepoCache(cache *Cache) *RepoCache {
	return NewRepoCache(cache)
}
