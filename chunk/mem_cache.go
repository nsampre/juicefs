package chunk

import (
	"errors"
	"sync"
	"time"
)

type memItem struct {
	atime time.Time
	page  *Page
}

type memcache struct {
	sync.Mutex
	capacity int64
	used     int64
	pages    map[string]memItem
}

func newMemStore(config *Config) *memcache {
	c := &memcache{
		capacity: config.CacheSize << 20,
		pages:    make(map[string]memItem),
	}
	return c
}

func (c *memcache) stats() (int64, int64) {
	c.Lock()
	defer c.Unlock()
	return int64(len(c.pages)), c.used
}

func (c *memcache) cache(key string, p *Page) {
	if c.capacity == 0 {
		return
	}
	c.Lock()
	defer c.Unlock()
	if _, ok := c.pages[key]; ok {
		return
	}
	p.Acquire()
	size := int64(cap(p.Data))
	c.pages[key] = memItem{time.Now(), p}
	c.used += size + 4096
	if c.used > c.capacity {
		c.cleanup()
	}
}

func (c *memcache) delete(key string, p *Page) {
	size := int64(cap(p.Data))
	c.used -= size + 4096
	p.Release()
	delete(c.pages, key)
}

func (c *memcache) remove(key string) {
	c.Lock()
	defer c.Unlock()
	if item, ok := c.pages[key]; ok {
		c.delete(key, item.page)
		logger.Debugf("remove %s from cache", key)
	}
}

func (c *memcache) load(key string) (ReadCloser, error) {
	c.Lock()
	defer c.Unlock()
	if item, ok := c.pages[key]; ok {
		c.pages[key] = memItem{time.Now(), item.page}
		return NewPageReader(item.page), nil
	}
	return nil, errors.New("not found")
}

// locked
func (c *memcache) cleanup() {
	var cnt int
	var lastKey string
	var lastValue memItem
	var now = time.Now()
	// for each two random keys, then compare the access time, evict the older one
	for k, v := range c.pages {
		if cnt == 0 || lastValue.atime.After(v.atime) {
			lastKey = k
			lastValue = v
		}
		cnt++
		if cnt > 1 {
			logger.Debugf("remove %s from cache, age: %d", lastKey, now.Sub(lastValue.atime))
			c.delete(lastKey, lastValue.page)
			cnt = 0
			if c.used < c.capacity {
				break
			}
		}
	}
}

func (c *memcache) stage(key string, data []byte, keepCache bool) (string, error) {
	return "", errors.New("not supported")
}
func (c *memcache) uploaded(key string, size int)  {}
func (c *memcache) scanStaging() map[string]string { return nil }
