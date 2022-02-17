package cache

import (
	"time"

	"k8s.io/client-go/tools/cache"
)

// TencentCloudCacheEntry is the internal structure stores inside TTLStore.
type TencentCloudCacheEntry struct {
	Key  string
	Data interface{}
}

// TTLCache is a cache with TTL.
type TTLCache struct {
	Store cache.Store
	TTL   time.Duration
}

// NewTTLCache creates a new TTLCache.
func NewTTLCache(ttl time.Duration) *TTLCache {
	return &TTLCache{
		Store: cache.NewTTLStore(cacheKeyFunc, ttl),
		TTL:   ttl,
	}
}

// TencentCloudCacheEntry defines the key function required in TTLStore.
func cacheKeyFunc(obj interface{}) (string, error) {
	return obj.(*TencentCloudCacheEntry).Key, nil
}

// Set sets the data cache for the key.
func (t *TTLCache) Set(key string, data interface{}) {
	_ = t.Store.Add(&TencentCloudCacheEntry{
		Key:  key,
		Data: data,
	})
}

// Get get the cache data for the key.
func (t *TTLCache) Get(key string) (interface{}, bool) {
	item, exists, err := t.Store.GetByKey(key)
	if err != nil || exists == false {
		return nil, false
	}
	return item.(*TencentCloudCacheEntry).Data, true
}

// Delete delete cache data for the key.
func (t *TTLCache) Delete(key string) bool {
	err := t.Store.Delete(key)
	if err != nil {
		return false
	}
	return true
}
