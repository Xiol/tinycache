package tinycache

import (
	"fmt"
	"math/rand"
	"strconv"
	"sync"
	"testing"
	"time"
)

func TestNewCacheWithDefaultTTL(t *testing.T) {
	cache := New[int](WithTTL(5 * time.Second))

	if cache.defaultTTL != 5*time.Second {
		t.Errorf("expected defaultTTL to be 5s, got %v", cache.defaultTTL)
	}
}

func TestNewCacheWithReapInterval(t *testing.T) {
	cache := New[int](WithReapInterval(1 * time.Second))
	defer cache.Close()

	time.Sleep(2 * time.Second)

	cache.SetTTL("key", 42, -1*time.Second)

	time.Sleep(2 * time.Second)

	if _, ok := cache.Get("key"); ok {
		t.Errorf("expected key to be deleted by reaper")
	}
}

func TestCacheSetAndGet(t *testing.T) {
	cache := New[int](WithTTL(5 * time.Second))

	cache.Set("key", 42)
	value, ok := cache.Get("key")
	if !ok || value != 42 {
		t.Errorf("expected to get 42, got %v", value)
	}
}

func TestCacheSetTTL(t *testing.T) {
	cache := New[int](WithTTL(5 * time.Second))

	cache.SetTTL("key", 42, 1*time.Second)
	value, ok := cache.Get("key")
	if !ok || value != 42 {
		t.Errorf("expected to get 42, got %v", value)
	}

	time.Sleep(2 * time.Second)

	if _, ok := cache.Get("key"); ok {
		t.Errorf("expected key to be expired")
	}
}

func TestCacheSetPermanent(t *testing.T) {
	cache := New[int](WithTTL(1 * time.Second))

	// Regular entry should expire
	cache.Set("temp-key", 100)

	// Permanent entry should never expire
	cache.SetPermanent("perm-key", 42)

	// Zero TTL should also create permanent entry
	cache.SetTTL("zero-ttl-key", 84, 0)

	// Wait for regular entry to expire
	time.Sleep(2 * time.Second)

	// Check regular entry is gone
	if _, ok := cache.Get("temp-key"); ok {
		t.Errorf("expected temporary key to be expired")
	}

	// Check permanent entry is still there
	value, ok := cache.Get("perm-key")
	if !ok || value != 42 {
		t.Errorf("expected permanent key to still exist with value 42, got %v, exists: %v", value, ok)
	}

	// Check zero TTL entry is still there
	value, ok = cache.Get("zero-ttl-key")
	if !ok || value != 84 {
		t.Errorf("expected zero TTL key to still exist with value 84, got %v, exists: %v", value, ok)
	}

	// Reap shouldn't affect permanent entries
	cache.Reap()

	value, ok = cache.Get("perm-key")
	if !ok || value != 42 {
		t.Errorf("expected permanent key to still exist after reap, got %v, exists: %v", value, ok)
	}
}

func TestCacheZeroDefaultTTL(t *testing.T) {
	// Create cache with zero default TTL (all entries permanent by default)
	cache := New[int](WithTTL(0))

	cache.Set("key1", 42)
	cache.Set("key2", 84)

	// Wait some time
	time.Sleep(2 * time.Second)

	// Check entries still exist
	value, ok := cache.Get("key1")
	if !ok || value != 42 {
		t.Errorf("expected key1 to exist with value 42, got %v, exists: %v", value, ok)
	}

	value, ok = cache.Get("key2")
	if !ok || value != 84 {
		t.Errorf("expected key2 to exist with value 84, got %v, exists: %v", value, ok)
	}

	// Run reaper
	cache.Reap()

	// Check entries still exist after reap
	value, ok = cache.Get("key1")
	if !ok || value != 42 {
		t.Errorf("expected key1 to still exist after reap, got %v, exists: %v", value, ok)
	}
}

func TestCacheNegativeTTL(t *testing.T) {
	cache := New[int](WithTTL(5 * time.Second))

	// Negative TTL should expire immediately
	cache.SetTTL("key", 42, -1*time.Second)

	// Should be expired already
	if _, ok := cache.Get("key"); ok {
		t.Errorf("expected key with negative TTL to be expired immediately")
	}
}

func TestCacheReap(t *testing.T) {
	cache := New[int](WithTTL(5 * time.Second))

	cache.SetTTL("key", 42, -1*time.Second)
	cache.Reap()

	if _, ok := cache.Get("key"); ok {
		t.Errorf("expected key to be deleted by reaper")
	}
}

func TestCacheSetAndGetStruct(t *testing.T) {
	type testStruct struct {
		Name  string
		Value int
	}

	cache := New[testStruct](WithTTL(5 * time.Second))

	expected := testStruct{Name: "test", Value: 42}
	cache.Set("key", expected)

	value, ok := cache.Get("key")
	if !ok {
		t.Errorf("expected to get a value, but got none")
	}
	if value.Name != expected.Name || value.Value != expected.Value {
		t.Errorf("expected to get %v, got %v", expected, value)
	}
}

func TestCacheConcurrentSetAndGet(t *testing.T) {
	cache := New[int](WithTTL(5 * time.Second))

	var wg sync.WaitGroup
	numGoroutines := 100
	numOperations := 1000

	// Concurrently set values
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				cache.Set(fmt.Sprintf("key-%d-%d", id, j), id*j)
			}
		}(i)
	}

	// Concurrently get values
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				cache.Get(fmt.Sprintf("key-%d-%d", id, j))
			}
		}(i)
	}

	wg.Wait()

	// Verify some values
	for i := 0; i < numGoroutines; i++ {
		for j := 0; j < numOperations; j++ {
			value, ok := cache.Get(fmt.Sprintf("key-%d-%d", i, j))
			if !ok {
				t.Errorf("expected to get a value for key-%d-%d, but got none", i, j)
			}
			if value != i*j {
				t.Errorf("expected to get %d for key-%d-%d, but got %d", i*j, i, j, value)
			}
		}
	}
}

func TestCloseWithNilChannel(t *testing.T) {
	// Create a cache without a reaper (so closeCh is nil)
	cache := New[int]()

	// This should not panic
	cache.Close()
}

// ******* BENCHMARKS *******

// Small struct for testing struct caching performance
type benchStruct struct {
	ID       int
	Name     string
	Value    float64
	Created  time.Time
	Metadata map[string]string
}

// Create a medium-sized struct for benchmarking
func newBenchStruct(id int) benchStruct {
	return benchStruct{
		ID:      id,
		Name:    fmt.Sprintf("test-item-%d", id),
		Value:   float64(id) * 1.5,
		Created: time.Now(),
		Metadata: map[string]string{
			"type":        "benchmark",
			"description": "A test item for benchmarking struct performance",
			"category":    strconv.Itoa(id % 10),
		},
	}
}

// Basic set/get benchmark for integers
func BenchmarkIntCache_Set(b *testing.B) {
	cache := New[int](WithTTL(5 * time.Minute))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cache.Set(strconv.Itoa(i), i)
	}
}

func BenchmarkIntCache_Get(b *testing.B) {
	cache := New[int](WithTTL(5 * time.Minute))

	// Pre-populate the cache
	for i := 0; i < 10000; i++ {
		cache.Set(strconv.Itoa(i), i)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Get a mix of existing and non-existing keys
		cache.Get(strconv.Itoa(i % 20000))
	}
}

// Benchmark for struct caching
func BenchmarkStructCache_Set(b *testing.B) {
	cache := New[benchStruct](WithTTL(5 * time.Minute))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cache.Set(strconv.Itoa(i), newBenchStruct(i))
	}
}

func BenchmarkStructCache_Get(b *testing.B) {
	cache := New[benchStruct](WithTTL(5 * time.Minute))

	// Pre-populate the cache
	for i := 0; i < 10000; i++ {
		cache.Set(strconv.Itoa(i), newBenchStruct(i))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Get a mix of existing and non-existing keys
		cache.Get(strconv.Itoa(i % 20000))
	}
}

// Benchmark TTL expiration handling
func BenchmarkCache_GetWithExpiredTTL(b *testing.B) {
	cache := New[int](WithTTL(1 * time.Nanosecond))

	// Pre-populate the cache with already expired items
	for i := 0; i < 10000; i++ {
		cache.Set(strconv.Itoa(i), i)
	}

	// Wait to ensure expiration
	time.Sleep(1 * time.Millisecond)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Getting expired items forces expiration check
		cache.Get(strconv.Itoa(i % 10000))
	}
}

// Benchmark the Reap function
func BenchmarkCache_Reap(b *testing.B) {
	// Create sets of 10,000 entries for each iteration
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		b.StopTimer()
		cache := New[int](WithTTL(1 * time.Nanosecond))

		// Add many items
		for j := 0; j < 10000; j++ {
			cache.Set(strconv.Itoa(j), j)
		}

		// Wait to ensure expiration
		time.Sleep(1 * time.Millisecond)
		b.StartTimer()

		// Benchmark the reap operation
		cache.Reap()
	}
}

// Benchmark permanent entries
func BenchmarkCache_GetPermanent(b *testing.B) {
	cache := New[int](WithTTL(5 * time.Minute))

	// Pre-populate with permanent entries
	for i := 0; i < 10000; i++ {
		cache.SetPermanent(strconv.Itoa(i), i)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cache.Get(strconv.Itoa(i % 10000))
	}
}

// Benchmark concurrent access
func BenchmarkCache_ConcurrentAccess(b *testing.B) {
	cache := New[int](WithTTL(5 * time.Minute))

	// Pre-populate
	for i := 0; i < 10000; i++ {
		cache.Set(strconv.Itoa(i), i)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		// Create a local random source for each goroutine
		r := rand.New(rand.NewSource(time.Now().UnixNano()))

		for pb.Next() {
			// 80% reads, 20% writes to simulate typical cache usage
			if r.Float32() < 0.8 {
				cache.Get(strconv.Itoa(r.Intn(20000)))
			} else {
				cache.Set(strconv.Itoa(r.Intn(20000)), r.Int())
			}
		}
	})
}

// Benchmark entry reuse via sync.Pool
func BenchmarkCache_EntryReuse(b *testing.B) {
	cache := New[int](WithTTL(1 * time.Millisecond))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Set, wait for expiration, then set again to trigger pool reuse
		key := "test-key"
		cache.Set(key, i)

		if i%100 == 0 {
			// Periodically force expiration to trigger pool reuse
			time.Sleep(2 * time.Millisecond)
			cache.Reap()
		}

		// Set the same key again, which should reuse the entry from the pool
		cache.Set(key, i+1)
	}
}

// Benchmark comparing different sized values
func BenchmarkCache_DifferentSizes(b *testing.B) {
	// Test with small values
	b.Run("SmallValue", func(b *testing.B) {
		cache := New[int](WithTTL(5 * time.Minute))
		for i := 0; i < b.N; i++ {
			cache.Set(strconv.Itoa(i), i)
			cache.Get(strconv.Itoa(i))
		}
	})

	// Test with medium strings
	b.Run("MediumString", func(b *testing.B) {
		cache := New[string](WithTTL(5 * time.Minute))
		for i := 0; i < b.N; i++ {
			value := fmt.Sprintf("medium-sized-string-value-%d-for-testing-performance", i)
			cache.Set(strconv.Itoa(i), value)
			cache.Get(strconv.Itoa(i))
		}
	})

	// Test with large structs
	b.Run("LargeStruct", func(b *testing.B) {
		type largeStruct struct {
			ID        int
			Name      string
			Values    [100]float64
			Tags      []string
			Timestamp time.Time
			Active    bool
			Metadata  map[string]interface{}
		}

		cache := New[largeStruct](WithTTL(5 * time.Minute))

		for i := 0; i < b.N; i++ {
			// Create a large struct with arrays and maps
			value := largeStruct{
				ID:        i,
				Name:      fmt.Sprintf("large-struct-%d", i),
				Timestamp: time.Now(),
				Active:    true,
				Tags:      []string{"benchmark", "performance", "testing", "large"},
				Metadata: map[string]interface{}{
					"description": "A large struct for benchmarking",
					"version":     1.0,
					"created":     time.Now().String(),
					"nested": map[string]string{
						"level1": "value1",
						"level2": "value2",
					},
				},
			}

			// Fill the array with values
			for j := 0; j < 100; j++ {
				value.Values[j] = float64(i) * float64(j)
			}

			cache.Set(strconv.Itoa(i), value)
			cache.Get(strconv.Itoa(i))
		}
	})
}

// Compare map with mutex vs sync.Map for different workloads
func BenchmarkSyncMapVsMapMutex(b *testing.B) {
	// Helper for testing a map with mutex
	type mutexCache struct {
		mu    sync.RWMutex
		store map[string]int
	}

	// Test with read-heavy workload (90% reads)
	b.Run("ReadHeavy", func(b *testing.B) {
		// Test sync.Map
		b.Run("SyncMap", func(b *testing.B) {
			var sm sync.Map
			for i := 0; i < 1000; i++ {
				sm.Store(strconv.Itoa(i), i)
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if rand.Float32() < 0.9 {
					// 90% reads
					sm.Load(strconv.Itoa(rand.Intn(2000)))
				} else {
					// 10% writes
					sm.Store(strconv.Itoa(rand.Intn(2000)), i)
				}
			}
		})

		// Test map with mutex
		b.Run("MapMutex", func(b *testing.B) {
			mc := mutexCache{
				store: make(map[string]int, 1000),
			}
			for i := 0; i < 1000; i++ {
				mc.store[strconv.Itoa(i)] = i
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if rand.Float32() < 0.9 {
					// 90% reads
					mc.mu.RLock()
					_ = mc.store[strconv.Itoa(rand.Intn(2000))]
					mc.mu.RUnlock()
				} else {
					// 10% writes
					mc.mu.Lock()
					mc.store[strconv.Itoa(rand.Intn(2000))] = i
					mc.mu.Unlock()
				}
			}
		})
	})

	// Test with write-heavy workload (50% writes)
	b.Run("WriteHeavy", func(b *testing.B) {
		// Test sync.Map
		b.Run("SyncMap", func(b *testing.B) {
			var sm sync.Map
			for i := 0; i < 1000; i++ {
				sm.Store(strconv.Itoa(i), i)
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if rand.Float32() < 0.5 {
					// 50% reads
					sm.Load(strconv.Itoa(rand.Intn(2000)))
				} else {
					// 50% writes
					sm.Store(strconv.Itoa(rand.Intn(2000)), i)
				}
			}
		})

		// Test map with mutex
		b.Run("MapMutex", func(b *testing.B) {
			mc := mutexCache{
				store: make(map[string]int, 1000),
			}
			for i := 0; i < 1000; i++ {
				mc.store[strconv.Itoa(i)] = i
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if rand.Float32() < 0.5 {
					// 50% reads
					mc.mu.RLock()
					_ = mc.store[strconv.Itoa(rand.Intn(2000))]
					mc.mu.RUnlock()
				} else {
					// 50% writes
					mc.mu.Lock()
					mc.store[strconv.Itoa(rand.Intn(2000))] = i
					mc.mu.Unlock()
				}
			}
		})
	})
}
