package cache

type ModelCache struct {
	cache *Cache
}

func NewModelCache(cache *Cache) *ModelCache {
	return &ModelCache{
		cache: cache,
	}
}
