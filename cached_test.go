package cached

import (
	"context"
	"fmt"
	"log"
	"sync"
	"testing"
	"time"
)

// Test: Return values are correctly cached
func TestCachedReturnValues(t *testing.T) {
	// mock cache
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cached = NewFunctionCache(ctx)

	// Define a simple function to be cached
	f := func(args ...interface{}) interface{} {
		return args[0].(int) + args[1].(int)
	}

	// Create a cached version of the function
	cachedFunc := NewCachedFunction(f)

	// Call the cached function with the same arguments multiple times
	result1 := cachedFunc(1, 2)
	result2 := cachedFunc(1, 2)

	// Check if the results are the same
	if result1 != result2 {
		t.Errorf("Expected %v, got %v", result1, result2)
	}

	// Call the cached function with different arguments
	result3 := cachedFunc(2, 3)

	// Check if the results are different
	if result1 == result3 {
		t.Errorf("Expected different results for different arguments")
	}
}

// Test: Results expire after X minutes
func TestCachedFunctionExpiryTimeLimit(t *testing.T) {
	// mock timers
	CacheExpiryTime = time.Second
	CacheExpirySleepTime = time.Second
	// mock cache
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cached = NewFunctionCache(ctx)

	// Define a simple function to be cached
	f := func(args ...interface{}) interface{} {
		return args[0].(int) + args[1].(int)
	}

	// Create a cached version of the function
	cachedFunc := NewCachedFunction(f)

	// Call the cached function with some arguments
	cachedFunc(1, 2)
	args := []interface{}{1, 2}
	key1 := fmt.Sprintf("%v", args)

	// Wait for the cache to expire
	time.Sleep(2 * CacheExpiryTime)

	_, ok := cached.cache[key1]
	if ok {
		t.Errorf("Expected cache to be expired, but it still exists: %v", cached.cache[key1])
	}
}

// Test: Cache never exceeds MaxCacheSize entries
func TestCachedFunctionCapacityLimit(t *testing.T) {
	// mock timers
	CacheExpiryTime = time.Second * 10
	CacheExpirySleepTime = time.Second * 10
	// mock cache
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cached = NewFunctionCache(ctx)

	// Define a simple function to be cached
	f := func(args ...interface{}) interface{} {
		return args[0].(int) + args[1].(int)
	}

	// Create a cached version of the function
	cachedFunc := NewCachedFunction(f)

	// Fill the cache to its maximum capacity
	for i := 0; i < MaxCacheSize; i++ {
		cachedFunc(i, i+1)
	}

	// Call the cached function with new arguments to trigger eviction
	cachedFunc(MaxCacheSize, MaxCacheSize+1)

	// Check if the cache size is within the limit
	if len(cached.cache) > MaxCacheSize {
		t.Errorf("Expected cache size to be within limit, but got %d", len(cached.cache))
	}
}

// Test: Oldest entries are evicted when the cache is full
func TestCachedFunctionEviction(t *testing.T) {
	// mock timers
	CacheExpiryTime = time.Second
	CacheExpirySleepTime = time.Second
	// mock cache
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cached = NewFunctionCache(ctx)

	// Define a simple function to be cached
	f := func(args ...interface{}) interface{} {
		return args[0].(int) + args[1].(int)
	}

	// Create a cached version of the function
	cachedFunc := NewCachedFunction(f)

	var first string

	// Fill the cache to its maximum capacity
	for i := 0; i < MaxCacheSize; i++ {
		if i == 0 {
			args := []interface{}{i, i + 1}
			first = fmt.Sprintf("%v", args)
		}

		cachedFunc(i, i+1)
	}

	// Call the cached function with new arguments to trigger eviction
	cachedFunc(MaxCacheSize, MaxCacheSize+1)

	// Check if the oldest entry is evicted
	if _, ok := cached.cache[first]; ok {
		t.Errorf("Expected oldest entry to be evicted, but it still exists")
	}
}

// Test: Concurrent calls with same input are deduplicated
func TestCachedFunctionConcurrentCalls(t *testing.T) {
	// mock timers
	CacheExpiryTime = time.Second
	CacheExpirySleepTime = time.Second
	// mock cache
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cached = NewFunctionCache(ctx)

	var calls int

	// Define a simple function to be cached
	f := func(args ...interface{}) interface{} {
		calls++
		return args[0].(int) + args[1].(int)
	}

	// Create a cached version of the function
	cachedFunc := NewCachedFunction(f)

	var result1, result2 interface{}
	var wg sync.WaitGroup

	// Call the cached function concurrently with the same arguments
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			result1 = cachedFunc(1, 2)
			time.Sleep(time.Millisecond)
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			result2 = cachedFunc(1, 2)
			time.Sleep(time.Millisecond)
		}
	}()
	wg.Wait()

	// Check if the results are the same
	if result1 != result2 {
		t.Errorf("Expected %v, got %v", result1, result2)
	}

	if calls != 1 {
		t.Errorf("Expected function to be called once, but it was called %d times", calls)
	}
}

// Test: Cache is thread-safe
func TestCachedFunctionThreadSafetyWithSleep(t *testing.T) {
	// mock timers
	CacheExpiryTime = 100 * time.Second
	CacheExpirySleepTime = 100 * time.Second
	// mock cache
	MaxCacheSize = 1000
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cached = NewFunctionCache(ctx)

	// Define a simple function to be cached
	f := func(args ...interface{}) interface{} {
		log.Printf("Test function: Sleeping for 100ms\n")
		time.Sleep(100 * time.Millisecond) // Simulate some processing time
		return args[0].(int) + args[1].(int)
	}

	// Create a cached version of the function
	cachedFunc := NewCachedFunction(f)

	var wg sync.WaitGroup

	// Call the cached function concurrently with different arguments
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			cachedFunc(1, 2)
		}(i)
	}
	wg.Wait()
}

// Benchmark: Direct function execution
func BenchmarkDirectFunctionExecution(b *testing.B) {
	// mock timers
	CacheExpiryTime = 100 * time.Second
	CacheExpirySleepTime = 100 * time.Second
	// mock cache
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cached = NewFunctionCache(ctx)

	// Define a simple function to be benchmarked
	f := func(args ...interface{}) interface{} {
		return args[0].(int) + args[1].(int)
	}

	// Benchmark the direct function execution
	for i := 0; i < b.N; i++ {
		f(i, i+1)
	}
}

// Benchmark: Cached function execution
func BenchmarkCachedFunctionExecution(b *testing.B) {
	// mock timers
	CacheExpiryTime = 100 * time.Second
	CacheExpirySleepTime = 100 * time.Second
	// mock cache
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cached = NewFunctionCache(ctx)

	// Define a simple function to be cached
	f := func(args ...interface{}) interface{} {
		return args[0].(int) + args[1].(int)
	}

	// Create a cached version of the function
	cachedFunc := NewCachedFunction(f)

	// Benchmark the cached function execution
	for i := 0; i < b.N; i++ {
		cachedFunc(i, i+1)
	}
}

// Benchmark: high parallelity
func BenchmarkCachedFunctionExecutionHighParallelism(b *testing.B) {
	// mock timers
	CacheExpiryTime = 100 * time.Second
	CacheExpirySleepTime = 100 * time.Second
	// mock cache
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cached = NewFunctionCache(ctx)

	// Define a simple function to be cached
	f := func(args ...interface{}) interface{} {
		return args[0].(int) + args[1].(int)
	}
	// Create a cached version of the function
	cachedFunc := NewCachedFunction(f)
	// Benchmark the cached function execution
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			cachedFunc(1, 2)
		}
	})
}
