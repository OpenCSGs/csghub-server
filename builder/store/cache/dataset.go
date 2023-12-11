package cache

type DatasetCache struct {
	cache *Cache
}

func NewDatasetCache(cache *Cache) *DatasetCache {
	return &DatasetCache{
		cache: cache,
	}
}
