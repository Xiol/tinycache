package tinycache

import (
	"fmt"
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
