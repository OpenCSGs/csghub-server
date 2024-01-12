package cache

type NamespaceCache struct {
	cache *Cache
}

func NewNamespaceCache(cache *Cache) *NamespaceCache {
	return &NamespaceCache{
		cache: cache,
	}
}
