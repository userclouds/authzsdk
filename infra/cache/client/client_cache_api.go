package client

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/gofrs/uuid"

	"userclouds.com/infra/cache/shared"
	"userclouds.com/infra/cache/shared/metrics"
	"userclouds.com/infra/jsonclient"
	"userclouds.com/infra/ucerr"
	"userclouds.com/infra/uclog"
)

// The client cache API is designed to support a consistent, write-through (i.e. values on create/update are written to the cache and a cache client is guaranteed
// read after write consistency). Each item in the cache is stored under its primary key (i.e. itemID -> itemValue). All operations are supported on the primary key.
// The consistency is guaranteed via an optimistic locking mechanism implemented using a sentinel value. The sentinel value is stored in the cache under the key at
// the start of the operation and is checked at the end of the operation. If the sentinel value matches, the result operation of the operation is stored in the cache.
// If the sentinel value does not match (ie the item was modified by another operation), different actions are taken depending on the new value of the primary key.
// The sentinel rules are described in cache_writethrough_sentinel_manager.go.
//
// Every item type that doesn't flush entire cache on create/update/delete also has a dependency list key. The dependency key contains a list of all keys that are in
// the cache that need to be invalidated if the item value changes. On post invalidation the dependency list is set to a tombstone value. The tombstone value blocks
// addition of new dependencies to the list. Failure to update the dependency list for one of the dependencies caused the value to not be stored in the cache. This is
// used to invalidate in flight reads that don't take a shared lock (like the primary key) with the update/delete/create operation. The tombstone expiration time is set
// to longest time a server side operation can take.

//
// The cache is also designed to support secondary keys for an item (ie. itemAlias -> itemID). For simplicity, the secondary keys are only used for Read operations
// (ie you can't update/delete an item using a secondary key). While the the items are always stored under their primary key and secondary key(s) (ie the item is
// never stored under the secondary key alone), we can't guarantee that the primary key will always expire at same time as secondary key. We need to invalidate inflight
// reads via secondary keys (ie GetItemByName()), during delete operations via a primary key (our delete operation doesn't return the item that has been deleted so secondary key
// can't be calculated) and also invalidate value stored under secondary key prior to start of the delete. We handle it in two different ways depending on type of the item,
// depending on if the item has a dependency list. Items that don't have a dependency key this via a per type global collection key that is locked by every create/delete/update
// operation and by every read operation. The is most efficient for low change volume types like EdgeType and ObjectType. Items that have a dependency list,
// handle it by adding secondary key(s) value(s) to its own dependency list. This requires extra write/read from the dependency list but is more efficient for high change volume
// item types like Edge and Object.
//
// The cache support one global collection per item type. This collection is meant to contain every item of given type. It makes sense for items with low change volume
// and is invalidate on any create/delete/update operation to any item of that type. This is done by locking per type global collection key on every create/delete/update
// operation and by every read operation (read lock).

// The cache supports any number of per item collection (ie. []Edges on Object). We use that functionality to store paths between two objects as well as edges.

// CacheSingleItem is an interface for any single non array item that can be stored in the cache
// This interface also links the type (ObjectType, EdgeType, Object, Edge) with the cache key names for each type of use
type CacheSingleItem interface {
	// GetPrimaryKey returns the primary cache key where the item is stored and which is used to lock the item
	GetPrimaryKey(c CacheKeyNameProvider) shared.CacheKey
	// GetSecondaryKeys returns any secondary keys which also contain the item for lookup by another dimension (ie TypeName, Alias, etc)
	GetSecondaryKeys(c CacheKeyNameProvider) []shared.CacheKey
	// GetGlobalCollectionKey returns the key for the collection of all items of this type (ie all ObjectTypes, all EdgeTypes, etc)
	GetGlobalCollectionKey(c CacheKeyNameProvider) shared.CacheKey
	// GetPerItemCollectionKey returns the key for the collection of per item items of another type (ie Edges in/out of a specific Object)
	GetPerItemCollectionKey(c CacheKeyNameProvider) shared.CacheKey
	// GetDependenciesKey returns the key containing dependent keys that should invalidated if the item is invalidated
	GetDependenciesKey(c CacheKeyNameProvider) shared.CacheKey
	// GetDependencyKeys returns the list of keys for items this item depends on (ie Edge depends on both source and target objects)
	GetDependencyKeys(c CacheKeyNameProvider) []shared.CacheKey
	// TTL returns the TTL for the item
	TTL(c CacheTTLProvider) time.Duration
}

// CacheProvider is the interface for the cache backend for a given tenant which can be implemented by in-memory, redis, memcache, etc
type CacheProvider interface {
	// GetValue gets the value in cache key (if any) and tries to lock the key for Read is lockOnMiss = true
	GetValue(ctx context.Context, key shared.CacheKey, lockOnMiss bool) (*string, shared.CacheSentinel, error)
	// SetValue sets the value in cache key(s) to val with given expiration time if the sentinel matches lkey and returns true if the value was set
	SetValue(ctx context.Context, lkey shared.CacheKey, keysToSet []shared.CacheKey, val string, sentinel shared.CacheSentinel, ttl time.Duration) (bool, bool, error)
	// DeleteValue deletes the value(s) in passed in keys, force is true also deletes keys with sentinel or tombstone values
	DeleteValue(ctx context.Context, key []shared.CacheKey, setTombstone bool, force bool) error
	// WriteSentinel writes the sentinel value into the given keys, returns NoLockSentinel if it couldn't acquire the lock
	WriteSentinel(ctx context.Context, stype shared.SentinelType, keys []shared.CacheKey) (shared.CacheSentinel, error)
	// ReleaseSentinel clears the sentinel value from the given keys
	ReleaseSentinel(ctx context.Context, keys []shared.CacheKey, s shared.CacheSentinel)
	// AddDependency adds the given cache key(s) as dependencies of an item represented by by key. Fails if any of the dependency keys passed in contain tombstone
	AddDependency(ctx context.Context, keysIn []shared.CacheKey, dependentKey []shared.CacheKey, ttl time.Duration) error
	// ClearDependencies clears the dependencies of an item represented by key and removes all dependent keys from the cache
	ClearDependencies(ctx context.Context, key shared.CacheKey, setTombstone bool) error
	// Flush flushes the cache
	Flush(ctx context.Context)
}

// CacheManager is the bundle cache classes that are needed to interact with the cache
type CacheManager struct {
	P CacheProvider
	N CacheKeyNameProvider
	T CacheTTLProvider
}

// NewCacheManager returns a new CacheManager with given contents
func NewCacheManager(p CacheProvider, n CacheKeyNameProvider, t CacheTTLProvider) CacheManager {
	return CacheManager{P: p, N: n, T: t}
}

// CacheKeyTTLID is the type for the ID used to identify the cache key TTL via CacheTTLProvider interface
type CacheKeyTTLID string

// CacheTTLProvider is the interface for the container that can provide per item cache TTLs
type CacheTTLProvider interface {
	TTL(id CacheKeyTTLID) time.Duration
}

// SkipCacheTTL is TTL set when cache is not used
const SkipCacheTTL time.Duration = 0

// CacheKeyNameID is the type for the ID used to identify the cache key name via CacheKeyNameProvider interface
type CacheKeyNameID string

// CacheKeyNameProvider is the interface for the container that can provide cache names for cache keys that
// can be shared across different cache providers
type CacheKeyNameProvider interface {
	GetKeyName(id CacheKeyNameID, components []string) shared.CacheKey
	// GetKeyNameWithID is a wrapper around GetKeyName that converts the itemID to []string
	GetKeyNameWithID(id CacheKeyNameID, itemID uuid.UUID) shared.CacheKey
	// GetKeyNameWithString is a wrapper around GetKeyName that converts the itemName to []string
	GetKeyNameWithString(id CacheKeyNameID, itemName string) shared.CacheKey
	// GetKeyNameStatic is a wrapper around GetKeyName that passing in empty []string
	GetKeyNameStatic(id CacheKeyNameID) shared.CacheKey
}

// getItemLockKeys returns the keys to lock for the given item
func getItemLockKeys[item CacheSingleItem](ctx context.Context, lockType shared.SentinelType, c CacheKeyNameProvider, i item) []shared.CacheKey {
	keys := []shared.CacheKey{i.GetPrimaryKey(c)} // primary key is always first
	switch lockType {
	case shared.Create:
		// Takes a lock if item does not exist, if read lock is in place
		// If write lock is in place, replaces it with new write lock
		if i.GetGlobalCollectionKey(c) != "" {
			keys = append(keys, i.GetGlobalCollectionKey(c))
		}
		keys = append(keys, i.GetSecondaryKeys(c)...)
	case shared.Update:
		// Takes a write lock if item does not exist or if read lock is in place
		// Do not take a lock if a conflict or delete lock is in place
		// If write lock is in place, upgrade it to conflict lock
		if i.GetGlobalCollectionKey(c) != "" {
			keys = append(keys, i.GetGlobalCollectionKey(c))
		}
		keys = append(keys, i.GetSecondaryKeys(c)...)
	case shared.Delete:
		// Takes all locks regardless of key state
		if i.GetGlobalCollectionKey(c) != "" {
			keys = append(keys, i.GetGlobalCollectionKey(c))
		}
		keys = append(keys, i.GetSecondaryKeys(c)...)
		if i.GetPerItemCollectionKey(c) != "" {
			keys = append(keys, i.GetPerItemCollectionKey(c))
		}
	case shared.Read:
		// Only takes a read lock if the primary key is not set
	}
	return keys
}

// TakeItemLock takes a lock for the given item. Typically used for Create, Update, Delete operations on an item
func TakeItemLock[item CacheSingleItem](ctx context.Context, lockType shared.SentinelType, c CacheManager, i item) (shared.CacheSentinel, error) {
	return takeLockWorker(ctx, c, lockType, i, getItemLockKeys[item](ctx, lockType, c.N, i))
}

// TakePerItemCollectionLock takes a lock for the collection associated with a given item
func TakePerItemCollectionLock[item CacheSingleItem](ctx context.Context, lockType shared.SentinelType, c CacheManager, additionalColKeys []shared.CacheKey, i item) (shared.CacheSentinel, error) {
	if lockType != shared.Delete && lockType != shared.Read {
		return shared.NoLockSentinel, ucerr.New("Unexpected lock type for collection lock")
	}

	// Lock the primary per item collection and any sub collections that are passed in
	keys := []shared.CacheKey{i.GetPerItemCollectionKey(c.N)}
	keys = append(keys, additionalColKeys...)

	return takeLockWorker(ctx, c, lockType, i, keys)
}

func takeLockWorker[item CacheSingleItem](ctx context.Context, c CacheManager, lockType shared.SentinelType, i item, keys []shared.CacheKey) (shared.CacheSentinel, error) {
	s := shared.NoLockSentinel

	var err error

	// Create/Update:
	//  Takes a lock if item does not exist, if read lock is in place
	//  If write lock is in place, replaces it with new write lock
	//  when the write completes it resets the value in the cache if it is different from value that it wrote to the server or bump the lock to conflict
	// Delete:
	//  Takes all locks regardless of key state
	// Read:
	//  Takes lock only if key is empty or unlocked
	s, err = c.P.WriteSentinel(ctx, lockType, keys)

	// If we are deleting, clear the dependencies and tombstone the dependency key prior to starting the delete
	// to ensure that stale data is not returned after the server registers the delete
	if lockType == shared.Delete && err == nil {
		err = c.P.ClearDependencies(ctx, i.GetDependenciesKey(c.N), true)
	}

	// Return a friendly error to the user indicating that the call should be retried
	if err != nil {
		uclog.Warningf(ctx, "Failed to get a lock for keys %v of type %v with %v", keys, lockType, err)
		return shared.NoLockSentinel, ucerr.Wrap(ucerr.WrapWithFriendlyStructure(jsonclient.Error{StatusCode: http.StatusConflict}, jsonclient.SDKStructuredError{
			Error: "Failed to get a cache lock due to contention. Please retry the call",
		}))
	}
	return s, nil
}

// ReleaseItemLock releases the lock for the given item
func ReleaseItemLock[item CacheSingleItem](ctx context.Context, c CacheManager, lockType shared.SentinelType, i item, sentinel shared.CacheSentinel) {
	if sentinel == shared.NoLockSentinel {
		return // nothing to clear if the lock wasn't acquired
	}

	keys := getItemLockKeys[item](ctx, lockType, c.N, i)

	c.P.ReleaseSentinel(ctx, keys, sentinel)
}

// ReleasePerItemCollectionLock releases the lock for the collection associated with a given item
func ReleasePerItemCollectionLock[item CacheSingleItem](ctx context.Context, c CacheManager, additionalColKeys []shared.CacheKey, i item, sentinel shared.CacheSentinel) {
	if sentinel == shared.NoLockSentinel {
		return // nothing to clear if the lock wasn't acquired
	}

	// Unlock the primary per item collection and any sub collections that are passed in
	keys := []shared.CacheKey{i.GetPerItemCollectionKey(c.N)}
	keys = append(keys, additionalColKeys...)

	c.P.ReleaseSentinel(ctx, keys, sentinel)
}

// GetItemFromCache gets the the value stored in key from the cache. The value could a single item or an array of items
func GetItemFromCache[item any](ctx context.Context, c CacheManager, key shared.CacheKey, lockOnMiss bool, ttl time.Duration) (*item, shared.CacheSentinel, error) {
	if ttl == SkipCacheTTL {
		return nil, "", nil
	}
	start := time.Now().UTC()
	value, s, err := c.P.GetValue(ctx, key, lockOnMiss)
	took := time.Now().UTC().Sub(start)
	if err != nil {
		return nil, "", ucerr.Wrap(err)
	}
	if value == nil {
		metrics.RecordCacheMiss(ctx, took)
		return nil, s, nil
	}

	var i item

	if err := json.Unmarshal([]byte(*value), &i); err != nil {
		return nil, "", nil
	}
	metrics.RecordCacheHit(ctx, took)
	return &i, "", nil
}

// SaveItemToCache saves the given item to the cache
func SaveItemToCache[item CacheSingleItem](ctx context.Context, c CacheManager, i item, sentinel shared.CacheSentinel,
	clearCollection bool, additionalColKeys []shared.CacheKey) {
	saveItemToCacheWorker(ctx, c, i, i.GetPrimaryKey(c.N), sentinel, clearCollection, additionalColKeys)
}

// SaveItemsFromCollectionToCache saves the items from a given collection into their separate keys
func SaveItemsFromCollectionToCache[item CacheSingleItem](ctx context.Context, c CacheManager, items []item, sentinel shared.CacheSentinel) {
	for _, i := range items {
		saveItemToCacheWorker(ctx, c, i, i.GetGlobalCollectionKey(c.N), sentinel, false, nil)
	}
}

func saveItemToCacheWorker[item CacheSingleItem](ctx context.Context, c CacheManager, i item, lkey shared.CacheKey, sentinel shared.CacheSentinel,
	clearCollection bool, additionalColKeys []shared.CacheKey) {
	if i.TTL(c.T) == SkipCacheTTL {
		return
	}

	if sentinel == shared.NoLockSentinel {
		return // no need to do work if we don't have the sentinel
	}

	if b, err := json.Marshal(i); err == nil {
		keyNames := []shared.CacheKey{}
		keyNames = append(keyNames, i.GetSecondaryKeys(c.N)...)
		keyNames = append(keyNames, i.GetPrimaryKey(c.N))
		keyset, conflict, err := c.P.SetValue(ctx, lkey, keyNames, string(b), sentinel, i.TTL(c.T))
		if err != nil {
			uclog.Errorf(ctx, "Error saving item to cache: %v", err)
		}
		// Cleart all the collections that this item might appear in. This is needed for create/update operations that might change the collection
		ckeys := []shared.CacheKey{}
		clearKeysOnError := false
		if clearCollection && !conflict {
			// Check if there is a default global collection for all items of this type
			if i.GetGlobalCollectionKey(c.N) != "" {
				ckeys = append(ckeys, i.GetGlobalCollectionKey(c.N))
			}
			// Check if there are any additional collections that this item might appear in passed in by the caller
			if len(additionalColKeys) > 0 {
				ckeys = append(ckeys, additionalColKeys...)
			}
			if err := c.P.DeleteValue(ctx, ckeys, false, true /* force delete regardless of value */); err != nil {
				uclog.Errorf(ctx, "Error clearing collection keys from cache: %v", err)
				clearKeysOnError = true
				keyset = false
			}

			// Check if there is a dependency list for this item (only needed on update to clear secondary collections)
			if i.GetDependenciesKey(c.N) != "" {
				if err := c.P.ClearDependencies(ctx, i.GetDependenciesKey(c.N), false); err != nil {
					uclog.Errorf(ctx, "Error clearing dependencies %v from cache: %v", i.GetDependenciesKey(c.N), err)
					clearKeysOnError = true
					keyset = false
				}

			}
			uclog.Verbosef(ctx, "Cleared collection keys %v from cache", ckeys)
		}

		depKeys := i.GetDependencyKeys(c.N)
		if len(depKeys) > 0 && keyset {
			if err := c.P.AddDependency(ctx, depKeys, keyNames, i.TTL(c.T)); err != nil {
				uclog.Warningf(ctx, "Failed to add dependency %v to key %v: %v", keyNames, depKeys, err)
				clearKeysOnError = true
				keyset = false
			}

		}
		if selfDepKey := i.GetDependenciesKey(c.N); selfDepKey != "" && keyset && len(i.GetSecondaryKeys(c.N)) != 0 {
			if err := c.P.AddDependency(ctx, []shared.CacheKey{selfDepKey}, i.GetSecondaryKeys(c.N), i.TTL(c.T)); err != nil {
				// This may fail if the item was deleted between where we stored it in the primary/secondary keys and here
				uclog.Debugf(ctx, "Failed to add secondary key dependency %v to key %v: %v", i.GetSecondaryKeys(c.N), selfDepKey, err)
				clearKeysOnError = true
			}
		}
		// Cache is still in consistent state in this case, we just failed to add the cache the item to do contention
		if clearKeysOnError {
			if err := c.P.DeleteValue(ctx, keyNames, false, false); err != nil {
				uclog.Warningf(ctx, "Failed to delete secondary key after dependency failure %v: %v", i.GetSecondaryKeys(c.N), err)
			}
		}
	}
}

// SaveItemsToCollection saves the given collection to collection key associated with the item or global to item type
// If this is a per item collection than "item" argument is the item with with the collection is associated and "cItems" is the collection
// to be stored.
func SaveItemsToCollection[item CacheSingleItem, cItem CacheSingleItem](ctx context.Context, c CacheManager,
	i item, colItems []cItem, lockKey shared.CacheKey, colKey shared.CacheKey, sentinel shared.CacheSentinel, isGlobal bool, ttl time.Duration) {
	if ttl == SkipCacheTTL {
		return
	}

	if colKey == "" || lockKey == "" {
		return // error condition
	}

	if sentinel == shared.NoLockSentinel {
		return // no need to do work if we don't have the sentinel
	}

	if b, err := json.Marshal(colItems); err == nil {
		// Get a list of items this collection depends on so that can add our collection key to their dependencies list
		dependentItems := map[shared.CacheKey]bool{}
		dependentKeys := make([]shared.CacheKey, 0, len(colItems))
		for _, ci := range colItems {
			depKeys := ci.GetDependencyKeys(c.N)
			for _, depKey := range depKeys {
				if !dependentItems[depKey] && depKey != "" {
					dependentItems[depKey] = true
					dependentKeys = append(dependentKeys, depKey)
				}
			}
			depKey := ci.GetDependenciesKey(c.N)
			if depKey != "" && !dependentItems[depKey] { // Some items can't be individually deleted/updated so they have no dependencies key
				dependentKeys = append(dependentKeys, depKey)
			}
		}
		// Don't cache the collection if it has too many dependencies
		if len(dependentKeys) > 100 /* TODO figure out the optimal number */ {
			return
		}

		if !isGlobal && i.GetDependenciesKey(c.N) != "" && !dependentItems[i.GetDependenciesKey(c.N)] {
			dependentKeys = append(dependentKeys, i.GetDependenciesKey(c.N))
		}
		saveCollection := true

		// We write the collection key into the dependency lists of items it depends on before saving it/
		// That way we save the collection if and only if all the lists are updated successfully.
		if len(dependentKeys) > 0 {
			if err := c.P.AddDependency(ctx, dependentKeys, []shared.CacheKey{colKey}, i.TTL(c.T)); err != nil {
				uclog.Warningf(ctx, "Didn't cache collection failed to add dependency %v to key %v: %v", dependentKeys, colKey, err)
				saveCollection = false
			}
		}
		// If we don't save the collection the cache is still in a consistent state - we just didn't cache the collection
		if saveCollection {
			if r, _, err := c.P.SetValue(ctx, lockKey, []shared.CacheKey{colKey}, string(b), sentinel, ttl); err == nil && r {
				uclog.Verbosef(ctx, "Saved collection %v to cache", colKey)
			}
		}
	}
}
