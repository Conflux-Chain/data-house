package model

import (
	"container/list"
	"sync"
	"time"
)

// LRUCache implements a thread-safe LRU cache with generic support
type LRUCache[K comparable, V any] struct {
	mu       sync.RWMutex
	capacity int                 // Maximum number of items
	size     int                 // Current number of items
	cache    map[K]*list.Element // Map for O(1) lookups
	list     *list.List          // Doubly linked list for LRU ordering
	ttl      time.Duration       // Time-to-live for cache entries (0 = no expiry)
	stats    Stats               // Cache statistics
}

// cacheEntry represents an item in the cache
type cacheEntry[K comparable, V any] struct {
	key       K
	value     V
	expiresAt time.Time // When this entry expires
}

// Stats holds cache statistics
type Stats struct {
	Hits        int64 // Number of cache hits
	Misses      int64 // Number of cache misses
	Evictions   int64 // Number of items evicted
	Inserts     int64 // Number of items inserted
	Expirations int64 // Number of items expired
}

// NewLRUCache creates a new LRU cache with the specified capacity and TTL
// If ttl is 0, entries never expire
func NewLRUCache[K comparable, V any](capacity int, ttl time.Duration) *LRUCache[K, V] {
	if capacity <= 0 {
		capacity = 100 // Default capacity
	}

	return &LRUCache[K, V]{
		capacity: capacity,
		cache:    make(map[K]*list.Element),
		list:     list.New(),
		ttl:      ttl,
		stats:    Stats{},
	}
}

// Get retrieves a value from the cache by key
// Returns the value and true if found, otherwise zero value and false
func (c *LRUCache[K, V]) Get(key K) (V, bool) {
	c.mu.RLock()
	element, exists := c.cache[key]
	c.mu.RUnlock()

	if !exists {
		c.mu.Lock()
		c.stats.Misses++
		c.mu.Unlock()
		var zero V
		return zero, false
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	entry := element.Value.(*cacheEntry[K, V])

	// Check if entry has expired
	if c.ttl > 0 && time.Now().After(entry.expiresAt) {
		c.removeElement(element)
		c.stats.Misses++
		c.stats.Expirations++
		var zero V
		return zero, false
	}

	// Move to front (most recently used)
	c.list.MoveToFront(element)
	c.stats.Hits++

	return entry.value, true
}

// Put adds or updates a value in the cache
func (c *LRUCache[K, V]) Put(key K, value V) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Check if key already exists
	if element, exists := c.cache[key]; exists {
		// Update existing entry
		entry := element.Value.(*cacheEntry[K, V])
		entry.value = value
		if c.ttl > 0 {
			entry.expiresAt = time.Now().Add(c.ttl)
		}
		c.list.MoveToFront(element)
		return
	}

	// Create new entry
	entry := &cacheEntry[K, V]{
		key:   key,
		value: value,
	}
	if c.ttl > 0 {
		entry.expiresAt = time.Now().Add(c.ttl)
	}

	// Add to cache
	element := c.list.PushFront(entry)
	c.cache[key] = element
	c.size++
	c.stats.Inserts++

	// Evict if over capacity
	if c.size > c.capacity {
		c.evictOldest()
	}
}

// Remove deletes a key from the cache
func (c *LRUCache[K, V]) Remove(key K) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	element, exists := c.cache[key]
	if !exists {
		return false
	}

	c.removeElement(element)
	return true
}

// removeElement removes an element from the cache
func (c *LRUCache[K, V]) removeElement(element *list.Element) {
	entry := element.Value.(*cacheEntry[K, V])
	c.list.Remove(element)
	delete(c.cache, entry.key)
	c.size--
}

// evictOldest removes the least recently used item
func (c *LRUCache[K, V]) evictOldest() {
	element := c.list.Back()
	if element != nil {
		c.removeElement(element)
		c.stats.Evictions++
	}
}

// Clear removes all items from the cache
func (c *LRUCache[K, V]) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.cache = make(map[K]*list.Element)
	c.list.Init()
	c.size = 0
}

// Len returns the current number of items in the cache
func (c *LRUCache[K, V]) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.size
}

// Capacity returns the cache capacity
func (c *LRUCache[K, V]) Capacity() int {
	return c.capacity
}

// Stats returns current cache statistics
func (c *LRUCache[K, V]) Stats() Stats {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.stats
}

// ResetStats resets all statistics counters
func (c *LRUCache[K, V]) ResetStats() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.stats = Stats{}
}

// Keys returns all keys in the cache (from most to least recently used)
func (c *LRUCache[K, V]) Keys() []K {
	c.mu.RLock()
	defer c.mu.RUnlock()

	keys := make([]K, 0, c.size)
	for element := c.list.Front(); element != nil; element = element.Next() {
		entry := element.Value.(*cacheEntry[K, V])
		keys = append(keys, entry.key)
	}
	return keys
}

// CleanupExpired removes all expired entries from the cache
func (c *LRUCache[K, V]) CleanupExpired() int {
	if c.ttl <= 0 {
		return 0
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	expiredCount := 0
	now := time.Now()

	// Collect expired elements
	var expiredElements []*list.Element
	for element := c.list.Back(); element != nil; element = element.Prev() {
		entry := element.Value.(*cacheEntry[K, V])
		if now.After(entry.expiresAt) {
			expiredElements = append(expiredElements, element)
		}
	}

	// Remove expired elements
	for _, element := range expiredElements {
		c.removeElement(element)
		expiredCount++
	}

	c.stats.Expirations += int64(expiredCount)
	return expiredCount
}
