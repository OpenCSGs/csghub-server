package cache

type OrgCache struct {
	cache *Cache
}

func NewOrgCache(cache *Cache) *OrgCache {
	return &OrgCache{
		cache: cache,
	}
}
