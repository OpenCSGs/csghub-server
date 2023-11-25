package cache

type UserCache struct {
	cache *Cache
}

func NewUserCache(cache *Cache) *UserCache {
	return &UserCache{
		cache: cache,
	}
}
