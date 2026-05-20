package tinycache

import (
	"sync"
	"time"
)

const (
	// noExpiration represents a timestamp that will never expire (maximum int64 value)
	noExpiration int64 = 1<<63 - 1
)

type entry[T any] struct {
	value      T
	expiresAt  int64
}

type Cache[T any] struct {
	mu         sync.RWMutex
	store      map[string]*entry[T]
	defaultTTL time.Duration
	closeCh    chan struct{}
	stopReaper func()
	entryPool  sync.Pool
}

type cacheOptions struct {
	defaultTTL   time.Duration
	reapInterval time.Duration
}

type Option func(*cacheOptions)

func WithTTL(ttl time.Duration) Option {
	return func(o *cacheOptions) {
		o.defaultTTL = ttl
	}
}

func WithReapInterval(interval time.Duration) Option {
	return func(o *cacheOptions) {
		o.reapInterval = interval
	}
}

func New[T any](opts ...Option) *Cache[T] {
	options := &cacheOptions{}
	for _, opt := range opts {
		opt(options)
	}

	cache := &Cache[T]{
		defaultTTL: options.defaultTTL,
		store:      make(map[string]*entry[T]),
		stopReaper: func() {},
		entryPool: sync.Pool{
			New: func() any {
				return new(entry[T])
			},
		},
	}

	if options.reapInterval > 0 {
		cache.closeCh = make(chan struct{})
		cache.stopReaper = sync.OnceFunc(func() { close(cache.closeCh) })
		go func() {
			ticker := time.NewTicker(options.reapInterval)
			defer ticker.Stop()
			for {
				select {
				case <-cache.closeCh:
					return
				case <-ticker.C:
					cache.Reap()
				}
			}
		}()
	}

	return cache
}

func expiryFor(ttl time.Duration) int64 {
	if ttl == 0 {
		return noExpiration
	}
	return time.Now().Add(ttl).UnixNano()
}

func (c *Cache[T]) Delete(key string) {
	c.mu.Lock()
	if e, ok := c.store[key]; ok {
		delete(c.store, key)
		c.entryPool.Put(e)
	}
	c.mu.Unlock()
}

func (c *Cache[T]) set(key string, value T, expiresAt int64) {
	e := c.entryPool.Get().(*entry[T])
	e.value = value
	e.expiresAt = expiresAt

	c.mu.Lock()
	if old, exists := c.store[key]; exists {
		c.entryPool.Put(old)
	}
	c.store[key] = e
	c.mu.Unlock()
}

func (c *Cache[T]) Set(key string, value T) {
	c.set(key, value, expiryFor(c.defaultTTL))
}

func (c *Cache[T]) SetTTL(key string, value T, ttl time.Duration) {
	c.set(key, value, expiryFor(ttl))
}

func (c *Cache[T]) SetPermanent(key string, value T) {
	c.set(key, value, noExpiration)
}

func (c *Cache[T]) Get(key string) (T, bool) {
	var zero T

	c.mu.RLock()
	e, ok := c.store[key]
	if !ok {
		c.mu.RUnlock()
		return zero, false
	}

	now := time.Now().UnixNano()
	if e.expiresAt == noExpiration || e.expiresAt > now {
		value := e.value
		c.mu.RUnlock()
		return value, true
	}
	c.mu.RUnlock()

	// Expired: re-check expiry under the write lock before deleting, so a
	// concurrent Set that happened between RUnlock and Lock isn't clobbered.
	c.mu.Lock()
	if cur, exists := c.store[key]; exists {
		now = time.Now().UnixNano()
		if cur.expiresAt != noExpiration && cur.expiresAt < now {
			delete(c.store, key)
			c.entryPool.Put(cur)
		}
	}
	c.mu.Unlock()
	return zero, false
}

func (c *Cache[T]) Reap() {
	var keysToDelete []string

	now := time.Now().UnixNano()

	c.mu.RLock()
	for key, e := range c.store {
		if e.expiresAt != noExpiration && e.expiresAt < now {
			keysToDelete = append(keysToDelete, key)
		}
	}
	c.mu.RUnlock()

	if len(keysToDelete) == 0 {
		return
	}

	c.mu.Lock()
	now = time.Now().UnixNano()
	for _, key := range keysToDelete {
		if e, ok := c.store[key]; ok {
			if e.expiresAt != noExpiration && e.expiresAt < now {
				delete(c.store, key)
				c.entryPool.Put(e)
			}
		}
	}
	c.mu.Unlock()
}

func (c *Cache[T]) Close() {
	c.stopReaper()

	c.mu.Lock()
	for key, e := range c.store {
		c.entryPool.Put(e)
		delete(c.store, key)
	}
	c.mu.Unlock()
}
