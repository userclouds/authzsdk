package client

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"

	"userclouds.com/infra/cache/shared"
	"userclouds.com/infra/ucerr"
	"userclouds.com/infra/uclog"
)

const (
	// If the cache is accessed by a number of clients (across all machines) above this value performing create/update/delete operations on same
	// keys, the operation may fail for some of them due to optimistic locking not retrying enough times.
	maxRdConflictRetries = 15
	// RegionalRedisCacheName is default name of the regional redis cache
	RegionalRedisCacheName = "redisRegionalCache"
)

// RedisClientCacheProvider is the base implementation of the CacheProvider interface
type RedisClientCacheProvider struct {
	rc        *redis.Client
	prefix    string
	sm        *shared.WriteThroughCacheSentinelManager
	cacheName string
}

// NewRedisClientCacheProvider creates a new RedisClientCacheProvider
func NewRedisClientCacheProvider(rc *redis.Client, prefix string, cacheName string) *RedisClientCacheProvider {
	return &RedisClientCacheProvider{rc: rc, prefix: prefix, sm: shared.NewWriteThroughCacheSentinelManager(), cacheName: cacheName}
}

// WriteSentinel writes the sentinel value into the given keys
func (c *RedisClientCacheProvider) WriteSentinel(ctx context.Context, stype shared.SentinelType, keysIn []shared.CacheKey) (shared.CacheSentinel, error) {
	sentinel := c.sm.GenerateSentinel(stype)
	keys, err := getValidatedStringKeysFromCacheKeys(keysIn, c.prefix)
	if err != nil {
		return shared.NoLockSentinel, ucerr.Wrap(err)
	}
	// There must be at least one key to lock
	if len(keys) == 0 {
		return shared.NoLockSentinel, ucerr.New("WriteSentinel was passed no keys to set")
	}

	lockValue := shared.NoLockSentinel
	// Transactional function to read current value of the key and try to take the lock for this operation depending on the key value
	txf := func(tx *redis.Tx) error {
		// Operation is committed only if the watched keys remain unchanged.
		_, err := tx.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
			lockValue = shared.NoLockSentinel
			if !c.sm.IsDeleteSentinelPrefix(sentinel) {
				// Check if the primary key for the operation is already locked
				value, err := c.rc.Get(ctx, keys[0]).Result()
				if err != nil && err != redis.Nil {
					// If we can't read the key, we can't take a lock
					return ucerr.Wrap(err)
				}
				// If the key is already locked and see if we have precedence
				if err == nil && c.sm.IsSentinelValue(value) {
					if !c.sm.CanSetSentinel(shared.CacheSentinel(value), sentinel) {
						return nil
					}
				}
				// Proceed to take the lock if key is empty (err == redis.Nil) or it doesn't contain sentinel value
			}

			if err := multiSetWithPipe(ctx, pipe, keys, string(sentinel), shared.SentinelTTL); err != nil {
				return ucerr.Wrap(err)
			}
			lockValue = sentinel
			return nil
		})
		return ucerr.Wrap(err)
	}

	// Retry if the key has been changed.
	for i := 0; i < maxRdConflictRetries; i++ {
		err := c.rc.Watch(ctx, txf, keys[0])
		if err == nil {
			// Success.
			return lockValue, nil
		}
		if errors.Is(err, redis.TxFailedErr) {
			// Optimistic lock lost. Retry.
			continue
		}
		// Return any other error.
		return shared.NoLockSentinel, ucerr.Wrap(err)
	}

	uclog.Debugf(ctx, fmt.Sprintf("WriteSentinel - reached maximum number of retries on keys %v skipping cache", keys))
	return shared.NoLockSentinel, ucerr.New("WriteSentinel reached maximum number of retries")
}

// getValidatedStringKeysFromCacheKeys filters out any empty keys and does the type conversion
func getValidatedStringKeysFromCacheKeys(keys []shared.CacheKey, prefix string) ([]string, error) {
	strKeys := make([]string, 0, len(keys))
	for _, k := range keys {
		if k != "" {
			if strings.HasPrefix(string(k), prefix) {
				strKeys = append(strKeys, string(k))
			} else {
				return nil, ucerr.New(fmt.Sprintf("Key %v does not have prefix %v", k, prefix))
			}
		}
	}
	return strKeys, nil
}

func getValidatedStringKeyFromCacheKey(key shared.CacheKey, prefix string) (string, error) {
	if strings.HasPrefix(string(key), prefix) || key == "" {
		return string(key), nil
	}
	return "", ucerr.New(fmt.Sprintf("Key %v does not have prefix %v", key, prefix))
}

// ReleaseSentinel clears the sentinel value from the given keys
func (c *RedisClientCacheProvider) ReleaseSentinel(ctx context.Context, keysIn []shared.CacheKey, s shared.CacheSentinel) {
	// Filter out any empty keys
	keys, err := getValidatedStringKeysFromCacheKeys(keysIn, c.prefix)
	// If there are no keys to potentially clear, return
	if err != nil || len(keys) == 0 {
		return
	}

	// Using optimistic concurrency control to clear the sentinels set by our operation. We need to make sure that no ones else
	// writes to the keys between the read and the delete so that we don't accidentally clear another operations sentinel

	// Transactional function to read current value of keys and delete them only if they contain the sentinel value
	txf := func(tx *redis.Tx) error {
		values, err := c.rc.MGet(ctx, keys...).Result()
		keysToClear := []string{}
		if err == nil {
			keysToClear = make([]string, 0, len(keys))
			for i, v := range values {
				vS, ok := v.(string)
				if ok && vS == string(s) {
					keysToClear = append(keysToClear, keys[i])
				}
			}

		}

		if len(keysToClear) == 0 {
			return nil
		}

		// Operation is committed only if the watched keys remain unchanged.
		_, err = tx.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
			if len(keysToClear) > 0 {
				if err := pipe.Del(ctx, keysToClear...).Err(); err != nil && err != redis.Nil {
					uclog.Errorf(ctx, "Error clearing key(s) %v sentinel  %v", keysToClear, err)
				}
				uclog.Verbosef(ctx, "Cache[%v] cleared key(s) %v sentinel %v", c.cacheName, keysToClear, s)
			}
			return nil
		})
		return ucerr.Wrap(err)
	}

	// Retry if the key has been changed.
	for i := 0; i < maxRdConflictRetries; i++ {
		err := c.rc.Watch(ctx, txf, keys...)
		if err == nil {
			// Success.
			return
		}
		if errors.Is(err, redis.TxFailedErr) {
			// Optimistic lock lost. Retry.
			continue
		}
		// Return any other error.
		uclog.Debugf(ctx, fmt.Sprintf("ReleaseSentinel - failed on keys %v with %v skipping cache. Keys maybe locked until sentinel expires", keys, err))
		return
	}
}

// multiSetWithPipe add commands to set the keys and expiration to given pipe
func multiSetWithPipe(ctx context.Context, pipe redis.Pipeliner, keys []string, value string, ttl time.Duration) error {
	var ifaces = make([]interface{}, 0, len(keys)*2)
	for i := range keys {
		ifaces = append(ifaces, keys[i], value)
	}
	if err := pipe.MSet(ctx, ifaces...).Err(); err != nil {
		return ucerr.Wrap(err)
	}
	for i := range keys {
		pipe.Expire(ctx, keys[i], ttl)
	}
	return nil
}

// SetValue sets the value in cache key(s) to val with given expiration time if the sentinel matches and returns true if the value was set
func (c *RedisClientCacheProvider) SetValue(ctx context.Context, lkeyIn shared.CacheKey, keysToSet []shared.CacheKey, val string,
	sentinel shared.CacheSentinel, ttl time.Duration) (bool, bool, error) {

	keys, err := getValidatedStringKeysFromCacheKeys(keysToSet, c.prefix)
	if err != nil {
		return false, false, ucerr.Wrap(err)
	}
	// There needs to be at least a single key to check for sentinel/set to value
	if len(keys) == 0 {
		return false, false, ucerr.New("No keys provided to SetValue")
	}

	lkey := string(lkeyIn)
	conflictDetected := false
	valueSet := false

	// Transactional function to read value of pkey and perform the corresponding update depending on its value atomically
	txf := func(tx *redis.Tx) error {

		// Operation is committed only if the watched keys remain unchanged.
		_, err := tx.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
			conflictDetected = false
			valueSet = false

			cV, err := c.rc.Get(ctx, lkey).Result()
			// Either key is empty or we couldn't get it
			if err != nil {
				return nil
			}

			set, clear, conflict := c.sm.CanSetValue(cV, val, sentinel)

			if set { // Value can be set
				uclog.Verbosef(ctx, "Cache[%v] set key %v", c.cacheName, keys)
				if err := multiSetWithPipe(ctx, pipe, keys, val, ttl); err != nil {
					return ucerr.Wrap(err)
				}
				valueSet = true
				return nil
			} else if clear { // Intermediate state detected so clear the cache
				uclog.Verbosef(ctx, "Cache[%v] cleared on value mismatch or conflict sentinel key %v curr var %v would store %v", c.cacheName, keys, cV, val)
				if err := pipe.Del(ctx, keys...).Err(); err != nil && err != redis.Nil {
					uclog.Errorf(ctx, "Error clearing key(s) %v mismatch -  %v", keys, err)
				}
				return nil
			} else if conflict { // Conflict detected so upgrade the lock to conflict
				if err := multiSetWithPipe(ctx, pipe, keys, cV+string(sentinel), shared.SentinelTTL); err != nil {
					return ucerr.Wrap(err)
				}
				uclog.Verbosef(ctx, "Cache[%v] lock upgraded to conflict on write collision %v got %v added %v", c.cacheName, lkey, cV, sentinel)
				conflictDetected = true
				return nil
			}

			uclog.Verbosef(ctx, "Cache[%v] not set key %v on sentinel mismatch got %v expect %v", c.cacheName, lkey, cV, sentinel)
			conflictDetected = true
			return nil
		})
		return ucerr.Wrap(err)
	}

	// Retry if the key has been changed.
	for i := 0; i < maxRdConflictRetries; i++ {
		err := c.rc.Watch(ctx, txf, lkey)
		if err == nil {
			// Success.
			return valueSet, conflictDetected, nil
		}
		if errors.Is(err, redis.TxFailedErr) {
			// Optimistic lock lost. Retry.
			continue
		}
		// Return any other error.
		return false, false, ucerr.Wrap(err)
	}
	uclog.Debugf(ctx, fmt.Sprintf("SetValue - hit too many retries %v skipping cache.", keys))
	return false, false, ucerr.New("SetValue hit too many retries")
}

// GetValue gets the value in CacheKey (if any) and tries to lock the key for Read is lockOnMiss = true
func (c *RedisClientCacheProvider) GetValue(ctx context.Context, keyIn shared.CacheKey, lockOnMiss bool) (*string, shared.CacheSentinel, error) {
	key, err := getValidatedStringKeyFromCacheKey(keyIn, c.prefix)
	if err != nil {
		return nil, "", ucerr.Wrap(err)
	}
	if key == "" {
		return nil, "", ucerr.New("Empty key provided to GetValue")
	}

	value, err := c.rc.Get(ctx, key).Result()
	if err == redis.Nil {
		if lockOnMiss {
			sentinel := c.sm.GenerateSentinel(shared.Read)
			// Since SetNX is atomic we don't need to worry about the other operation on key between the Get and SetNX
			r, err := c.rc.SetNX(ctx, key, string(sentinel), shared.SentinelTTL).Result()
			if err != nil {
				uclog.Verbosef(ctx, "Cache[%v] miss key %v lock fail %v", c.cacheName, key, err)
				return nil, "", ucerr.Wrap(err)
			}
			if r {
				uclog.Verbosef(ctx, "Cache[%v] miss key %v sentinel set %v", c.cacheName, key, sentinel)
				return nil, shared.CacheSentinel(sentinel), nil
			}
		}
		uclog.Verbosef(ctx, "Cache[%v] miss key %v no lock requested", c.cacheName, key)
		return nil, shared.NoLockSentinel, nil
	}
	if err != nil {
		return nil, shared.NoLockSentinel, ucerr.Wrap(err)
	}
	if c.sm.IsSentinelValue(value) {
		uclog.Verbosef(ctx, "Cache[%v] key %v is locked for in progress op %v", c.cacheName, key, value)
		return nil, shared.NoLockSentinel, nil
	}

	uclog.Verbosef(ctx, "Cache[%v] hit key %v", c.cacheName, key)
	return &value, shared.NoLockSentinel, nil
}

// DeleteValue deletes the value(s) in passed in keys
func (c *RedisClientCacheProvider) DeleteValue(ctx context.Context, keysIn []shared.CacheKey, setTombstone bool, force bool) error {
	keysAll, err := getValidatedStringKeysFromCacheKeys(keysIn, c.prefix)
	if err != nil {
		return ucerr.Wrap(err)
	}
	if len(keysAll) != 0 {
		if force && !setTombstone {
			return c.rc.Del(ctx, keysAll...).Err()
		}
		batchSize := 2
		var end int
		for start := 0; start < len(keysAll); start += batchSize {
			end += batchSize
			if end > len(keysAll) {
				end = len(keysAll)
			}

			keys := keysAll[start:end]

			// Transactional function to only clear keys if they don't contain sentinel or tombstone
			txf := func(tx *redis.Tx) error {
				// Operation is committed only if the watched keys remain unchanged.
				_, err := tx.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
					if force {
						return ucerr.Wrap(multiSetWithPipe(ctx, pipe, keys, string(shared.TombstoneSentinel), shared.InvalidationTombstoneTTL))
					}
					values, err := c.rc.MGet(ctx, keys...).Result()
					if err != nil && err != redis.Nil {
						return ucerr.Wrap(err)
					}

					keysToDelete := []string{}
					for i, v := range values {
						if vS, ok := v.(string); ok && !c.sm.IsSentinelValue(vS) && !shared.IsTombstoneSentinel(vS) {
							keysToDelete = append(keysToDelete, keys[i])
						}
					}

					if len(keysToDelete) > 0 {
						if setTombstone {
							return ucerr.Wrap(multiSetWithPipe(ctx, pipe, keysToDelete, string(shared.TombstoneSentinel), shared.InvalidationTombstoneTTL))
						}
						if err := pipe.Del(ctx, keysToDelete...).Err(); err != nil {
							return ucerr.Wrap(err)
						}
					}

					return nil
				})
				return ucerr.Wrap(err)
			}

			// Retry if the key has been changed.
			success := false
			for i := 0; i < maxRdConflictRetries; i++ {
				err := c.rc.Watch(ctx, txf, keys...)
				if err == nil {
					// Success.
					success = true
					break
				}
				if errors.Is(err, redis.TxFailedErr) {
					// Optimistic lock lost. Retry.
					continue
				}
				// Return any other error.
				return ucerr.Wrap(err)
			}
			if !success {
				uclog.Warningf(ctx, fmt.Sprintf("Failed delete values - reached maximum number of retries on keys %v", keys))
				return ucerr.New("Failed to DeleteValue reached maximum number of retries")
			}
		}
	}
	return nil
}

// AddDependency adds the given cache key(s) as dependencies of an item represented by by key
func (c *RedisClientCacheProvider) AddDependency(ctx context.Context, keysIn []shared.CacheKey, values []shared.CacheKey, ttl time.Duration) error {
	keysAll, err := getValidatedStringKeysFromCacheKeys(keysIn, c.prefix)
	if err != nil {
		return ucerr.Wrap(err)
	}
	i := make([]interface{}, 0, len(values))
	for _, v := range values {
		if v != "" { // Skip empty values
			i = append(i, string(v))
		}
	}

	if len(keysAll) == 0 {
		return ucerr.New("No key provided to AddDependency")
	}

	if len(keysAll) > 100 {
		return ucerr.New(fmt.Sprintf("Too many keys %v provided to to AddDependency", len(keysAll)))
	}

	if len(i) == 0 {
		return ucerr.New("No non blank values provided to AddDependency")
	}
	// Using optimistic concurrency control to ensure we only add a new dependency if the key is not tombstoned.

	// There is a tradeoff between the number of calls we make to the cache and the probability of collision.
	// The collision is least likely if we update a single dependency least at a time but that would require
	// sequential N calls to the cache. The probability of collision is highest if we update all dependencies at once, but that
	// may lead to a lot of retries and possibly failure under high contention.

	batchSize := 2
	var end int
	for start := 0; start < len(keysAll); start += batchSize {
		end += batchSize
		if end > len(keysAll) {
			end = len(keysAll)
		}

		keys := keysAll[start:end]

		// Transactional function to check if key is not tombstoned and add the dependency.
		txf := func(tx *redis.Tx) error {
			// Operation is committed only if the watched keys remain unchanged.
			_, err := tx.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
				values, err := c.rc.MGet(ctx, keys...).Result()
				if err != nil && err != redis.Nil {
					return ucerr.Wrap(err)
				}

				for _, v := range values {
					vS, ok := v.(string)
					if ok && shared.IsTombstoneSentinel(vS) {
						return ucerr.New("Can't add dependency: key is tombstoned")
					}
				}

				for _, key := range keys {
					if err := pipe.SAdd(ctx, key, i...).Err(); err != nil {
						return ucerr.Wrap(err)
					}
					// Bump expiration which mean that the expired member accumulate in the set and we need to clean them up. Using ZSET sorted by timestamps may be a better option
					if err := pipe.Expire(ctx, key, ttl).Err(); err != nil {
						return ucerr.Wrap(err)
					}
				}
				return nil
			})
			return ucerr.Wrap(err)
		}

		// Retry if the key has been changed.
		success := false
		for i := 0; i < maxRdConflictRetries; i++ {
			err := c.rc.Watch(ctx, txf, keys...)
			if err == nil {
				// Success.
				success = true
				break
			}
			if errors.Is(err, redis.TxFailedErr) {
				// Optimistic lock lost. Retry.
				continue
			}
			// Return any other error.
			return ucerr.Wrap(err)
		}
		if !success {
			uclog.Warningf(ctx, fmt.Sprintf("Failed to add dependencies - reached maximum number of retries on keys %v", keys))
			return ucerr.New("Add dependencies reached maximum number of retries")
		}
	}

	return nil
}

// ClearDependencies clears the dependencies of an item represented by key and removes all dependent keys from the cache
func (c *RedisClientCacheProvider) ClearDependencies(ctx context.Context, keyIn shared.CacheKey, setTombstone bool) error {
	key, err := getValidatedStringKeyFromCacheKey(keyIn, c.prefix)
	if err != nil {
		return ucerr.Wrap(err)
	}

	// Using optimistic concurrency control to clear the dependent keys for each value in key. This may cause us to flush more keys than needed but
	// never miss one. We tombstone the key to prevent new dependencies from being added from reads that might have been in flight during deletion.

	// Transactional function to read list of dependent keys and delete them
	txf := func(tx *redis.Tx) error {
		// Operation is committed only if the watched keys remain unchanged.
		_, err := tx.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
			keys := []string{}
			var isTombstone bool
			if m, err := tx.SMembersMap(ctx, string(key)).Result(); err == nil {
				keys = make([]string, 0, len(m))
				for k := range m {
					keys = append(keys, k)
				}
			} else if v, err := c.rc.Get(ctx, key).Result(); err == nil && shared.IsTombstoneSentinel(v) {
				isTombstone = true
			}

			if len(keys) != 0 {
				if err := pipe.Del(ctx, keys...).Err(); err != nil && err != redis.Nil {
					return ucerr.Wrap(err)
				}
				uclog.Verbosef(ctx, "Cache[%v] cleared dependencies %v keys", c.cacheName, keys)
			}
			if setTombstone {
				if err := pipe.Set(ctx, key, string(shared.TombstoneSentinel), shared.SentinelTTL).Err(); err != nil {
					return ucerr.Wrap(err)
				}
				uclog.Verbosef(ctx, "Cache[%v] set tombstone for %v", c.cacheName, key)
			} else if !isTombstone {
				if err := pipe.Del(ctx, key).Err(); err != nil && err != redis.Nil {
					return ucerr.Wrap(err)
				}
			}
			return nil
		})
		return ucerr.Wrap(err)
	}

	// Retry if the key has been changed.
	for i := 0; i < maxRdConflictRetries; i++ {
		err := c.rc.Watch(ctx, txf, key)
		if err == nil {
			// Success.
			return nil
		}
		if errors.Is(err, redis.TxFailedErr) {
			// Optimistic lock lost. Retry.
			continue
		}
		// Return any other error.
		return ucerr.Wrap(err)
	}
	uclog.Warningf(ctx, fmt.Sprintf("Failed to clear dependencies - reached maximum number of retries on keys %v", key))
	return ucerr.New("Clear dependencies reached maximum number of retries")
}

// Flush flushes the cache (applies only to the tenant for which the client was created)
func (c *RedisClientCacheProvider) Flush(ctx context.Context, prefix string) error {
	pipe := c.rc.Pipeline()
	iter := c.rc.Scan(ctx, 0, prefix+"*", 0).Iterator()
	for iter.Next(ctx) {
		pipe.Del(ctx, iter.Val())
	}
	if iter.Err() != nil {
		return ucerr.Wrap(iter.Err())
	}
	if _, err := pipe.Exec(ctx); err != nil {
		return ucerr.Wrap(err)
	}
	return nil
}

// GetCacheName returns the name of the cache
func (c *RedisClientCacheProvider) GetCacheName(ctx context.Context) string {
	return c.cacheName
}
