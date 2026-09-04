package engine

import (
	"sync"
	"time"
)

type PDFTextCacheItem struct {
	Text     string    `json:"text"`
	ExpireAt time.Time `json:"expire_at"`
}

type PDFTextCache struct {
	mu    sync.RWMutex
	items map[string]PDFTextCacheItem
	ttl   time.Duration
}

var PDFText = NewPDFTextCache(time.Hour * 2)

func NewPDFTextCache(ttl time.Duration) *PDFTextCache {
	return &PDFTextCache{
		items: make(map[string]PDFTextCacheItem),
		ttl:   ttl,
	}
}

// Get returns cached PDF text when it exists and has not expired.
func (c *PDFTextCache) Get(fileID string) (string, bool) {
	c.mu.RLock()
	item, ok := c.items[fileID]
	c.mu.RUnlock()

	if !ok {
		return "", false
	}
	if !item.ExpireAt.IsZero() && time.Now().After(item.ExpireAt) {
		c.Delete(fileID)
		return "", false
	}

	return item.Text, true
}

// Set stores PDF text in the in-memory cache.
func (c *PDFTextCache) Set(fileID, text string) {
	c.mu.Lock()
	c.items[fileID] = PDFTextCacheItem{
		Text:     text,
		ExpireAt: time.Now().Add(c.ttl),
	}
	defer c.mu.Unlock()
}

// Delete one expired file
func (c *PDFTextCache) Delete(fileID string) bool {
	c.mu.Lock()
	_, ok := c.items[fileID]
	delete(c.items, fileID)
	c.mu.Unlock()
	return ok
}

// Clear expired file
func (c *PDFTextCache) Clear() {
	for fileID := range c.items {
		if c.items[fileID].ExpireAt.IsZero() || time.Now().After(c.items[fileID].ExpireAt) {
			c.Delete(fileID)
		}
	}
}
