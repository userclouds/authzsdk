package shared

import "time"

// SentinelType names
const (
	Create SentinelType = "create"
	Update SentinelType = "update"
	Delete SentinelType = "delete"
	Read   SentinelType = "read"
)

const (

	// SentinelTTL value when setting a sentinel value in the cache
	SentinelTTL = 60 * time.Second
	// InvalidationTombstoneTTL value when setting a tombstone value in the cache for cross region invalidation
	InvalidationTombstoneTTL = 5 * time.Second

	// TombstoneSentinel represents the sentinel value for a tombstone
	TombstoneSentinel CacheSentinel = "Tombstone"

	// NoLockSentinel represents the sentinel value for no lock
	NoLockSentinel CacheSentinel = ""
)

// IsTombstoneSentinel returns true if the given data is a tombstone sentinel
func IsTombstoneSentinel(data string) bool {
	return data == string(TombstoneSentinel)
}
