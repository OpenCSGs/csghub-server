package cache

type SSHKeyCache struct {
	cache *Cache
}

func NewSSHKeyCache(cache *Cache) *SSHKeyCache {
	return &SSHKeyCache{
		cache: cache,
	}
}
