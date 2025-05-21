package cached

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"sync"
	"time"
)

func init() {
	if os.Getenv("DEBUG") == "" {
		log.SetOutput(io.Discard)
	}
}

var (
	// MaxCacheSize is a max numer of entries in the cache
	MaxCacheSize = 1000
	// CacheExpiryTime is a cache expiry time and sleep time for the expiration goroutine
	CacheExpiryTime = 5 * time.Minute
	// CacheExpirySleepTime is a cache expiry sleep time
	CacheExpirySleepTime = 1 * time.Minute
)

var cached = NewFunctionCache(context.Background())

// FunctionCache is a structure that holds the cache, entry time, in-flight requests, and mutexes for synchronization.
type FunctionCache struct {
	m        sync.Mutex
	cache    map[string]interface{}
	entry    map[string]time.Time
	inflight map[string]bool
	mutex    map[string]*sync.Mutex
	cond     map[string]*sync.Cond
	waits    map[string]int
}

// NewFunctionCache creates a new FunctionCache instance.
func NewFunctionCache(ctx context.Context) *FunctionCache {
	fc := &FunctionCache{
		cache:    make(map[string]interface{}),
		entry:    make(map[string]time.Time),
		inflight: make(map[string]bool),
		mutex:    make(map[string]*sync.Mutex),
		cond:     make(map[string]*sync.Cond),
		waits:    make(map[string]int),
	}

	// Feature 3. Expiration of the cache
	go func(ctx context.Context) {
		for {
			time.Sleep(CacheExpirySleepTime)
			if ctx.Err() != nil {
				return
			}
			fc.m.Lock()
			for k, t := range fc.entry {
				if time.Since(t) > CacheExpiryTime {
					delete(fc.cache, k)
					delete(fc.entry, k)
				}
			}
			fc.m.Unlock()
		}
	}(ctx)

	return fc
}

// NewCachedFunction creates a cached version of the given function with memoization, in-flight request deduplication, and expiration.
func NewCachedFunction(f func(args ...interface{}) interface{}) func(args ...interface{}) interface{} {
	return func(args ...interface{}) interface{} {
		key := fmt.Sprintf("%v", args)

		// Feature 4. Capacity limit
		cached.m.Lock()
		if len(cached.cache) >= MaxCacheSize {
			// Remove the oldest entry making new slot available
			var oldestKey string
			var oldestTime time.Time
			for k, t := range cached.entry {
				if oldestTime.IsZero() || t.Before(oldestTime) {
					oldestKey = k
					oldestTime = t
				}
			}
			delete(cached.cache, oldestKey)
			delete(cached.entry, oldestKey)
			log.Printf("Evicted oldest entry: %v, cache size: %d\n", oldestKey, len(cached.cache))
		}
		cached.m.Unlock()

		// Feature 1. Memoization
		cached.m.Lock()
		if result, found := cached.cache[key]; found {
			log.Printf("Cache hit: %v -> %v\n", key, result)
			cached.m.Unlock()
			return result
		}
		cached.m.Unlock()

		// Feature 2. In-Flight Request Deduplication - register waiter
		cached.m.Lock()
		if _, found := cached.inflight[key]; found {
			cached.cond[key].L.Lock()
			cached.waits[key]++
			log.Printf("Waiting for slot: %v, waits: %d\n", key, cached.waits[key])
			cached.m.Unlock()
			cached.cond[key].Wait()
			cached.cond[key].L.Unlock()
			cached.m.Lock()
			if result, found := cached.cache[key]; found {
				log.Printf("Cache hit after waiting: %v -> %v\n", key, result)
				cached.m.Unlock()
				return result
			}

			// If the cache is still not available, return nil
			log.Println("Cache not available after waiting, returning nil")
			cached.m.Unlock()
			return nil
		}
		cached.m.Unlock()

		// Call the original function and cache the result
		cached.m.Lock()
		cached.inflight[key] = true
		cached.mutex[key] = &sync.Mutex{}
		cached.cond[key] = sync.NewCond(cached.mutex[key])
		cached.m.Unlock()

		// Call the original function
		log.Printf("Calling original function: %v\n", key)
		result := f(args...)
		log.Printf("Original function result: %v -> %v\n", key, result)

		cached.m.Lock()
		cached.cache[key] = result
		cached.entry[key] = time.Now()
		cached.m.Unlock()

		// Feature 2. In-Flight Request Deduplication - notify waiters
		cached.m.Lock()
		if _, found := cached.inflight[key]; found {
			cached.cond[key].L.Lock()
			log.Printf("Notifying waiters for slot: %v\n", key)
			cached.cond[key].Broadcast()
			cached.cond[key].L.Unlock()
			delete(cached.inflight, key)
		}
		cached.m.Unlock()

		// Return the result with time stamp of it
		log.Printf("Returning result: %v -> %v\n", key, result)
		return result
	}
}
