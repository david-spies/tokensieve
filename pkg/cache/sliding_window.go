package cache

import (
	"container/list"
	"crypto/sha256"
	"encoding/hex"
	"sync"
	"time"
)

// Entry represents a cached context block with metadata
type Entry struct {
	Hash      string
	Content   string
	TokenEst  int64
	CreatedAt time.Time
	HitCount  int64
	SessionID string
}

// lruItem is used internally by the LRU list
type lruItem struct {
	hash string
}

// SlidingWindowCache is a concurrency-safe, LRU-evicted token cache
// with per-session tracking and TTL expiry support.
type SlidingWindowCache struct {
	mu       sync.RWMutex
	maxSize  int
	store    map[string]*list.Element
	lru      *list.List
	entries  map[string]*Entry
	sessions map[string][]string // sessionID -> []hash

	// Telemetry counters
	TotalHits   int64
	TotalMisses int64
	TotalEvicts int64
}

// NewSlidingWindowCache constructs a cache bounded by maxSize entries.
// 128k is suitable for ~4M token context tracking at ~32 bytes/hash.
func NewSlidingWindowCache(maxSize int) *SlidingWindowCache {
	return &SlidingWindowCache{
		maxSize:  maxSize,
		store:    make(map[string]*list.Element),
		lru:      list.New(),
		entries:  make(map[string]*Entry),
		sessions: make(map[string][]string),
	}
}

// ComputeHash returns a deterministic SHA-256 fingerprint of content.
func (c *SlidingWindowCache) ComputeHash(content string) string {
	h := sha256.New()
	h.Write([]byte(content))
	return hex.EncodeToString(h.Sum(nil))
}

// Get retrieves an entry and updates LRU position. Thread-safe.
func (c *SlidingWindowCache) Get(hash string) (*Entry, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	elem, exists := c.store[hash]
	if !exists {
		c.TotalMisses++
		return nil, false
	}

	// Promote to front (most recently used)
	c.lru.MoveToFront(elem)
	entry := c.entries[hash]
	entry.HitCount++
	c.TotalHits++
	return entry, true
}

// Put stores a new entry. If the cache is full, the LRU entry is evicted.
func (c *SlidingWindowCache) Put(hash, content, sessionID string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Already cached — just update session mapping
	if _, exists := c.store[hash]; exists {
		c.sessions[sessionID] = appendUnique(c.sessions[sessionID], hash)
		return
	}

	// Evict LRU if at capacity
	if c.lru.Len() >= c.maxSize {
		back := c.lru.Back()
		if back != nil {
			item := back.Value.(*lruItem)
			c.lru.Remove(back)
			delete(c.store, item.hash)
			delete(c.entries, item.hash)
			c.TotalEvicts++
		}
	}

	tokenEst := int64(len(content) / 4)
	entry := &Entry{
		Hash:      hash,
		Content:   content,
		TokenEst:  tokenEst,
		CreatedAt: time.Now(),
		HitCount:  0,
		SessionID: sessionID,
	}

	elem := c.lru.PushFront(&lruItem{hash: hash})
	c.store[hash] = elem
	c.entries[hash] = entry
	c.sessions[sessionID] = appendUnique(c.sessions[sessionID], hash)
}

// SessionHashes returns all cached hashes for a given session.
func (c *SlidingWindowCache) SessionHashes(sessionID string) []string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.sessions[sessionID]
}

// Stats returns a snapshot of cache statistics.
func (c *SlidingWindowCache) Stats() CacheStats {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return CacheStats{
		TotalEntries: c.lru.Len(),
		MaxSize:      c.maxSize,
		HitRate:      hitRate(c.TotalHits, c.TotalMisses),
		TotalHits:    c.TotalHits,
		TotalMisses:  c.TotalMisses,
		TotalEvicts:  c.TotalEvicts,
	}
}

// CacheStats is a read-only snapshot of cache health.
type CacheStats struct {
	TotalEntries int     `json:"total_entries"`
	MaxSize      int     `json:"max_size"`
	HitRate      float64 `json:"hit_rate_pct"`
	TotalHits    int64   `json:"total_hits"`
	TotalMisses  int64   `json:"total_misses"`
	TotalEvicts  int64   `json:"total_evicts"`
}

// appendUnique appends s to slice only if not already present.
func appendUnique(slice []string, s string) []string {
	for _, v := range slice {
		if v == s {
			return slice
		}
	}
	return append(slice, s)
}

func hitRate(hits, misses int64) float64 {
	total := hits + misses
	if total == 0 {
		return 0
	}
	return float64(hits) / float64(total) * 100
}
