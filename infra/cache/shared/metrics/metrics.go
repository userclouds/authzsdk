package metrics

import (
	"context"
	"sync"
	"time"

	"userclouds.com/infra/ucerr"
)

type contextKey int

const (
	ctxCacheMetrics contextKey = 1
)

var cacheMetricsLock sync.RWMutex

// CacheMetrics keeps track of cache calls during a request
// We don't take a lock to access these fields as we perform read only once per request at the end, effectively single threaded
type CacheMetrics struct {
	Calls             int
	Hits              int
	Misses            int
	Deletions         int
	GetLatencies      time.Duration
	StoreLatencies    time.Duration
	DeletionLatencies time.Duration
}

// GetTotalDuration returns the sum of the time spend calling the cache
func (m *CacheMetrics) GetTotalDuration() time.Duration {
	// We don't take a lock as we perform read only once per request at the end, effectively single threaded

	return m.GetLatencies + m.StoreLatencies + m.DeletionLatencies
}

// HadCalls returns true if there were any cache calls
func (m *CacheMetrics) HadCalls() bool {
	// We don't take a lock as we perform read only once per request at the end, effectively single threaded

	return m.Calls > 0
}

// GetMetrics returns the cache metrics structure from the context, errors out if it is not there.
func GetMetrics(ctx context.Context) (*CacheMetrics, error) {
	// We don't take a lock as we either perform read only once per request at the end, effectively single threaded, or we take a write lock before
	// modifying the data
	val := ctx.Value(ctxCacheMetrics)
	metrics, ok := val.(*CacheMetrics)
	if !ok {
		return nil, ucerr.Errorf("Can't find cache metric data in context")
	}
	return metrics, nil
}

// ResetContext resets/adds a cache metrics struct to the context to allow keeping track of cache calls during a request
func ResetContext(ctx context.Context) context.Context {
	return context.WithValue(ctx, ctxCacheMetrics, &CacheMetrics{})
}

// InitContext adds a cache metrics struct to the context to allow keeping track of cache calls during a request
func InitContext(ctx context.Context) context.Context {
	// We don't take a lock as we always perform only once per request at start, effectively single threaded. If that pattern changes, we need to take a lock
	val := ctx.Value(ctxCacheMetrics)

	if _, ok := val.(*CacheMetrics); !ok {
		return context.WithValue(ctx, ctxCacheMetrics, &CacheMetrics{})

	}
	return ctx
}

// RecordCacheHit records a cache hit
func RecordCacheHit(ctx context.Context, duration time.Duration) {
	metricsData, err := GetMetrics(ctx)
	if err != nil {
		return
	}
	// We take a lock as we may have parallelism in the request
	cacheMetricsLock.Lock()
	defer cacheMetricsLock.Unlock()

	metricsData.Calls++
	metricsData.Hits++
	metricsData.GetLatencies += duration
}

// RecordCacheMiss records a cache miss
func RecordCacheMiss(ctx context.Context, duration time.Duration) {
	metricsData, err := GetMetrics(ctx)
	if err != nil {
		return
	}

	// We take a lock as we may have parallelism in the request
	cacheMetricsLock.Lock()
	defer cacheMetricsLock.Unlock()

	metricsData.Calls++
	metricsData.Misses++
	metricsData.GetLatencies += duration
}

// RecordMultiGet records getting multiple objects from the cache
func RecordMultiGet(ctx context.Context, hits, misses int, duration time.Duration) {
	metricsData, err := GetMetrics(ctx)
	if err != nil {
		return
	}

	// We take a lock as we may have parallelism in the request
	cacheMetricsLock.Lock()
	defer cacheMetricsLock.Unlock()

	metricsData.Calls++
	metricsData.Hits += hits
	metricsData.Misses += misses
	metricsData.GetLatencies += duration
}

// RecordCacheStore records store data in the cache
func RecordCacheStore(ctx context.Context, start time.Time) {
	metricsData, err := GetMetrics(ctx)
	if err != nil {
		return
	}

	// We take a lock as we may have parallelism in the request
	cacheMetricsLock.Lock()
	defer cacheMetricsLock.Unlock()

	metricsData.Calls++
	metricsData.StoreLatencies += time.Now().UTC().Sub(start)
}

// RecordCacheDelete records a deletion from the cache
func RecordCacheDelete(ctx context.Context, start time.Time) {
	metricsData, err := GetMetrics(ctx)
	if err != nil {
		return
	}

	// We take a lock as we may have parallelism in the request
	cacheMetricsLock.Lock()
	defer cacheMetricsLock.Unlock()

	metricsData.Calls++
	metricsData.Deletions++

	metricsData.DeletionLatencies += time.Now().UTC().Sub(start)
}
