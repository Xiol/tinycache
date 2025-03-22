package tinycache

import (
	"sync"
	"sync/atomic"
	"time"
)

const (
	// noExpiration represents a timestamp that will never expire (maximum int64 value)
	noExpiration int64 = 1<<63 - 1
)

type entry[T any] struct {
	value       T
	expiresUnix int64
}

type Cache[T any] struct {
	mu         sync.RWMutex
	store      map[string]*entry[T]
	defaultTTL time.Duration
	closeCh    chan struct{}
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
		entryPool: sync.Pool{
			New: func() any {
				return new(entry[T])
			},
		},
	}

	if options.reapInterval > 0 {
		cache.closeCh = make(chan struct{})
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

func (c *Cache[T]) Delete(key string) {
	c.mu.Lock()
	if e, ok := c.store[key]; ok {
		delete(c.store, key)
		c.entryPool.Put(e)
	}
	c.mu.Unlock()
}

func (c *Cache[T]) Set(key string, value T) {
	var expiresUnix int64

	if c.defaultTTL == 0 {
		expiresUnix = noExpiration
	} else {
		expiresUnix = time.Now().Add(c.defaultTTL).Unix()
	}

	e := c.entryPool.Get().(*entry[T])
	e.value = value
	atomic.StoreInt64(&e.expiresUnix, expiresUnix)

	c.mu.Lock()
	if oldEntry, exists := c.store[key]; exists {
		c.entryPool.Put(oldEntry)
	}
	c.store[key] = e
	c.mu.Unlock()
}

func (c *Cache[T]) SetTTL(key string, value T, ttl time.Duration) {
	var expiresUnix int64

	if ttl == 0 {
		expiresUnix = noExpiration
	} else {
		expiresUnix = time.Now().Add(ttl).Unix()
	}

	e := c.entryPool.Get().(*entry[T])
	e.value = value
	atomic.StoreInt64(&e.expiresUnix, expiresUnix)

	c.mu.Lock()
	if oldEntry, exists := c.store[key]; exists {
		c.entryPool.Put(oldEntry)
	}
	c.store[key] = e
	c.mu.Unlock()
}

func (c *Cache[T]) SetPermanent(key string, value T) {
	e := c.entryPool.Get().(*entry[T])
	e.value = value
	atomic.StoreInt64(&e.expiresUnix, noExpiration)

	c.mu.Lock()
	if oldEntry, exists := c.store[key]; exists {
		c.entryPool.Put(oldEntry)
	}
	c.store[key] = e
	c.mu.Unlock()
}

func (c *Cache[T]) Get(key string) (T, bool) {
	var value T
	var found bool

	c.mu.RLock()
	if e, ok := c.store[key]; ok {
		expiresUnix := atomic.LoadInt64(&e.expiresUnix)

		now := time.Now().Unix()
		if expiresUnix == noExpiration || expiresUnix > now {
			value = e.value
			found = true
		} else {
			// Need to switch to write lock to delete expired entry
			c.mu.RUnlock()
			c.Delete(key)
			return value, false
		}
	}
	c.mu.RUnlock()

	return value, found
}

func (c *Cache[T]) Reap() {
	var keysToDelete []string

	now := time.Now().Unix()

	c.mu.RLock()
	for key, e := range c.store {
		expiresUnix := atomic.LoadInt64(&e.expiresUnix)
		if expiresUnix != noExpiration && expiresUnix < now {
			keysToDelete = append(keysToDelete, key)
		}
	}
	c.mu.RUnlock()

	if len(keysToDelete) > 0 {
		c.mu.Lock()
		for _, key := range keysToDelete {
			if e, ok := c.store[key]; ok {
				expiresUnix := atomic.LoadInt64(&e.expiresUnix)
				if expiresUnix != noExpiration && expiresUnix < now {
					delete(c.store, key)
					c.entryPool.Put(e)
				}
			}
		}
		c.mu.Unlock()
	}
}

func (c *Cache[T]) Close() {
	if c.closeCh != nil {
		close(c.closeCh)
	}

	c.mu.Lock()
	for key, e := range c.store {
		c.entryPool.Put(e)
		delete(c.store, key)
	}
	c.mu.Unlock()
}
