package shared

import (
	"strings"

	"github.com/gofrs/uuid"
)

// NoLockSentinel represents the sentinel value for no lock
const NoLockSentinel CacheSentinel = ""

// SentinelType names
const (
	Create SentinelType = "create"
	Update SentinelType = "update"
	Delete SentinelType = "delete"
	Read   SentinelType = "read"
)

// TombstoneSentinel represents the sentinel value for a tombstone
const TombstoneSentinel CacheSentinel = "Tombstone"

const sentinelPrefix = "sentinel_"
const sentinelWritePrefix = "write_"
const sentinelDeletePrefix = "delete_"
const sentinelReadPrefix = "read_"

// WriteThroughCacheSentinelManager is the implementation of sentinel management for a write through cache
type WriteThroughCacheSentinelManager struct {
}

// NewWriteThroughCacheSentinelManager creates a new BaseCacheSentinelManager
func NewWriteThroughCacheSentinelManager() *WriteThroughCacheSentinelManager {
	return &WriteThroughCacheSentinelManager{}
}

// GenerateSentinel generates a sentinel value for the given sentinel type
func (c *WriteThroughCacheSentinelManager) GenerateSentinel(stype SentinelType) CacheSentinel {
	id := uuid.Must(uuid.NewV4()).String()
	switch stype {
	case Read:
		return CacheSentinel(readSentinelPrefix() + id)
	case Create, Update:
		return CacheSentinel(writeSentinelPrefix() + id)
	case Delete:
		return CacheSentinel(deleteSentinelPrefix() + id)

	}
	return NoLockSentinel
}

// CanSetSentinel returns true if new sentinel can be set for the given current sentinel
func (c *WriteThroughCacheSentinelManager) CanSetSentinel(currVal CacheSentinel, newVal CacheSentinel) bool {
	// If we are doing a read - read sentinel loses to all other sentinels including other in progress reads
	if c.IsReadSentinelPrefix(newVal) {
		return false
	}
	// If there is delete in progress, writes can't take a lock and delete don't need to
	if c.IsDeleteSentinelPrefix(currVal) {
		return false
	}
	// If there is a write in progress, take the lock from it and depend on clean up on value conflict in SetValue
	// This means that if we finish before an earlier write(s), we will write value into the cache, the writes finishing after us
	// will check the value and clear it if it doesn't match what they got from server. If the earlier write(s) finish before us, they will
	// bump our lock to conflict so we will not write the value into the cache but will clear the lock

	return true
}

// CanSetValue returns operation to take given existing key value, new value, and sentinel for the operation
func (c *WriteThroughCacheSentinelManager) CanSetValue(currVal string, val string, sentinel CacheSentinel) (set bool, clear bool, conflict bool) {
	if currVal == string(sentinel) {
		// The sentinel is still in the key which means nothing interrupted the operation and value can be safely stored in the cache
		return true, false, false
	} else if c.IsWriteSentinelPrefix(sentinel) {
		// We are doing a write of an item and we are interleaved with other write(s)
		if !c.IsSentinelValue(currVal) && val != currVal {
			// There is a value in the cache and it doesn't match what we got from the server, clear the cache because we had interleaving writes
			// finish before us with a different value. We can't tell what the server side order of completion was
			return false, true, false
		} else if strings.HasPrefix(currVal, string(sentinel)) {
			// Another write that was interleaved with this one and finished first, setting the sentinel to indicate conflict. We can't tell what the server
			// side order of completion was so clear the cache
			return false, true, false
		} else if c.IsWriteSentinelPrefix(CacheSentinel(currVal)) {
			// There is another write in progress that started after us. There is no way to tell if that write will commit same value to the cache
			// so upgrade its lock to conflict so it doesn't commit its result
			return false, false, true
		}
	}
	return false, false, false
}

// IsSentinelValue returns true if the value passed in is a sentinel value
func (c *WriteThroughCacheSentinelManager) IsSentinelValue(v string) bool {
	return strings.HasPrefix(v, sentinelPrefix)
}

// IsReadSentinelPrefix returns true if the sentinel value is a read sentinel
func (c *WriteThroughCacheSentinelManager) IsReadSentinelPrefix(v CacheSentinel) bool {
	return strings.HasPrefix(string(v), readSentinelPrefix())
}

// IsWriteSentinelPrefix returns true if the sentinel value is a write sentinel
func (c *WriteThroughCacheSentinelManager) IsWriteSentinelPrefix(v CacheSentinel) bool {
	return strings.HasPrefix(string(v), writeSentinelPrefix())
}

// IsDeleteSentinelPrefix returns true if the sentinel value is a delete sentinel
func (c *WriteThroughCacheSentinelManager) IsDeleteSentinelPrefix(v CacheSentinel) bool {
	return strings.HasPrefix(string(v), deleteSentinelPrefix())
}

func deleteSentinelPrefix() string {
	return sentinelPrefix + sentinelDeletePrefix
}

func writeSentinelPrefix() string {
	return sentinelPrefix + sentinelWritePrefix
}

func readSentinelPrefix() string {
	return sentinelPrefix + sentinelReadPrefix
}
