package authz

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/gofrs/uuid"

	clientcache "userclouds.com/infra/cache/client"
	cache "userclouds.com/infra/cache/shared"
	"userclouds.com/infra/jsonclient"
	"userclouds.com/infra/namespace/region"
	"userclouds.com/infra/pagination"
	"userclouds.com/infra/request"
	"userclouds.com/infra/sdkclient"
	"userclouds.com/infra/ucdb"
	"userclouds.com/infra/ucerr"
)

const (
	// DefaultObjTypeTTL specifies how long ObjectTypes remain in the cache by default. If you frequently delete ObjectTypes - you should lower this number
	DefaultObjTypeTTL time.Duration = 10 * time.Minute
	// DefaultEdgeTypeTTL specifies how long EdgeTypes remain in the cache by default. If you frequently delete ObjectTypes - you should lower this number
	DefaultEdgeTypeTTL time.Duration = 10 * time.Minute
	// DefaultObjTTL specifies how long Objects remain in the cache by default. If you frequently delete Objects (such as users) - you should lower this number
	DefaultObjTTL time.Duration = 5 * time.Minute
	// DefaultEdgeTTL specifies how long Edges remain in the cache by default. It is assumed that edges churn frequently so this number is set lower
	DefaultEdgeTTL time.Duration = 30 * time.Second
)

type options struct {
	ifNotExists           bool
	bypassCache           bool
	organizationID        uuid.UUID
	cacheProvider         clientcache.CacheProvider
	paginationOptions     []pagination.Option
	jsonclientOptions     []jsonclient.Option
	bypassAuthHeaderCheck bool // if we're using per-request header forwarding via PassthroughAuthorization, don't check for auth header
}

// Option makes authz.Client extensible
type Option interface {
	apply(*options)
}

type optFunc func(*options)

func (o optFunc) apply(opts *options) {
	o(opts)
}

// IfNotExists returns an Option that will cause the client not to return an error if an identical object to the one being created already exists
func IfNotExists() Option {
	return optFunc(func(opts *options) {
		opts.ifNotExists = true
	})
}

// BypassCache returns an Option that will cause the client to bypass the cache for the request (supported for read operations only)
func BypassCache() Option {
	return optFunc(func(opts *options) {
		opts.bypassCache = true
	})
}

// OrganizationID returns an Option that will cause the client to use the specified organization ID for the request
func OrganizationID(organizationID uuid.UUID) Option {
	return optFunc(func(opts *options) {
		opts.organizationID = organizationID
	})
}

// Pagination is a wrapper around pagination.Option
func Pagination(opt ...pagination.Option) Option {
	return optFunc(func(opts *options) {
		opts.paginationOptions = append(opts.paginationOptions, opt...)
	})
}

// JSONClient is a wrapper around jsonclient.Option
func JSONClient(opt ...jsonclient.Option) Option {
	return optFunc(func(opts *options) {
		opts.jsonclientOptions = append(opts.jsonclientOptions, opt...)
	})
}

// CacheProvider returns an Option that will cause the client to use given cache provider (can only be used on call to NewClient)
func CacheProvider(cp clientcache.CacheProvider) Option {
	return optFunc(func(opts *options) {
		opts.cacheProvider = cp
	})
}

// PassthroughAuthorization returns an Option that will cause the client to use the auth header from the request context
func PassthroughAuthorization() Option {
	return optFunc(func(opts *options) {
		opts.jsonclientOptions = append(opts.jsonclientOptions, jsonclient.PerRequestHeader(func(ctx context.Context) (string, string) {
			return "Authorization", request.GetAuthHeader(ctx)
		}))
		opts.bypassAuthHeaderCheck = true
	})
}

// Client is a client for the authz service
type Client struct {
	client            *sdkclient.Client
	options           options
	basePrefix        string
	basePrefixWithOrg string

	// Object type root cache contains:
	//    ObjTypeID (primary key) -> ObjType and objTypePrefix + TypeName (secondary key) -> ObjType
	//    ObjTypeCollection(global collection key) -> []ObjType (all object types
	// Edge type root cache contains:
	//    EdgeTypeID (primary key) -> EdgeType and edgeTypePrefix + TypeName (secondary key) -> EdgeType
	// Object root cache contains:
	//    ObjectID (primary key) -> Object and typeID + Object.Alias (secondary key) -> Object
	//    ObjectCollection(global collection key) -> lock only
	//    ObjectID + Edges (per item collection key) -> []Edges (all outgoing/incoming)
	//    ObjectID1 + Edges + ObjectID2 (per item sub collection key) -> []Edges (all between ObjectID1/ObjectID2)
	//    ObjectID1 + Path + ObjectID2 + Attribute (per item sub collection key) -> []AtributeNode (path between ObjectID1 and ObjectID2 for Attribute)
	//    ObjectID + Depencency (dependency key) -> []CacheKeys (all cache keys that depend on this object)
	// Edge root cache contains:
	//    EdgeID (primary key) -> Edge
	//    SourceObjID + TargetObjID + EdgeTypeID (secondary key) -> Edge
	//    EdgeID + Dependency (dependency key) -> []CacheKeys (all cache keys that depend on this edge)

	cp   clientcache.CacheProvider
	ttlP clientcache.CacheTTLProvider
}

// NewClient creates a new authz client
// Web API base URL, e.g. "http://localhost:1234".
func NewClient(url string, opts ...Option) (*Client, error) {
	return NewCustomClient(DefaultObjTypeTTL, DefaultEdgeTypeTTL, DefaultObjTTL, DefaultEdgeTTL, url, opts...)
}

// NewCustomClient creates a new authz client with different cache defaults
// Web API base URL, e.g. "http://localhost:1234".
func NewCustomClient(objTypeTTL time.Duration, edgeTypeTTL time.Duration, objTTL time.Duration, edgeTTL time.Duration,
	url string, opts ...Option) (*Client, error) {

	var options options
	for _, opt := range opts {
		opt.apply(&options)
	}

	cp := options.cacheProvider

	// If cache provider is not specified use default
	if cp == nil {
		cp = clientcache.NewInMemoryClientCacheProvider()
	}

	ttlP := newAuthzCacheTTLProvider(objTypeTTL, edgeTypeTTL, objTTL, edgeTTL)
	// TODO should be tenantID_OrgID
	basePrefixWihOrg := url + options.organizationID.String()

	c := &Client{
		client:            sdkclient.New(strings.TrimSuffix(url, "/"), options.jsonclientOptions...),
		options:           options,
		cp:                cp,
		ttlP:              ttlP,
		basePrefix:        url,
		basePrefixWithOrg: basePrefixWihOrg,
	}

	if !options.bypassAuthHeaderCheck {
		if err := c.client.ValidateBearerTokenHeader(); err != nil {
			return nil, ucerr.Wrap(err)
		}
	}

	return c, nil
}

func (c *Client) getCacheKeyNameProvider(orgID uuid.UUID) clientcache.CacheKeyNameProvider {
	if orgID == uuid.Nil {
		return newAuthZCacheNameProvider(c.basePrefixWithOrg)
	}
	return newAuthZCacheNameProvider(c.basePrefix + orgID.String())
}

// ErrObjectNotFound is returned if an object is not found.
var ErrObjectNotFound = ucerr.New("object not found")

// ErrRelationshipTypeNotFound is returned if a relationship type name
// (e.g. "editor") is not found.
var ErrRelationshipTypeNotFound = ucerr.New("relationship type not found")

// FlushCache clears all contents of the cache
func (c *Client) FlushCache() {
	c.cp.Flush(context.Background())
}

// FlushCacheEdges clears the edge cache only.
func (c *Client) FlushCacheEdges() {
	c.cp.Flush(context.Background())
}

// FlushCacheObjectsAndEdges clears the objects/edges cache only.
func (c *Client) FlushCacheObjectsAndEdges() {
	c.cp.Flush(context.Background())
}

// CreateObjectType creates a new type of object for the authz system.
func (c *Client) CreateObjectType(ctx context.Context, id uuid.UUID, typeName string, opts ...Option) (*ObjectType, error) {
	ctx = request.SetRequestIDIfNotSet(ctx, uuid.Must(uuid.NewV4()))

	options := c.options
	for _, opt := range opts {
		opt.apply(&options)
	}

	req := ObjectType{
		BaseModel: ucdb.NewBase(),
		TypeName:  typeName,
	}
	if id != uuid.Nil {
		req.ID = id
	}

	cm := clientcache.NewCacheManager(c.cp, c.getCacheKeyNameProvider(options.organizationID), c.ttlP)

	s, err := clientcache.TakeItemLock(ctx, cache.Create, cm, req)
	if err != nil {
		return nil, ucerr.Wrap(err)
	}
	defer clientcache.ReleaseItemLock(ctx, cm, cache.Create, req, s)

	var resp ObjectType
	if options.ifNotExists && id == uuid.Nil {
		exists, existingID, err := c.client.CreateIfNotExists(ctx, "/authz/objecttypes", req, &resp)
		if err != nil {
			return nil, ucerr.Wrap(err)
		}
		if exists {
			resp = req
			resp.ID = existingID
		}
	} else {
		if err := c.client.Post(ctx, "/authz/objecttypes", req, &resp); err != nil {
			return nil, ucerr.Wrap(err)
		}
	}

	clientcache.SaveItemToCache(ctx, cm, resp, s, true, nil)

	return &resp, nil
}

// FindObjectTypeID resolves an object type name to an ID.
func (c *Client) FindObjectTypeID(ctx context.Context, typeName string, opts ...Option) (uuid.UUID, error) {
	ctx = request.SetRequestIDIfNotSet(ctx, uuid.Must(uuid.NewV4()))

	options := c.options
	for _, opt := range opts {
		opt.apply(&options)
	}

	if !options.bypassCache {
		cm := clientcache.NewCacheManager(c.cp, c.getCacheKeyNameProvider(uuid.Nil), c.ttlP)
		v, _, err := clientcache.GetItemFromCache[ObjectType](ctx, cm, cm.N.GetKeyNameWithString(objTypeNameKeyID, typeName), false, c.ttlP.TTL(objTypeTTL))
		if err != nil {
			return uuid.Nil, ucerr.Wrap(err)
		}
		if v != nil {
			return v.ID, nil
		}
	}

	objTypes, err := c.ListObjectTypes(ctx, opts...)
	if err != nil {
		return uuid.Nil, ucerr.Wrap(err)
	}

	// Double check in case the cache was invalidated between the get and the lookup
	for _, objType := range objTypes {
		if objType.TypeName == typeName {
			return objType.ID, nil
		}
	}

	return uuid.Nil, ucerr.Errorf("authz object type '%s' not found", typeName)
}

// GetObjectType returns an object type by ID.
func (c *Client) GetObjectType(ctx context.Context, id uuid.UUID, opts ...Option) (*ObjectType, error) {
	ctx = request.SetRequestIDIfNotSet(ctx, uuid.Must(uuid.NewV4()))

	options := c.options
	for _, opt := range opts {
		opt.apply(&options)
	}

	cm := clientcache.NewCacheManager(c.cp, c.getCacheKeyNameProvider(uuid.Nil), c.ttlP)
	s := cache.NoLockSentinel
	if !options.bypassCache {
		var v *ObjectType
		var err error
		v, s, err = clientcache.GetItemFromCache[ObjectType](ctx, cm, cm.N.GetKeyNameWithID(objTypeKeyID, id), true, c.ttlP.TTL(objTypeTTL))
		if err != nil {
			return nil, ucerr.Wrap(err)
		}
		if v != nil {
			return v, nil
		}
	}
	var resp ObjectType
	if err := c.client.Get(ctx, fmt.Sprintf("/authz/objecttypes/%v", id), &resp); err != nil {
		return nil, ucerr.Wrap(err)
	}

	clientcache.SaveItemToCache(ctx, cm, resp, s, false, nil)
	return &resp, nil
}

// ListObjectTypesResponse is the paginated response from listing object types.
type ListObjectTypesResponse struct {
	Data []ObjectType `json:"data"`
	pagination.ResponseFields
}

// ListObjectTypes lists all object types in the system
func (c *Client) ListObjectTypes(ctx context.Context, opts ...Option) ([]ObjectType, error) {
	ctx = request.SetRequestIDIfNotSet(ctx, uuid.Must(uuid.NewV4()))

	options := c.options
	for _, opt := range opts {
		opt.apply(&options)
	}

	cm := clientcache.NewCacheManager(c.cp, c.getCacheKeyNameProvider(uuid.Nil), c.ttlP)
	s := cache.NoLockSentinel
	if !options.bypassCache {
		var v *[]ObjectType
		var err error
		cm := clientcache.NewCacheManager(c.cp, c.getCacheKeyNameProvider(uuid.Nil), c.ttlP)
		v, s, err = clientcache.GetItemFromCache[[]ObjectType](ctx, cm, cm.N.GetKeyNameStatic(objTypeCollectionKeyID), true, c.ttlP.TTL(objTypeTTL))
		if err != nil {
			return nil, ucerr.Wrap(err)
		}
		if v != nil {
			return *v, nil
		}
	}
	// TODO: we should eventually support pagination arguments to this method, but for now we assume
	// there aren't that many object types and just fetch them all.

	pager, err := pagination.ApplyOptions()
	if err != nil {
		return nil, ucerr.Wrap(err)
	}

	objTypes := make([]ObjectType, 0)

	for {
		query := pager.Query()

		var resp ListObjectTypesResponse
		if err := c.client.Get(ctx, fmt.Sprintf("/authz/objecttypes?%s", query.Encode()), &resp); err != nil {
			return nil, ucerr.Wrap(err)
		}

		objTypes = append(objTypes, resp.Data...)

		clientcache.SaveItemsFromCollectionToCache(ctx, cm, resp.Data, s)

		if !pager.AdvanceCursor(resp.ResponseFields) {
			break
		}
	}
	ckey := cm.N.GetKeyNameStatic(objTypeCollectionKeyID)
	clientcache.SaveItemsToCollection(ctx, cm, ObjectType{}, objTypes, ckey, ckey, s, true, c.ttlP.TTL(objTypeTTL))

	return objTypes, nil
}

// DeleteObjectType deletes an object type by ID.
func (c *Client) DeleteObjectType(ctx context.Context, objectTypeID uuid.UUID) error {
	ctx = request.SetRequestIDIfNotSet(ctx, uuid.Must(uuid.NewV4()))

	// We don't take a delete lock since we will flush the cache after the delete anyway
	if err := c.client.Delete(ctx, fmt.Sprintf("/authz/objecttypes/%s", objectTypeID), nil); err != nil {
		return ucerr.Wrap(err)
	}

	// There are so many potential inconsistencies when object type is deleted so flush the whole cache
	c.FlushCache()
	return nil
}

// CreateEdgeType creates a new type of edge for the authz system.
func (c *Client) CreateEdgeType(ctx context.Context, id uuid.UUID, sourceObjectTypeID, targetObjectTypeID uuid.UUID, typeName string, attributes Attributes, opts ...Option) (*EdgeType, error) {
	ctx = request.SetRequestIDIfNotSet(ctx, uuid.Must(uuid.NewV4()))

	options := c.options
	for _, opt := range opts {
		opt.apply(&options)
	}

	req := EdgeType{
		BaseModel:          ucdb.NewBase(),
		TypeName:           typeName,
		SourceObjectTypeID: sourceObjectTypeID,
		TargetObjectTypeID: targetObjectTypeID,
		Attributes:         attributes,
		OrganizationID:     options.organizationID,
	}
	if id != uuid.Nil {
		req.ID = id
	}

	cm := clientcache.NewCacheManager(c.cp, c.getCacheKeyNameProvider(options.organizationID), c.ttlP)

	s, err := clientcache.TakeItemLock(ctx, cache.Create, cm, req)
	if err != nil {
		return nil, ucerr.Wrap(err)
	}
	defer clientcache.ReleaseItemLock(ctx, cm, cache.Create, req, s)

	var resp EdgeType
	if options.ifNotExists && id == uuid.Nil {
		exists, existingID, err := c.client.CreateIfNotExists(ctx, "/authz/edgetypes", req, &resp)
		if err != nil {
			return nil, ucerr.Wrap(err)
		}
		if exists {
			resp = req
			resp.ID = existingID
		}
	} else {
		if err := c.client.Post(ctx, "/authz/edgetypes", req, &resp); err != nil {
			return nil, ucerr.Wrap(err)
		}
	}

	clientcache.SaveItemToCache(ctx, cm, resp, s, true, nil)

	return &resp, nil
}

// UpdateEdgeTypeRequest is the request struct for updating an edge type
type UpdateEdgeTypeRequest struct {
	TypeName   string     `json:"type_name" validate:"notempty"`
	Attributes Attributes `json:"attributes"`
}

// UpdateEdgeType updates an existing edge type in the authz system.
func (c *Client) UpdateEdgeType(ctx context.Context, id uuid.UUID, sourceObjectTypeID, targetObjectTypeID uuid.UUID, typeName string, attributes Attributes) (*EdgeType, error) {
	ctx = request.SetRequestIDIfNotSet(ctx, uuid.Must(uuid.NewV4()))

	req := UpdateEdgeTypeRequest{
		TypeName:   typeName,
		Attributes: attributes,
	}

	cm := clientcache.NewCacheManager(c.cp, c.getCacheKeyNameProvider(uuid.Nil), c.ttlP)

	eT := EdgeType{BaseModel: ucdb.NewBaseWithID(id)}
	s, err := clientcache.TakeItemLock(ctx, cache.Update, cm, eT)
	if err != nil {
		return nil, ucerr.Wrap(err)
	}
	defer clientcache.ReleaseItemLock(ctx, cm, cache.Update, eT, s)

	var resp EdgeType
	if err := c.client.Put(ctx, fmt.Sprintf("/authz/edgetypes/%s", id), req, &resp); err != nil {
		return nil, ucerr.Wrap(err)
	}

	clientcache.SaveItemToCache(ctx, cm, resp, s, true, nil)

	// For now flush the cache because we don't track all the paths that need to be invalidated
	c.FlushCache()

	return &resp, nil
}

// GetEdgeType gets an edge type (relationship) by its type ID.
func (c *Client) GetEdgeType(ctx context.Context, edgeTypeID uuid.UUID, opts ...Option) (*EdgeType, error) {
	ctx = request.SetRequestIDIfNotSet(ctx, uuid.Must(uuid.NewV4()))

	options := c.options
	for _, opt := range opts {
		opt.apply(&options)
	}

	cm := clientcache.NewCacheManager(c.cp, c.getCacheKeyNameProvider(uuid.Nil), c.ttlP)
	s := cache.NoLockSentinel
	if !options.bypassCache {
		var v *EdgeType
		var err error

		v, s, err = clientcache.GetItemFromCache[EdgeType](ctx, cm, cm.N.GetKeyNameWithID(edgeTypeKeyID, edgeTypeID), true, c.ttlP.TTL(edgeTypeTTL))
		if err != nil {
			return nil, ucerr.Wrap(err)
		}
		if v != nil {
			return v, nil
		}
	}
	var resp EdgeType
	if err := c.client.Get(ctx, fmt.Sprintf("/authz/edgetypes/%s", edgeTypeID), &resp); err != nil {
		return nil, ucerr.Wrap(err)
	}

	clientcache.SaveItemToCache(ctx, cm, resp, s, false, nil)
	return &resp, nil
}

// FindEdgeTypeID resolves an edge type name to an ID.
func (c *Client) FindEdgeTypeID(ctx context.Context, typeName string, opts ...Option) (uuid.UUID, error) {
	ctx = request.SetRequestIDIfNotSet(ctx, uuid.Must(uuid.NewV4()))

	options := c.options
	for _, opt := range opts {
		opt.apply(&options)
	}

	if !options.bypassCache {
		cm := clientcache.NewCacheManager(c.cp, c.getCacheKeyNameProvider(uuid.Nil), c.ttlP)
		v, _, err := clientcache.GetItemFromCache[EdgeType](ctx, cm, cm.N.GetKeyNameWithString(edgeTypeNameKeyID, typeName), false, c.ttlP.TTL(edgeTypeTTL))
		if err != nil {
			return uuid.Nil, ucerr.Wrap(err)
		}
		if v != nil {
			return v.ID, nil
		}
	}

	edgeTypes, err := c.ListEdgeTypes(ctx, opts...)
	if err != nil {
		return uuid.Nil, ucerr.Wrap(err)
	}

	// Double check if the cache was invalidated on the miss
	for _, edgeType := range edgeTypes {
		if edgeType.TypeName == typeName {
			return edgeType.ID, nil
		}
	}
	return uuid.Nil, ucerr.Errorf("authz edge type '%s' not found", typeName)
}

// ListEdgeTypesResponse is the paginated response from listing edge types.
type ListEdgeTypesResponse struct {
	Data []EdgeType `json:"data"`
	pagination.ResponseFields
}

// Description implements the Described interface for OpenAPI
func (r ListEdgeTypesResponse) Description() string {
	return "This object contains an array of edge types and pagination information"
}

// ListEdgeTypes lists all available edge types
func (c *Client) ListEdgeTypes(ctx context.Context, opts ...Option) ([]EdgeType, error) {
	ctx = request.SetRequestIDIfNotSet(ctx, uuid.Must(uuid.NewV4()))

	options := c.options
	for _, opt := range opts {
		opt.apply(&options)
	}

	cm := clientcache.NewCacheManager(c.cp, c.getCacheKeyNameProvider(options.organizationID), c.ttlP)
	s := cache.NoLockSentinel
	if !options.bypassCache {
		var v *[]EdgeType
		var err error
		v, s, err = clientcache.GetItemFromCache[[]EdgeType](ctx, cm, cm.N.GetKeyNameStatic(edgeTypeCollectionKeyID), true, c.ttlP.TTL(edgeTypeTTL))
		if err != nil {
			return nil, ucerr.Wrap(err)
		}
		if v != nil {
			return *v, nil
		}
	}

	// TODO: we should eventually support pagination arguments to this method, but for now we assume
	// there aren't that many edge types and just fetch them all.
	pager, err := pagination.ApplyOptions()
	if err != nil {
		return nil, ucerr.Wrap(err)
	}

	edgeTypes := make([]EdgeType, 0)

	for {
		query := pager.Query()
		if options.organizationID != uuid.Nil {
			query.Add("organization_id", options.organizationID.String())
		}

		var resp ListEdgeTypesResponse
		if err := c.client.Get(ctx, fmt.Sprintf("/authz/edgetypes?%s", query.Encode()), &resp); err != nil {
			return nil, ucerr.Wrap(err)
		}

		edgeTypes = append(edgeTypes, resp.Data...)

		clientcache.SaveItemsFromCollectionToCache(ctx, cm, resp.Data, s)

		if !pager.AdvanceCursor(resp.ResponseFields) {
			break
		}
	}
	ckey := cm.N.GetKeyNameStatic(edgeTypeCollectionKeyID)
	clientcache.SaveItemsToCollection(ctx, cm, EdgeType{}, edgeTypes, ckey, ckey, s, true, c.ttlP.TTL(edgeTypeTTL))

	return edgeTypes, nil
}

// DeleteEdgeType deletes an edge type by ID.
func (c *Client) DeleteEdgeType(ctx context.Context, edgeTypeID uuid.UUID) error {
	ctx = request.SetRequestIDIfNotSet(ctx, uuid.Must(uuid.NewV4()))

	// We don't take a delete lock since we will flush the cache after the delete anyway
	if err := c.client.Delete(ctx, fmt.Sprintf("/authz/edgetypes/%s", edgeTypeID), nil); err != nil {
		return ucerr.Wrap(err)
	}
	// There are so many potential inconsistencies when edge type is deleted so flush the whole cache
	c.FlushCache()
	return nil
}

// CreateObject creates a new object with a given ID, name, and type.
func (c *Client) CreateObject(ctx context.Context, id, typeID uuid.UUID, alias string, opts ...Option) (*Object, error) {
	ctx = request.SetRequestIDIfNotSet(ctx, uuid.Must(uuid.NewV4()))

	options := c.options
	for _, opt := range opts {
		opt.apply(&options)
	}

	obj := Object{
		BaseModel:      ucdb.NewBase(),
		Alias:          &alias,
		TypeID:         typeID,
		OrganizationID: options.organizationID,
	}
	if id != uuid.Nil {
		obj.ID = id
	}

	if alias == "" { // this allows storing multiple objects with "" alias
		obj.Alias = nil
	}

	cm := clientcache.NewCacheManager(c.cp, c.getCacheKeyNameProvider(options.organizationID), c.ttlP)

	s, err := clientcache.TakeItemLock(ctx, cache.Create, cm, obj)
	if err != nil {
		return nil, ucerr.Wrap(err)
	}
	defer clientcache.ReleaseItemLock(ctx, cm, cache.Create, obj, s)

	var resp Object
	if options.ifNotExists && id == uuid.Nil {
		exists, existingID, err := c.client.CreateIfNotExists(ctx, "/authz/objects", obj, &resp)
		if err != nil {
			return nil, ucerr.Wrap(err)
		}
		if exists {
			resp = obj
			resp.ID = existingID
		}
	} else {
		if err := c.client.Post(ctx, "/authz/objects", obj, &resp); err != nil {
			return nil, ucerr.Wrap(err)
		}
	}

	clientcache.SaveItemToCache(ctx, cm, resp, s, true, nil)
	return &resp, nil
}

// GetObject returns an object by ID.
func (c *Client) GetObject(ctx context.Context, id uuid.UUID, opts ...Option) (*Object, error) {
	ctx = request.SetRequestIDIfNotSet(ctx, uuid.Must(uuid.NewV4()))

	options := c.options
	for _, opt := range opts {
		opt.apply(&options)
	}

	cm := clientcache.NewCacheManager(c.cp, c.getCacheKeyNameProvider(uuid.Nil), c.ttlP)

	s := cache.NoLockSentinel
	if !options.bypassCache {
		var v *Object
		var err error
		v, s, err = clientcache.GetItemFromCache[Object](ctx, cm, cm.N.GetKeyNameWithID(objectKeyID, id), true, c.ttlP.TTL(objTTL))
		if err != nil {
			return nil, ucerr.Wrap(err)
		}
		if v != nil {
			return v, nil
		}
	}

	var resp Object
	if err := c.client.Get(ctx, fmt.Sprintf("/authz/objects/%s", id), &resp); err != nil {
		return nil, ucerr.Wrap(err)
	}

	clientcache.SaveItemToCache(ctx, cm, resp, s, false, nil)
	return &resp, nil
}

// GetObjectForName returns an object with a given name.
func (c *Client) GetObjectForName(ctx context.Context, typeID uuid.UUID, name string, opts ...Option) (*Object, error) {
	ctx = request.SetRequestIDIfNotSet(ctx, uuid.Must(uuid.NewV4()))

	if typeID == UserObjectTypeID {
		return nil, ucerr.New("_user objects do not currently support lookup by alias")
	}

	options := c.options
	for _, opt := range opts {
		opt.apply(&options)
	}

	cm := clientcache.NewCacheManager(c.cp, c.getCacheKeyNameProvider(uuid.Nil), c.ttlP)

	if !options.bypassCache {
		var v *Object
		var err error

		v, _, err = clientcache.GetItemFromCache[Object](ctx, cm, cm.N.GetKeyName(objAliasNameKeyID, []string{typeID.String(), name}), false, c.ttlP.TTL(objTTL))
		if err != nil {
			return nil, ucerr.Wrap(err)
		}
		if v != nil {
			return v, nil
		}
	}

	// TODO: support a name-based path, e.g. `/authz/objects/<objectname>`
	pager, err := pagination.ApplyOptions()
	if err != nil {
		return nil, ucerr.Wrap(err)
	}
	query := pager.Query()
	query.Add("type_id", typeID.String())
	query.Add("name", name)
	resp, err := c.ListObjectsFromQuery(ctx, query, opts...)
	if err != nil {
		return nil, ucerr.Wrap(err)
	}

	if len(resp.Data) > 0 {
		return &resp.Data[0], nil
	}
	return nil, ErrObjectNotFound
}

// DeleteObject deletes an object by ID.
func (c *Client) DeleteObject(ctx context.Context, id uuid.UUID) error {
	ctx = request.SetRequestIDIfNotSet(ctx, uuid.Must(uuid.NewV4()))

	cm := clientcache.NewCacheManager(c.cp, c.getCacheKeyNameProvider(uuid.Nil), c.ttlP)
	obj := &Object{BaseModel: ucdb.NewBaseWithID(id)}
	// Stop in flight reads/writes of this object, edges leading to/from this object, paths including this object and object collection from committing to the cache
	obj, _, err := clientcache.GetItemFromCache[Object](ctx, cm, obj.GetPrimaryKey(cm.N), false, c.ttlP.TTL(objTTL))
	if err != nil {
		return ucerr.Wrap(err)
	}

	if obj == nil {
		obj = &Object{BaseModel: ucdb.NewBaseWithID(id)}
	}
	s, err := clientcache.TakeItemLock(ctx, cache.Delete, cm, *obj)
	if err != nil {
		return ucerr.Wrap(err)
	}
	defer clientcache.ReleaseItemLock(ctx, cm, cache.Delete, *obj, s)

	if err := c.client.Delete(ctx, fmt.Sprintf("/authz/objects/%s", id), nil); err != nil {
		return ucerr.Wrap(err)
	}
	return nil
}

// DeleteEdgesByObject deletes all edges going in or  out of an object by ID.
func (c *Client) DeleteEdgesByObject(ctx context.Context, id uuid.UUID) error {
	ctx = request.SetRequestIDIfNotSet(ctx, uuid.Must(uuid.NewV4()))

	cm := clientcache.NewCacheManager(c.cp, c.getCacheKeyNameProvider(uuid.Nil), c.ttlP)
	// Stop in flight reads of edges that include this object as source or target as well as paths starting from this object from committing to the cache
	// We don't block reads of collections/paths that end at this object since we may not have full set of edges without reading the server
	obj := Object{BaseModel: ucdb.NewBaseWithID(id)}

	// Taking a lock will delete all edges and paths that include this object as source or target. We intentionally tombstone the dependency key for the object to
	// prevent inflight reads of edge collection from object connected to this one from committing potentially stale results to the cache.
	s, err := clientcache.TakePerItemCollectionLock[Object](ctx, cache.Delete, cm, nil, obj)

	if err != nil {
		return ucerr.Wrap(err)
	}
	defer clientcache.ReleasePerItemCollectionLock[Object](ctx, cm, nil, obj, s)

	if err := c.client.Delete(ctx, fmt.Sprintf("/authz/objects/%s/edges", id), nil); err != nil {
		return ucerr.Wrap(err)
	}
	return nil
}

// ListObjectsResponse represents a paginated response from listing objects.
type ListObjectsResponse struct {
	Data []Object `json:"data"`
	pagination.ResponseFields
}

// ListObjects lists `limit` objects in sorted order with pagination, starting after a given ID (or uuid.Nil to start from the beginning).
func (c *Client) ListObjects(ctx context.Context, opts ...Option) (*ListObjectsResponse, error) {
	ctx = request.SetRequestIDIfNotSet(ctx, uuid.Must(uuid.NewV4()))

	options := c.options
	for _, opt := range opts {
		opt.apply(&options)
	}

	pager, err := pagination.ApplyOptions(options.paginationOptions...)
	if err != nil {
		return nil, ucerr.Wrap(err)
	}
	query := pager.Query()
	if options.organizationID != uuid.Nil {
		query.Add("organization_id", options.organizationID.String())
	}
	return c.ListObjectsFromQuery(ctx, query, opts...)
}

// ListObjectsFromQuery takes in a query that can handle filters passed from console as well as the default method.
func (c *Client) ListObjectsFromQuery(ctx context.Context, query url.Values, opts ...Option) (*ListObjectsResponse, error) {
	ctx = request.SetRequestIDIfNotSet(ctx, uuid.Must(uuid.NewV4()))

	options := c.options
	for _, opt := range opts {
		opt.apply(&options)
	}
	if options.organizationID != uuid.Nil {
		query.Add("organization_id", options.organizationID.String())
	}

	cm := clientcache.NewCacheManager(c.cp, c.getCacheKeyNameProvider(options.organizationID), c.ttlP)

	s := cache.NoLockSentinel
	if !options.bypassCache {
		var err error
		ckey := cm.N.GetKeyNameStatic(objCollectionKeyID)
		_, s, err := clientcache.GetItemFromCache[[]Object](ctx, cm, ckey, true, c.ttlP.TTL(objTTL))
		if err != nil {
			return nil, ucerr.Wrap(err)
		}
		// Release the lock after the request is done since we are not writing to globabal collection
		defer clientcache.ReleasePerItemCollectionLock[Object](ctx, cm, []cache.CacheKey{ckey}, Object{}, s)
	}

	// TODO needs to always be paginated get
	var resp ListObjectsResponse
	if err := c.client.Get(ctx, fmt.Sprintf("/authz/objects?%s", query.Encode()), &resp); err != nil {
		return nil, ucerr.Wrap(err)
	}

	clientcache.SaveItemsFromCollectionToCache(ctx, cm, resp.Data, s)

	return &resp, nil
}

// ListEdgesResponse is the paginated response from listing edges.
type ListEdgesResponse struct {
	Data []Edge `json:"data"`
	pagination.ResponseFields
}

// ListEdges lists `limit` edges.
func (c *Client) ListEdges(ctx context.Context, opts ...pagination.Option) (*ListEdgesResponse, error) {
	ctx = request.SetRequestIDIfNotSet(ctx, uuid.Must(uuid.NewV4()))

	// TODO: this function doesn't support organizations yet, because I haven't figured out a performant way to
	// do it.  The problem is that we need to filter by organization ID, but we don't have that information in
	// the edges table, only on the objects they connect.
	pager, err := pagination.ApplyOptions(opts...)
	if err != nil {
		return nil, ucerr.Wrap(err)
	}

	query := pager.Query()

	var resp ListEdgesResponse
	if err := c.client.Get(ctx, fmt.Sprintf("/authz/edges?%s", query.Encode()), &resp); err != nil {
		return nil, ucerr.Wrap(err)
	}

	// We don't save the individual edges in the cache because it is not certain that edges will be accessed by ID or name in the immediate future

	return &resp, nil
}

// ListEdgesOnObject lists `limit` edges (relationships) where the given object is a source or target.
func (c *Client) ListEdgesOnObject(ctx context.Context, objectID uuid.UUID, opts ...pagination.Option) (*ListEdgesResponse, error) {
	ctx = request.SetRequestIDIfNotSet(ctx, uuid.Must(uuid.NewV4()))

	pager, err := pagination.ApplyOptions(opts...)
	if err != nil {
		return nil, ucerr.Wrap(err)
	}

	query := pager.Query()

	cm := clientcache.NewCacheManager(c.cp, c.getCacheKeyNameProvider(uuid.Nil), c.ttlP)
	obj := Object{BaseModel: ucdb.NewBaseWithID(objectID)}

	edges, s, err := clientcache.GetItemFromCache[[]Edge](ctx, cm, cm.N.GetKeyNameWithID(objEdgesKeyID, objectID), true, c.ttlP.TTL(edgeTTL))

	if err != nil {
		return nil, ucerr.Wrap(err)
	}

	if edges != nil && len(*edges) <= pager.GetLimit() {
		resp := ListEdgesResponse{Data: *edges, ResponseFields: pagination.ResponseFields{HasNext: false}}
		return &resp, nil
	}

	// Only release the sentinal if we didn't get the edges from the cache
	if edges == nil {
		defer clientcache.ReleasePerItemCollectionLock(ctx, cm, nil, obj, s)
	}

	var resp ListEdgesResponse
	if err := c.client.Get(ctx, fmt.Sprintf("/authz/objects/%s/edges?%s", objectID, query.Encode()), &resp); err != nil {
		return nil, ucerr.Wrap(err)
	}

	// Only cache the response if it fits on one page
	if !resp.HasNext && !resp.HasPrev {
		ckey := cm.N.GetKeyNameWithID(objEdgesKeyID, objectID)
		clientcache.SaveItemsToCollection(ctx, cm, obj, resp.Data, ckey, ckey, s, false, c.ttlP.TTL(edgeTTL))
	}
	return &resp, nil
}

// ListEdgesBetweenObjects lists all edges (relationships) with a given source & target object.
func (c *Client) ListEdgesBetweenObjects(ctx context.Context, sourceObjectID, targetObjectID uuid.UUID, opts ...Option) ([]Edge, error) {
	ctx = request.SetRequestIDIfNotSet(ctx, uuid.Must(uuid.NewV4()))

	options := c.options
	for _, opt := range opts {
		opt.apply(&options)
	}

	cm := clientcache.NewCacheManager(c.cp, c.getCacheKeyNameProvider(uuid.Nil), c.ttlP)
	obj := Object{BaseModel: ucdb.NewBaseWithID(sourceObjectID)}
	ckey := cm.N.GetKeyName(edgesObjToObjID, []string{sourceObjectID.String(), targetObjectID.String()})

	s := cache.NoLockSentinel
	if !options.bypassCache {
		var cEdges *[]Edge
		var err error

		// First try to read the all in/out edges from source object
		cEdges, _, err = clientcache.GetItemFromCache[[]Edge](ctx, cm, cm.N.GetKeyNameWithID(objEdgesKeyID, sourceObjectID), false, c.ttlP.TTL(edgeTTL))
		if err != nil {
			return nil, ucerr.Wrap(err)
		}
		if cEdges != nil {
			filteredEdges := make([]Edge, 0)
			for _, edge := range *cEdges {
				if edge.TargetObjectID == targetObjectID {
					filteredEdges = append(filteredEdges, edge)
				}
			}
			return filteredEdges, nil
		}

		// Next try to read the edges between target object and source object. We could also try to read the edges from target object but in authz graph
		// it is rare to traverse in both directions so those collections would be less likely to be cached.
		cEdges, s, err = clientcache.GetItemFromCache[[]Edge](ctx, cm, ckey, true, c.ttlP.TTL(edgeTTL))
		if err != nil {
			return nil, ucerr.Wrap(err)
		}

		if cEdges != nil {
			return *cEdges, nil
		}

		// Clear the lock in case of an error
		defer clientcache.ReleasePerItemCollectionLock(ctx, cm, []cache.CacheKey{ckey}, obj, s)
	}
	query := url.Values{}
	query.Add("target_object_id", targetObjectID.String())
	var resp ListEdgesResponse
	var edges []Edge
	for {
		if err := c.client.Get(ctx, fmt.Sprintf("/authz/objects/%s/edges?%s", sourceObjectID, query.Encode()), &resp); err != nil {
			return nil, ucerr.Wrap(err)
		}
		edges = append(edges, resp.Data...)
		if !resp.HasNext {
			break
		}
	}

	clientcache.SaveItemsToCollection(ctx, cm, obj, resp.Data, ckey, ckey, s, false, c.ttlP.TTL(edgeTTL))

	return edges, nil
}

// GetEdge returns an edge by ID.
func (c *Client) GetEdge(ctx context.Context, id uuid.UUID, opts ...Option) (*Edge, error) {
	ctx = request.SetRequestIDIfNotSet(ctx, uuid.Must(uuid.NewV4()))

	options := c.options
	for _, opt := range opts {
		opt.apply(&options)
	}

	cm := clientcache.NewCacheManager(c.cp, c.getCacheKeyNameProvider(uuid.Nil), c.ttlP)

	s := cache.NoLockSentinel
	if !options.bypassCache {
		var edge *Edge
		var err error

		edge, s, err = clientcache.GetItemFromCache[Edge](ctx, cm, cm.N.GetKeyNameWithID(edgeKeyID, id), true, c.ttlP.TTL(edgeTTL))
		if err != nil {
			return nil, ucerr.Wrap(err)
		}
		if edge != nil {
			return edge, nil
		}
	}
	var resp Edge
	if err := c.client.Get(ctx, fmt.Sprintf("/authz/edges/%s", id), &resp); err != nil {
		return nil, ucerr.Wrap(err)
	}

	clientcache.SaveItemToCache(ctx, cm, resp, s, false, nil)

	return &resp, nil
}

// FindEdge finds an existing edge (relationship) between two objects.
func (c *Client) FindEdge(ctx context.Context, sourceObjectID, targetObjectID, edgeTypeID uuid.UUID, opts ...Option) (*Edge, error) {
	ctx = request.SetRequestIDIfNotSet(ctx, uuid.Must(uuid.NewV4()))

	options := c.options
	for _, opt := range opts {
		opt.apply(&options)
	}

	cm := clientcache.NewCacheManager(c.cp, c.getCacheKeyNameProvider(uuid.Nil), c.ttlP)

	s := cache.NoLockSentinel
	if !options.bypassCache {
		var edge *Edge
		var edges *[]Edge
		var err error

		// Try to fetch the invidual edge first using secondary key  Source_Target_TypeID
		edge, _, err = clientcache.GetItemFromCache[Edge](ctx, cm, cm.N.GetKeyName(edgeFullKeyID, []string{sourceObjectID.String(), targetObjectID.String(), edgeTypeID.String()}), false, c.ttlP.TTL(edgeTTL))
		// Since we are not taking a lock we can ignore cache errors
		if err == nil && edge != nil {
			return edge, nil
		}
		// If the edges are in the cache by source->target - iterate over that set first
		edges, _, err = clientcache.GetItemFromCache[[]Edge](ctx, cm, cm.N.GetKeyName(edgesObjToObjID, []string{sourceObjectID.String(), targetObjectID.String()}), false, c.ttlP.TTL(edgeTTL))
		// Since we are not taking a lock we can ignore cache errors
		if err == nil && edges != nil {
			for _, edge := range *edges {
				if edge.EdgeTypeID == edgeTypeID {
					return &edge, nil
				}
			}
			// In theory we could return NotFound here but this is a rare enough case that it makes sense to try the server
		}
		// If there is a cache miss, try to get the edges from all in/out edges on the source object
		edges, _, err = clientcache.GetItemFromCache[[]Edge](ctx, cm, cm.N.GetKeyNameWithID(objEdgesKeyID, sourceObjectID), false, c.ttlP.TTL(edgeTTL))
		// Since we are not taking a lock we can ignore cache errors
		if err == nil && edges != nil {
			for _, edge := range *edges {
				if edge.TargetObjectID == targetObjectID && edge.EdgeTypeID == edgeTypeID {
					return &edge, nil
				}
			}
			// In theory we could return NotFound here but this is a rare enough case that it makes sense to try the server
		}
		// We could also try all in/out edges from targetObjectID collection

		// If we still don't have the edge, try the server but we can't take a lock single edge lock since we don't know the primary key
		_, s, err = clientcache.GetItemFromCache[[]Object](ctx, cm, cm.N.GetKeyNameStatic(edgeCollectionKeyID), true, c.ttlP.TTL(objTTL))
		if err != nil {
			return nil, ucerr.Wrap(err)
		}
	}
	var resp ListEdgesResponse

	query := url.Values{}
	query.Add("source_object_id", sourceObjectID.String())
	query.Add("target_object_id", targetObjectID.String())
	query.Add("edge_type_id", edgeTypeID.String())
	if err := c.client.Get(ctx, fmt.Sprintf("/authz/edges?%s", query.Encode()), &resp); err != nil {
		return nil, ucerr.Wrap(err)
	}
	if len(resp.Data) != 1 {
		return nil, ucerr.Errorf("expected 1 edge from FindEdge, got %d", len(resp.Data))
	}

	clientcache.SaveItemsFromCollectionToCache(ctx, cm, resp.Data, s)

	return &resp.Data[0], nil
}

// CreateEdge creates an edge (relationship) between two objects.
func (c *Client) CreateEdge(ctx context.Context, id, sourceObjectID, targetObjectID, edgeTypeID uuid.UUID, opts ...Option) (*Edge, error) {
	ctx = request.SetRequestIDIfNotSet(ctx, uuid.Must(uuid.NewV4()))

	options := c.options
	for _, opt := range opts {
		opt.apply(&options)
	}

	req := Edge{
		BaseModel:      ucdb.NewBase(),
		EdgeTypeID:     edgeTypeID,
		SourceObjectID: sourceObjectID,
		TargetObjectID: targetObjectID,
	}
	if id != uuid.Nil {
		req.ID = id
	}
	cm := clientcache.NewCacheManager(c.cp, c.getCacheKeyNameProvider(uuid.Nil), c.ttlP)

	s, err := clientcache.TakeItemLock(ctx, cache.Create, cm, req)
	if err != nil {
		return nil, ucerr.Wrap(err)
	}
	defer clientcache.ReleaseItemLock(ctx, cm, cache.Create, req, s)

	var resp Edge
	if options.ifNotExists && id == uuid.Nil {
		exists, existingID, err := c.client.CreateIfNotExists(ctx, "/authz/edges", req, &resp)
		if err != nil {
			return nil, ucerr.Wrap(err)
		}
		if exists {
			resp = req
			resp.ID = existingID
		}
	} else {
		if err := c.client.Post(ctx, "/authz/edges", req, &resp); err != nil {
			return nil, ucerr.Wrap(err)
		}
	}

	clientcache.SaveItemToCache(ctx, cm, resp, s, true,
		// Clear additional collections that may be invalidated by this write
		[]cache.CacheKey{cm.N.GetKeyName(edgesObjToObjID, []string{resp.SourceObjectID.String(), resp.TargetObjectID.String()}), // Source -> Target edges collection
			cm.N.GetKeyNameWithID(objEdgesKeyID, resp.SourceObjectID),                                              // Source all in/out edges collection
			cm.N.GetKeyName(edgesObjToObjID, []string{resp.TargetObjectID.String(), resp.SourceObjectID.String()}), // Target -> Source edges collection
			cm.N.GetKeyNameWithID(objEdgesKeyID, resp.TargetObjectID),                                              // Target all in/out edges collection
		})

	return &resp, nil
}

// DeleteEdge deletes an edge by ID.
func (c *Client) DeleteEdge(ctx context.Context, edgeID uuid.UUID) error {
	ctx = request.SetRequestIDIfNotSet(ctx, uuid.Must(uuid.NewV4()))

	cm := clientcache.NewCacheManager(c.cp, c.getCacheKeyNameProvider(uuid.Nil), c.ttlP)
	edge, _, err := clientcache.GetItemFromCache[Edge](ctx, cm, cm.N.GetKeyNameWithID(edgeKeyID, edgeID), false, c.ttlP.TTL(edgeTTL))
	if err != nil {
		return ucerr.Wrap(err)
	}

	edgeBase := Edge{BaseModel: ucdb.NewBaseWithID(edgeID)}
	if edge == nil {
		edge = &edgeBase
	}
	s, err := clientcache.TakeItemLock(ctx, cache.Delete, cm, *edge)
	if err != nil {
		return ucerr.Wrap(err)
	}
	defer clientcache.ReleaseItemLock(ctx, cm, cache.Delete, *edge, s)

	if err = c.client.Delete(ctx, fmt.Sprintf("/authz/edges/%s", edgeID), nil); err != nil {
		return ucerr.Wrap(err)
	}

	return nil
}

// AttributePathNode is a node in a path list from source to target, if CheckAttribute succeeds.
type AttributePathNode struct {
	ObjectID uuid.UUID `json:"object_id"`
	EdgeID   uuid.UUID `json:"edge_id"`
}

// CheckAttributeResponse is returned by the checkattribute endpoint.
type CheckAttributeResponse struct {
	HasAttribute bool                `json:"has_attribute"`
	Path         []AttributePathNode `json:"path"`
}

// CheckAttribute returns true if the source object has the given attribute on the target object.
func (c *Client) CheckAttribute(ctx context.Context, sourceObjectID, targetObjectID uuid.UUID, attributeName string, opts ...Option) (*CheckAttributeResponse, error) {
	ctx = request.SetRequestIDIfNotSet(ctx, uuid.Must(uuid.NewV4()))

	options := c.options
	for _, opt := range opts {
		opt.apply(&options)
	}

	cm := clientcache.NewCacheManager(c.cp, c.getCacheKeyNameProvider(uuid.Nil), c.ttlP)
	ckey := cm.N.GetKeyName(attributePathObjToObjID, []string{sourceObjectID.String(), targetObjectID.String(), attributeName})

	s := cache.NoLockSentinel
	if !options.bypassCache {
		var path *[]AttributePathNode
		var err error

		path, s, err = clientcache.GetItemFromCache[[]AttributePathNode](ctx, cm, ckey, true, c.ttlP.TTL(edgeTTL))
		if err != nil {
			return nil, ucerr.Wrap(err)
		}

		if path != nil {
			return &CheckAttributeResponse{HasAttribute: true, Path: *path}, nil
		}
	}

	obj := Object{BaseModel: ucdb.NewBaseWithID(sourceObjectID)}

	// Release the lock in case of error
	defer clientcache.ReleasePerItemCollectionLock(ctx, cm, []cache.CacheKey{ckey}, obj, s)

	var resp CheckAttributeResponse
	query := url.Values{}
	query.Add("source_object_id", sourceObjectID.String())
	query.Add("target_object_id", targetObjectID.String())
	query.Add("attribute", attributeName)
	if err := c.client.Get(ctx, fmt.Sprintf("/authz/checkattribute?%s", query.Encode()), &resp); err != nil {
		return nil, ucerr.Wrap(err)
	}

	if resp.HasAttribute {
		// We can only cache positive responses, since we don't know when the path will be added to invalidate the negative result.
		clientcache.SaveItemsToCollection(ctx, cm, obj, resp.Path, ckey, ckey, s, false, c.ttlP.TTL(edgeTTL))
	}
	return &resp, nil
}

// ListAttributes returns a list of attributes that the source object has on the target object.
func (c *Client) ListAttributes(ctx context.Context, sourceObjectID, targetObjectID uuid.UUID) ([]string, error) {
	ctx = request.SetRequestIDIfNotSet(ctx, uuid.Must(uuid.NewV4()))

	var resp []string
	query := url.Values{}
	query.Add("source_object_id", sourceObjectID.String())
	query.Add("target_object_id", targetObjectID.String())
	if err := c.client.Get(ctx, fmt.Sprintf("/authz/listattributes?%s", query.Encode()), &resp); err != nil {
		return nil, ucerr.Wrap(err)
	}
	// This is currently uncacheable until we return a path for each attribute from the server that we can use for invalidation.
	return resp, nil
}

// ListObjectsReachableWithAttributeResponse is the response from the ListObjectsReachableWithAttribute endpoint.
type ListObjectsReachableWithAttributeResponse struct {
	Data []uuid.UUID `json:"data"`
}

// ListObjectsReachableWithAttribute returns a list of object IDs of a certain type that are reachable from the source object with the given attribute
func (c *Client) ListObjectsReachableWithAttribute(ctx context.Context, sourceObjectID uuid.UUID, targetObjectTypeID uuid.UUID, attributeName string) ([]uuid.UUID, error) {
	ctx = request.SetRequestIDIfNotSet(ctx, uuid.Must(uuid.NewV4()))

	var resp ListObjectsReachableWithAttributeResponse
	query := url.Values{}
	query.Add("source_object_id", sourceObjectID.String())
	query.Add("target_object_type_id", targetObjectTypeID.String())
	query.Add("attribute", attributeName)
	if err := c.client.Get(ctx, fmt.Sprintf("/authz/listobjectsreachablewithattribute?%s", query.Encode()), &resp); err != nil {
		return nil, ucerr.Wrap(err)
	}

	// This is currently uncacheable until we return a path for each reachable object from the server that we can use for invalidation.
	return resp.Data, nil
}

// ListOrganizationsResponse is the response from the ListOrganizations endpoint.
type ListOrganizationsResponse struct {
	Data []Organization `json:"data"`
	pagination.ResponseFields
}

// ListOrganizationsFromQuery takes in a query that can handle filters passed from console as well as the default method.
func (c *Client) ListOrganizationsFromQuery(ctx context.Context, query url.Values, opts ...Option) (*ListOrganizationsResponse, error) {
	ctx = request.SetRequestIDIfNotSet(ctx, uuid.Must(uuid.NewV4()))

	var options options
	for _, opt := range opts {
		opt.apply(&options)
	}

	var resp ListOrganizationsResponse
	if err := c.client.Get(ctx, fmt.Sprintf("/authz/organizations?%s", query.Encode()), &resp); err != nil {
		return nil, ucerr.Wrap(err)
	}

	return &resp, nil
}

// ListOrganizationsPaginated lists `limit` organizations in sorted order with pagination, starting after a given ID (or uuid.Nil to start from the beginning).
func (c *Client) ListOrganizationsPaginated(ctx context.Context, opts ...Option) (*ListOrganizationsResponse, error) {
	ctx = request.SetRequestIDIfNotSet(ctx, uuid.Must(uuid.NewV4()))

	var options options
	for _, opt := range opts {
		opt.apply(&options)
	}

	pager, err := pagination.ApplyOptions(options.paginationOptions...)
	if err != nil {
		return nil, ucerr.Wrap(err)
	}
	query := pager.Query()
	return c.ListOrganizationsFromQuery(ctx, query)
}

// ListOrganizations lists all organizations for a tenant
func (c *Client) ListOrganizations(ctx context.Context, opts ...Option) ([]Organization, error) {
	ctx = request.SetRequestIDIfNotSet(ctx, uuid.Must(uuid.NewV4()))

	options := c.options
	for _, opt := range opts {
		opt.apply(&options)
	}

	cm := clientcache.NewCacheManager(c.cp, c.getCacheKeyNameProvider(uuid.Nil), c.ttlP)
	s := cache.NoLockSentinel
	if !options.bypassCache {
		var v *[]Organization
		var err error
		cm := clientcache.NewCacheManager(c.cp, c.getCacheKeyNameProvider(uuid.Nil), c.ttlP)
		v, s, err = clientcache.GetItemFromCache[[]Organization](ctx, cm, cm.N.GetKeyNameStatic(orgCollectionKeyID), true, c.ttlP.TTL(orgTTL))
		if err != nil {
			return nil, ucerr.Wrap(err)
		}
		if v != nil {
			return *v, nil
		}
	}

	// TODO: we should eventually support pagination arguments to this method, but for now we assume
	// there aren't that many organizations and just fetch them all.

	orgs := make([]Organization, 0)

	pager, err := pagination.ApplyOptions()
	if err != nil {
		return nil, ucerr.Wrap(err)
	}

	for {
		query := pager.Query()

		var resp ListOrganizationsResponse
		if err := c.client.Get(ctx, fmt.Sprintf("/authz/organizations?%s", query.Encode()), &resp); err != nil {
			return nil, ucerr.Wrap(err)
		}

		orgs = append(orgs, resp.Data...)

		clientcache.SaveItemsFromCollectionToCache(ctx, cm, resp.Data, s)

		if !pager.AdvanceCursor(resp.ResponseFields) {
			break
		}
	}
	ckey := cm.N.GetKeyNameStatic(orgCollectionKeyID)
	clientcache.SaveItemsToCollection(ctx, cm, ObjectType{}, orgs, ckey, ckey, s, true, c.ttlP.TTL(orgTTL))
	return orgs, nil
}

// GetOrganization retrieves a single organization by its UUID
func (c *Client) GetOrganization(ctx context.Context, id uuid.UUID, opts ...Option) (*Organization, error) {
	ctx = request.SetRequestIDIfNotSet(ctx, uuid.Must(uuid.NewV4()))

	options := c.options
	for _, opt := range opts {
		opt.apply(&options)
	}

	cm := clientcache.NewCacheManager(c.cp, c.getCacheKeyNameProvider(uuid.Nil), c.ttlP)
	s := cache.NoLockSentinel
	if !options.bypassCache {
		var v *Organization
		var err error

		v, s, err = clientcache.GetItemFromCache[Organization](ctx, cm, cm.N.GetKeyNameWithID(orgKeyID, id), true, c.ttlP.TTL(orgTTL))
		if err != nil {
			return nil, ucerr.Wrap(err)
		}
		if v != nil {
			return v, nil
		}
	}

	var resp Organization
	if err := c.client.Get(ctx, fmt.Sprintf("/authz/organizations/%s", id), &resp); err != nil {
		return nil, ucerr.Wrap(err)
	}

	clientcache.SaveItemToCache(ctx, cm, resp, s, false, nil)

	return &resp, nil
}

// CreateOrganizationRequest is the request struct to the CreateOrganization endpoint
type CreateOrganizationRequest struct {
	ID     uuid.UUID     `json:"id"`
	Name   string        `json:"name" validate:"notempty"`
	Region region.Region `json:"region"` // this is a UC Region (not an AWS region)
}

// CreateOrganization creates an organization
func (c *Client) CreateOrganization(ctx context.Context, id uuid.UUID, name string, region region.Region) (*Organization, error) {
	ctx = request.SetRequestIDIfNotSet(ctx, uuid.Must(uuid.NewV4()))

	req := CreateOrganizationRequest{
		ID:     id,
		Name:   name,
		Region: region,
	}

	cm := clientcache.NewCacheManager(c.cp, c.getCacheKeyNameProvider(uuid.Nil), c.ttlP)
	org := Organization{BaseModel: ucdb.NewBaseWithID(id)}
	s, err := clientcache.TakeItemLock(ctx, cache.Create, cm, org)
	if err != nil {
		return nil, ucerr.Wrap(err)
	}
	defer clientcache.ReleaseItemLock(ctx, cm, cache.Create, org, s)

	var resp Organization
	if err := c.client.Post(ctx, "/authz/organizations", req, &resp); err != nil {
		return nil, ucerr.Wrap(err)
	}

	clientcache.SaveItemToCache(ctx, cm, resp, s, true, nil)

	return &resp, nil
}

// UpdateOrganizationRequest is the request struct to the UpdateOrganization endpoint
type UpdateOrganizationRequest struct {
	Name   string        `json:"name" validate:"notempty"`
	Region region.Region `json:"region"` // this is a UC Region (not an AWS region)
}

// UpdateOrganization updates an organization
func (c *Client) UpdateOrganization(ctx context.Context, id uuid.UUID, name string, region region.Region) (*Organization, error) {
	ctx = request.SetRequestIDIfNotSet(ctx, uuid.Must(uuid.NewV4()))

	req := UpdateOrganizationRequest{
		Name:   name,
		Region: region,
	}

	cm := clientcache.NewCacheManager(c.cp, c.getCacheKeyNameProvider(uuid.Nil), c.ttlP)
	org := Organization{BaseModel: ucdb.NewBaseWithID(id)}
	s, err := clientcache.TakeItemLock(ctx, cache.Update, cm, org)
	if err != nil {
		return nil, ucerr.Wrap(err)
	}

	var resp Organization
	if err := c.client.Put(ctx, fmt.Sprintf("/authz/organizations/%s", id), req, &resp); err != nil {
		return nil, ucerr.Wrap(err)
	}

	clientcache.SaveItemToCache(ctx, cm, resp, s, true, nil)

	return &resp, nil
}
