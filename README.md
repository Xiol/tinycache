# tinycache

tinycache is a tiny, no-frills, in-memory cache for a single type. Safe for concurrent usage. Lazy expiration by default but a reaping option can be enabled if needed.

## Usage

Grab the module:

```
go get -u github.com/Xiol/tinycache
```

Then use it:

```go
package main

import (
	"fmt"
	"time"

	"github.com/Xiol/tinycache"
)

func main() {
	cache := tinycache.New[string](
		// WithTTL sets the default TTL for all keys. TTLs for
		// individual keys can be set with SetTTL(), and this
		// overrides the default. If omitted, keys will not expire.
		tinycache.WithTTL(1*time.Second),

		// When WithReapInterval is provided, a reaper goroutine
		// is started which removes expired keys at the specified
		// interval. If omitted, keys will not be cleaned up
		// automatically. Call Reap() yourself, or Delete() keys
		// manually.
		tinycache.WithReapInterval(2*time.Second),
	)
	// Closing the cache will stop the reaper goroutine and remove
	// all cache entries
	defer cache.Close()

	// Set key/value pairs in the cache
	cache.Set("key1", "value1")
	cache.SetTTL("key2", "value2", 3*time.Second)

	// Retrieve values from the cache
	v, ok := cache.Get("key1")
	if ok {
		fmt.Println(v)
	}

	v2, ok := cache.Get("key2")
	if ok {
		fmt.Println(v2)
	}

	// Wait for TTL
	time.Sleep(2 * time.Second)

	_, ok = cache.Get("key1")
	if ok {
		panic("key1 should be expired")
	} else {
		fmt.Println("key1 expired")
	}

	vc2, ok := cache.Get("key2")
	if ok {
		fmt.Printf("key2 still alive: %s\n", vc2)
	} else {
		panic("key2 should still be alive")
	}

	// Delete a key from the cache
	cache.Delete("key2")
}
```