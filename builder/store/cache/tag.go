package cache

type TagCache struct {
	cache *Cache
}

func NewTagCache(cache *Cache) *TagCache {
	return &TagCache{
		cache: cache,
	}
}
