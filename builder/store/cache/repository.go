package cache

type RepoCache struct {
	cache *Cache
}

func NewRepoCache(cache *Cache) *RepoCache {
	return &RepoCache{
		cache: cache,
	}
}
