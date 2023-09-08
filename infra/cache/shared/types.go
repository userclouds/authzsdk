package shared

// CacheKey is the type storing the cache key name. It is a string but is a separate type to avoid bugs related to mixing up strings.
type CacheKey string

// CacheSentinel is the type storing in the cache marker for in progress operation
type CacheSentinel string

// SentinelType captures the type of the sentinel for different operations
type SentinelType string
