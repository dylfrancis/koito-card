package koito

import (
	"container/list"
	"sync"
)

type lruCache struct {
	mu       sync.Mutex
	capacity int
	recency  *list.List
	elements map[string]*list.Element
}

type lruEntry struct{ key, value string }

func newLRUCache(capacity int) *lruCache {
	return &lruCache{capacity: capacity, recency: list.New(), elements: map[string]*list.Element{}}
}

func (cache *lruCache) get(key string) (string, bool) {
	cache.mu.Lock()
	defer cache.mu.Unlock()
	element, found := cache.elements[key]
	if !found {
		return "", false
	}
	cache.recency.MoveToFront(element)
	return element.Value.(*lruEntry).value, true
}

func (cache *lruCache) add(key, value string) {
	cache.mu.Lock()
	defer cache.mu.Unlock()
	if element, found := cache.elements[key]; found {
		element.Value.(*lruEntry).value = value
		cache.recency.MoveToFront(element)
		return
	}
	cache.elements[key] = cache.recency.PushFront(&lruEntry{key, value})
	if cache.recency.Len() > cache.capacity {
		leastRecent := cache.recency.Back()
		cache.recency.Remove(leastRecent)
		delete(cache.elements, leastRecent.Value.(*lruEntry).key)
	}
}
