package client

import (
	"context"
	"sync"
	"time"

	"github.com/patrickmn/go-cache"

	"userclouds.com/infra/cache/shared"
	"userclouds.com/infra/ucerr"
	"userclouds.com/infra/uclog"
)

const (
	// DefaultCacheTTL specifies the default item TTL for in memory cache
	DefaultCacheTTL time.Duration = 30 * time.Second

	defaultCacheTTL time.Duration = 5 * time.Minute
	gcInterval      time.Duration = 5 * time.Minute
)

const memSentinelTTL = 70 * time.Second

// InMemoryClientCacheProvider is the base implementation of the CacheProvider interface
type InMemoryClientCacheProvider struct {
	cache *cache.Cache

	keysMutex sync.Mutex

	sm *shared.WriteThroughCacheSentinelManager
}

// NewInMemoryClientCacheProvider creates a new InMemoryClientCacheProvider
func NewInMemoryClientCacheProvider() *InMemoryClientCacheProvider {
	// TODO - Underlying library treats 0 and -1 as no expiration, so we need to set it to minimum value and not look up items from the cache, work around that
	cacheEdges := cache.New(DefaultCacheTTL, gcInterval)

	return &InMemoryClientCacheProvider{cache: cacheEdges, sm: shared.NewWriteThroughCacheSentinelManager()}
}

// inMemMultiSet sets all passed in keys to the same value with given TTL. Locking is left up to caller
func (c *InMemoryClientCacheProvider) inMemMultiSet(keys []string, value string, ifNotExists bool, ttl time.Duration) bool {
	r := true
	for _, key := range keys {
		if ifNotExists {
			if _, found := c.cache.Get(key); found {
				r = false
				continue
			}
		}
		c.cache.Set(key, value, ttl)
	}
	return r
}

// inMemMultiGet gets all passed in keys. Locking is left up to caller
func (c *InMemoryClientCacheProvider) inMemMultiGet(keys []string) []string {
	values := make([]string, len(keys))
	for i, key := range keys {
		if x, found := c.cache.Get(key); found {
			if val, ok := x.(string); ok {
				values[i] = val
			} else {
				values[i] = ""
			}
		}
	}
	return values
}

// imMemMultiDelete deletes all passed in keys. Locking is left up to caller
func (c *InMemoryClientCacheProvider) inMemMultiDelete(keys []string) {
	for _, key := range keys {
		c.cache.Delete(key)
	}
}

// WriteSentinel writes the sentinel value into the given keys
func (c *InMemoryClientCacheProvider) WriteSentinel(ctx context.Context, stype shared.SentinelType, keysIn []CacheKey) (shared.CacheSentinel, error) {
	sentinel := c.sm.GenerateSentinel(stype)
	keys := make([]string, len(keysIn))
	for i, k := range keysIn {
		if k == "" {
			continue
		}
		keys[i] = string(k)
	}

	if len(keys) == 0 {
		return shared.NoLockSentinel, ucerr.New("Expected at least one key passed to WriteSentinel")
	}

	c.keysMutex.Lock()
	defer c.keysMutex.Unlock()

	// If we are doing a delete we can always take a lock (even in case of other deletes)
	if !c.sm.IsDeleteSentinelPrefix(sentinel) {
		// Check if the primary key for the operation is already locked
		if x, found := c.cache.Get(keys[0]); found {
			if value, ok := x.(string); ok {
				// If the key is already locked and see if we have precedence
				if c.sm.IsSentinelValue(value) {
					if !c.sm.CanSetSentinel(shared.CacheSentinel(value), sentinel) {
						return shared.NoLockSentinel, nil
					}
				}
			}
		}
	}
	// If key is not found, doesn't have a lock, or our lock has precedence we can set it
	c.inMemMultiSet(keys, string(sentinel), false, memSentinelTTL)

	return sentinel, nil
}

// ReleaseSentinel clears the sentinel value from the given keys
func (c *InMemoryClientCacheProvider) ReleaseSentinel(ctx context.Context, keysIn []CacheKey, s shared.CacheSentinel) {
	keys := make([]string, len(keysIn))
	for i, k := range keysIn {
		if k == "" {
			continue
		}
		keys[i] = string(k)
	}

	if len(keys) == 0 {
		return
	}

	c.keysMutex.Lock()
	defer c.keysMutex.Unlock()

	values := c.inMemMultiGet(keys)

	keysToClear := make([]string, 0, len(keys))
	for i, v := range values {
		if v != "" && v == string(s) {
			keysToClear = append(keysToClear, keys[i])
		}
	}
	if len(keysToClear) > 0 {
		c.inMemMultiDelete(keysToClear)
	}

}

// SetValue sets the value in cache key(s) to val with given expiration time if the sentinel matches and returns true if the value was set
func (c *InMemoryClientCacheProvider) SetValue(ctx context.Context, lkeyIn CacheKey, keysToSet []CacheKey, val string, sentinel shared.CacheSentinel, ttl time.Duration) (bool, bool, error) {
	keys := make([]string, len(keysToSet))
	for i, k := range keysToSet {
		if k == "" {
			continue
		}
		keys[i] = string(k)
	}

	lkey := string(lkeyIn)
	if len(keys) == 0 {
		return false, false, ucerr.New("Expected at least one key passed to SetValue")
	}

	c.keysMutex.Lock()
	defer c.keysMutex.Unlock()

	if x, found := c.cache.Get(lkey); found {
		if cV, ok := x.(string); ok {
			set, clear, conflict := c.sm.CanSetValue(cV, val, sentinel)
			if set {
				// The sentinel is still in the key which means nothing interrupted the operation and value can be safely stored in the cache
				uclog.Verbosef(ctx, "Cache set key %v", keys)
				c.inMemMultiSet(keys, val, false, ttl)
				return true, false, nil
			} else if clear {
				uclog.Verbosef(ctx, "Cache cleared on value mismatch or conflict sentinal key %v curr val %v would store %v", keys, cV, val)
				c.inMemMultiDelete(keys)
				return false, false, nil
			} else if conflict {
				c.inMemMultiSet(keys, cV+string(sentinel), false, memSentinelTTL)
				uclog.Verbosef(ctx, "Lock upgraded to conflict on write collision %v got %v added %v", lkey, cV, sentinel)
				return false, true, nil
			}
			uclog.Verbosef(ctx, "Cache not set key %v on sentinel mismatch got %v expect %v", lkey, cV, sentinel)
			return false, true, nil
		}
	}
	uclog.Verbosef(ctx, "Cache not set key %v on sentinel %v key not found", lkey, sentinel)
	return false, false, nil
}

// GetValue gets the value in CacheKey (if any) and tries to lock the key for Read is lockOnMiss = true
func (c *InMemoryClientCacheProvider) GetValue(ctx context.Context, keyIn CacheKey, lockOnMiss bool) (*string, shared.CacheSentinel, error) {
	key := string(keyIn)

	c.keysMutex.Lock()
	defer c.keysMutex.Unlock()

	x, found := c.cache.Get(key)

	if !found {
		if lockOnMiss {
			sentinel := c.sm.GenerateSentinel(shared.Read)
			if r := c.inMemMultiSet([]string{key}, string(sentinel), true, memSentinelTTL); r {
				uclog.Verbosef(ctx, "Cache miss key %v sentinel set %v", key, sentinel)
				return nil, shared.CacheSentinel(sentinel), nil
			}
		}
		uclog.Verbosef(ctx, "Cache miss key %v no lock requested", key)
		return nil, shared.NoLockSentinel, nil
	}

	if value, ok := x.(string); ok {
		if c.sm.IsSentinelValue(value) {
			uclog.Verbosef(ctx, "Cache key %v is locked for in progress op %v", key, value)
			return nil, shared.NoLockSentinel, nil
		}

		uclog.Verbosef(ctx, "Cache hit key %v", key)
		return &value, shared.NoLockSentinel, nil
	}

	return nil, shared.NoLockSentinel, nil
}

// DeleteValue deletes the value(s) in passed in keys
func (c *InMemoryClientCacheProvider) DeleteValue(ctx context.Context, keysIn []CacheKey, force bool) error {
	keys := make([]string, len(keysIn))
	for i, k := range keysIn {
		if k == "" {
			continue
		}
		keys[i] = string(k)
	}

	c.keysMutex.Lock()
	defer c.keysMutex.Unlock()

	if force {
		// Delete regardless of value
		c.inMemMultiDelete(keys)
	} else {
		// Delete only unlocked keys
		for _, k := range keys {
			if x, found := c.cache.Get(k); found {
				if v, ok := x.(string); ok {
					if c.sm.IsSentinelValue(v) || v == string(shared.TombstoneSentinel) {
						// Skip locked key
						continue
					}
				}
				c.cache.Delete(k)
			}
		}
	}
	return nil
}

func (c *InMemoryClientCacheProvider) saveKeyArray(dkeys []string, newKeys []string, ttl time.Duration) error {

	for _, dkey := range dkeys {
		var keyToSet []string
		keyToSet = append(keyToSet, newKeys...)
		if x, found := c.cache.Get(dkey); found {
			keyNames, ok := x.([]string)
			if ok {
				for _, keyName := range keyNames {
					if _, found := c.cache.Get(keyName); found {
						keyToSet = append(keyToSet, keyName)
					}
				}
			} else {
				val, ok := x.(string)
				if !ok || val == string(shared.TombstoneSentinel) {
					return ucerr.New("Can't add dependency: key is tombstoned")
				}
			}
		}
		// Remove duplicates to minimize the length
		keyMap := make(map[string]bool, len(keyToSet))
		uniqueKeysToSet := make([]string, 0, len(keyToSet))
		for _, k := range keyToSet {
			if !keyMap[k] {
				keyMap[k] = true
				uniqueKeysToSet = append(uniqueKeysToSet, k)
			}
		}
		c.cache.Set(dkey, uniqueKeysToSet, ttl)
	}
	return nil
}

func (c *InMemoryClientCacheProvider) deleteKeyArray(dkey string, setTombstone bool) {
	if x, found := c.cache.Get(dkey); found {
		if keyNames, ok := x.([]string); ok {
			c.inMemMultiDelete(keyNames)
		}
	}
	if setTombstone {
		c.cache.Set(dkey, string(shared.TombstoneSentinel), memSentinelTTL)
	} else {
		c.cache.Delete(dkey)
	}
}

// AddDependency adds the given cache key(s) as dependencies of an item represented by by key
func (c *InMemoryClientCacheProvider) AddDependency(ctx context.Context, keysIn []CacheKey, values []CacheKey, ttl time.Duration) error {
	keys := getStringsFromCacheKeys(keysIn)
	i := make([]string, 0, len(values))
	for _, v := range values {
		if v == "" {
			continue
		}
		i = append(i, string(v))
	}

	c.keysMutex.Lock()
	defer c.keysMutex.Unlock()

	return ucerr.Wrap(c.saveKeyArray(keys, i, ttl))
}

// ClearDependencies clears the dependencies of an item represented by key and removes all dependent keys from the cache
func (c *InMemoryClientCacheProvider) ClearDependencies(ctx context.Context, key CacheKey, setTombstone bool) error {
	c.keysMutex.Lock()
	defer c.keysMutex.Unlock()

	c.deleteKeyArray(string(key), setTombstone)
	return nil
}

// Flush flushes the cache (applies only to the tenant for which the client was created)
func (c *InMemoryClientCacheProvider) Flush(ctx context.Context) {
	c.cache.Flush()
}
