package cache

type MemberCache struct {
	cache *Cache
}

func NewMemberCache(cache *Cache) *MemberCache {
	return &MemberCache{
		cache: cache,
	}
}
