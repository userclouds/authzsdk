package metrics

import (
	"context"
	"time"

	"userclouds.com/infra/ucerr"
)

type contextKey int

const (
	ctxCacheMetrics contextKey = 1
)

// CacheMetrics keeps track of DB calls during a request
type CacheMetrics struct {
	Calls             int
	Hits              int
	Misses            int
	Deletions         int
	GetLatencies      time.Duration
	StoreLatencies    time.Duration
	DeletionLatencies time.Duration
}

// GetTotalDuration returns the sum of the time spend calling the DB
func (m *CacheMetrics) GetTotalDuration() time.Duration {
	return m.GetLatencies + m.StoreLatencies + m.DeletionLatencies
}

// HadCalls returns true if there were any DB calls
func (m *CacheMetrics) HadCalls() bool {
	return m.Calls > 0
}

// GetMetrics returns the DB metrics structure from the context, errors out if it is not there.
func GetMetrics(ctx context.Context) (*CacheMetrics, error) {
	val := ctx.Value(ctxCacheMetrics)
	metrics, ok := val.(*CacheMetrics)
	if !ok {
		return nil, ucerr.Errorf("Can't find cache metric data in context")
	}
	return metrics, nil
}

// InitContext adds a DB metrics struct to the context to allow keeping track of DB calls during a request
func InitContext(ctx context.Context) context.Context {
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
	metricsData.Calls++
	metricsData.StoreLatencies += time.Now().UTC().Sub(start)
}

// RecordCacheDelete records a deletion from the cache
func RecordCacheDelete(ctx context.Context, start time.Time) {

	metricsData, err := GetMetrics(ctx)
	if err != nil {
		return
	}
	metricsData.Calls++
	metricsData.Deletions++

	metricsData.DeletionLatencies += time.Now().UTC().Sub(start)
}
