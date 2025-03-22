package tinycache

import (
	"sync"
	"time"
)

type entry[T any] struct {
	value   T
	expires time.Time
}

type Cache[T any] struct {
	store      sync.Map
	defaultTTL time.Duration
	closeCh    chan struct{}
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
	c.store.Delete(key)
}

func (c *Cache[T]) Set(key string, value T) {
	c.store.Store(key, entry[T]{value: value, expires: time.Now().Add(c.defaultTTL)})
}

func (c *Cache[T]) SetTTL(key string, value T, ttl time.Duration) {
	c.store.Store(key, entry[T]{value: value, expires: time.Now().Add(ttl)})
}

func (c *Cache[T]) Get(key string) (T, bool) {
	if v, ok := c.store.Load(key); ok {
		if entry, ok := v.(entry[T]); ok {
			if entry.expires.After(time.Now()) {
				return entry.value, true
			}
			c.Delete(key)
		}
	}

	var zero T
	return zero, false
}

func (c *Cache[T]) Reap() {
	now := time.Now()
	c.store.Range(func(key, value any) bool {
		if entry, ok := value.(entry[T]); ok {
			if entry.expires.Before(now) {
				c.Delete(key.(string))
			}
		}
		return true
	})
}

func (c *Cache[T]) Close() {
	close(c.closeCh)
	c.store.Range(func(key, _ any) bool {
		c.store.Delete(key)
		return true
	})
}
