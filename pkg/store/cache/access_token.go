package cache

type AccessTokenCache struct {
	cache *Cache
}

func NewAccessTokenCache(cache *Cache) *AccessTokenCache {
	return &AccessTokenCache{
		cache: cache,
	}
}
