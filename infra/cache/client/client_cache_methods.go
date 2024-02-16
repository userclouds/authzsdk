package client

import (
	"context"

	"github.com/gofrs/uuid"

	"userclouds.com/infra/cache/shared"
	"userclouds.com/infra/request"
	"userclouds.com/infra/ucerr"
	"userclouds.com/infra/uclog"
)

// CreateItem creates an item
func CreateItem[item CacheSingleItem](ctx context.Context, cm *CacheManager, id uuid.UUID, i *item, keyID CacheKeyNameID, secondaryKey shared.CacheKey, ifNotExists bool,
	bypassCache bool, additionalKeysToClear []shared.CacheKey, action func(i *item) (*item, error), equals func(input *item, current *item) bool) (*item, error) {
	var err error
	sentinel := shared.NoLockSentinel

	if i == nil {
		return nil, ucerr.Errorf("CreateItem is called with nil input")
	}

	// Validate the item
	if err := (*i).Validate(); err != nil {
		return nil, ucerr.Wrap(err)
	}

	// Check if the object type already exists in the cache if we're using ifNotExists
	if ifNotExists && !bypassCache {
		keyName := secondaryKey
		if !id.IsNil() {
			keyName = cm.N.GetKeyNameWithID(keyID, id)
		}
		v, _, _, err := GetItemFromCache[item](ctx, *cm, keyName, false)
		if err != nil {
			uclog.Errorf(ctx, "GetItemFromCache failed to get item from cache: %v", err)
		} else if v != nil && equals(i, v) {
			return v, nil
		}
	}

	// On the client we always invalidate the local cache even if the cache if bypassed for read operation
	if cm != nil {
		sentinel, err = TakeItemLock(ctx, shared.Create, *cm, *i)
		if err != nil {
			return nil, ucerr.Wrap(err)
		}
		defer ReleaseItemLock(ctx, *cm, shared.Create, *i, sentinel)
	}

	var resp *item
	if resp, err = action(i); err != nil {
		return nil, ucerr.Wrap(err)
	}

	if !bypassCache {
		SaveItemToCache(ctx, *cm, *resp, sentinel, true, additionalKeysToClear)
	}
	return resp, nil
}

// CreateItemServer creates an item (wrapper for calls from ORM)
func CreateItemServer[item CacheSingleItem](ctx context.Context, cm *CacheManager, i *item, keyID CacheKeyNameID, additionalKeysToClear []shared.CacheKey, action func(i *item) error) error {
	_, err := CreateItem[item](ctx, cm, uuid.Nil, i, keyID, "", false, cm == nil, additionalKeysToClear,
		func(i *item) (*item, error) { return i, ucerr.Wrap(action(i)) }, func(input, current *item) bool { return false })
	return ucerr.Wrap(err)
}

// CreateItemClient creates an item (wrapper for calls from client)
func CreateItemClient[item CacheSingleItem](ctx context.Context, cm *CacheManager, id uuid.UUID, i *item, keyID CacheKeyNameID, secondaryKey shared.CacheKey, ifNotExists bool,
	bypassCache bool, additionalKeysToClear []shared.CacheKey, action func(i *item) (*item, error), equals func(input *item, current *item) bool) (*item, error) {
	ctx = request.NewRequestID(ctx)

	uclog.Verbosef(ctx, "CreateItemClient: %v key %v", *i, keyID)

	val, err := CreateItem[item](ctx, cm, id, i, keyID, secondaryKey, ifNotExists, bypassCache, additionalKeysToClear, action, equals)

	if err != nil {
		uclog.Errorf(ctx, "CreateItemClient failed to create item %v: %v", *i, err)
	}

	return val, ucerr.Wrap(err)
}

// GetItem returns the item
func GetItem[item CacheSingleItem](ctx context.Context, cm *CacheManager, id uuid.UUID, keyID CacheKeyNameID, modifiedKeyID CacheKeyNameID, bypassCache bool, action func(id uuid.UUID, conflict shared.CacheSentinel, i *item) error) (*item, error) {
	sentinel := shared.NoLockSentinel
	conflict := shared.TombstoneSentinel
	if !bypassCache {
		var cachedObj *item
		var err error
		if modifiedKeyID == "" {
			cachedObj, conflict, sentinel, err = GetItemFromCache[item](ctx, *cm, cm.N.GetKeyNameWithID(keyID, id), true)
		} else {
			cachedObj, conflict, sentinel, err = GetItemFromCacheWithModifiedKey[item](ctx, *cm, cm.N.GetKeyNameWithID(keyID, id), cm.N.GetKeyNameWithID(modifiedKeyID, id), true)

		}
		if err != nil {
			uclog.Errorf(ctx, "GetItemFromCache failed to get item from cache: %v", err)
		} else if cachedObj != nil {
			return cachedObj, nil
		}
	}

	var resp item
	if err := action(id, conflict, &resp); err != nil {
		return nil, ucerr.Wrap(err)
	}

	if !bypassCache {
		SaveItemToCache(ctx, *cm, resp, sentinel, false, nil)
	}
	return &resp, nil
}

// GetItemClient returns the item (wrapper for calls from client)
func GetItemClient[item CacheSingleItem](ctx context.Context, cm CacheManager, id uuid.UUID, keyID CacheKeyNameID, bypassCache bool, action func(id uuid.UUID, conflict shared.CacheSentinel, i *item) error) (*item, error) {
	ctx = request.NewRequestID(ctx)

	uclog.Verbosef(ctx, "ClientGetItem: %v key %v", id, keyID)

	val, err := GetItem[item](ctx, &cm, id, keyID, "", bypassCache, action)

	if err != nil {
		uclog.Errorf(ctx, "ClientGetItem failed to get item %v: %v", id, err)
	}

	return val, ucerr.Wrap(err)
}

// ServerGetItem returns the item (wrapper for calls from ORM)
func ServerGetItem[item CacheSingleItem](ctx context.Context, cm *CacheManager, id uuid.UUID, keyID CacheKeyNameID, modifiedKeyID CacheKeyNameID, action func(id uuid.UUID, conflict shared.CacheSentinel, i *item) error) (*item, error) {
	return GetItem[item](ctx, cm, id, keyID, modifiedKeyID, cm == nil, action)
}
