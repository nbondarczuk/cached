package cached

import (
	"fmt"
	"log"
	"sync"
	"time"
)

var (
	MAX_CACHE_SIZE          = 1000
	CACHE_EXPIRY_TIME       = 5 * time.Minute
	CACHE_EXPIRY_SLEEP_TIME = 1 * time.Minute
)

var cached = NewFunctionCache()

type FunctionCache struct {
	m        sync.Mutex
	cache    map[string]interface{}
	entry    map[string]time.Time
	inflight map[string]bool
	mutex    map[string]*sync.Mutex
	cond     map[string]*sync.Cond
	waits    map[string]int
}

func NewFunctionCache() *FunctionCache {
	fc := &FunctionCache{
		cache:    make(map[string]interface{}),
		entry:    make(map[string]time.Time),
		inflight: make(map[string]bool),
		mutex:    make(map[string]*sync.Mutex),
		cond:     make(map[string]*sync.Cond),
		waits:    make(map[string]int),
	}
	// Feature 3. Expiration of the cache

	go func() {
		for {
			time.Sleep(CACHE_EXPIRY_SLEEP_TIME)
			fc.m.Lock()
			for k, t := range fc.entry {
				if time.Since(t) > CACHE_EXPIRY_TIME {
					delete(fc.cache, k)
					delete(fc.entry, k)
					delete(fc.inflight, k)
				}
			}
			fc.m.Unlock()
		}
	}()

	return fc
}

// NewCachedFunction creates a cached version of the given function with memoization, in-flight request deduplication, and expiration.
func NewCachedFunction(f func(args ...interface{}) interface{}) func(args ...interface{}) interface{} {
	return func(args ...interface{}) interface{} {
		key := fmt.Sprintf("%v", args)
		// Critical section to ensure safety
		cached.m.Lock()
		defer cached.m.Unlock()

		// Feature 4. Capacity limit
		if len(cached.cache) >= MAX_CACHE_SIZE {
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

		// Feature 1. Memoization
		if result, found := cached.cache[key]; found {
			log.Printf("Cache hit: %v -> %v\n", key, result)
			return result
		}

		// Feature 2. In-Flight Request Deduplication - register waiter
		if _, found := cached.inflight[key]; found {
			cached.cond[key].L.Lock()
			cached.waits[key]++
			log.Printf("Waiting for slot: %v, waits: %d\n", key, cached.waits[key])
			cached.cond[key].Wait()
			cached.cond[key].L.Unlock()
			if result, found := cached.cache[key]; found {
				log.Printf("Cache hit after waiting: %v\n", key)
				return result
			}

			// If the cache is still not available, return nil
			log.Println("Cache not available after waiting, returning nil")
			return nil
		}

		// Call the original function and cache the result
		cached.inflight[key] = true
		cached.mutex[key] = &sync.Mutex{}
		cached.cond[key] = sync.NewCond(cached.mutex[key])

		// Call the original function
		log.Printf("Calling original function: %v\n", key)
		result := f(args...)
		log.Printf("Original function result: %v\n", result)

		cached.cache[key] = result
		cached.entry[key] = time.Now()

		// Feature 2. In-Flight Request Deduplication - notify waiters
		if _, found := cached.inflight[key]; found {
			cached.cond[key].L.Lock()
			log.Printf("Notifying waiters for slot: %v\n", key)
			cached.cond[key].Broadcast()
			cached.cond[key].L.Unlock()
			delete(cached.inflight, key)
			delete(cached.cond, key)
		}

		// Return the result with time stamp of it
		log.Printf("Returning result: %v -> %v\n", key, result)
		return result
	}
}
