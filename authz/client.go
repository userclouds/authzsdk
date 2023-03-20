package authz

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gofrs/uuid"
	"github.com/patrickmn/go-cache"

	"userclouds.com/infra/jsonclient"
	"userclouds.com/infra/pagination"
	"userclouds.com/infra/ucdb"
	"userclouds.com/infra/ucerr"
)

const (
	objTypePrefix   string = "OBJECTTYPE_"
	edgeTypePrefix  string = "EDGETYPE_"
	keyArrrayPrefix string = "KEYS_"
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

	defaultCacheTTL time.Duration = 5 * time.Minute
	gcInterval      time.Duration = 5 * time.Minute
)

// Client is a client for the authz service
type Client struct {
	client *jsonclient.Client

	// Object type cache contains:
	//  ObjTypeID -> ObjType and objTypePrefix + TypeName -> ObjType
	cacheObjTypes *cache.Cache
	// Edge type cache contains:
	//  EdgeTypeID -> EdgeType and edgeTypePrefix + TypeName -> EdgeType
	cacheEdgeTypes *cache.Cache
	// Object cache contains:
	//  ObjectID -> Object and typeID + Object.Alias -> Object
	cacheObjects *cache.Cache
	// Edge cache contains:
	//  ObjectID -> []Edges (all outgoing/incoming)
	//  EdgeID -> Edge
	//  SourceObjID + TargetObjID -> []Edges (edge between source and target objects)
	//  SourceObjID + TargetObjID + EdgeTypeID -> Edge
	//  keyArrrayPrefix + ObjectID -> [] keys (contains all key name in above three mapping that maybe in the cache)
	cacheEdges *cache.Cache

	objTypeTTL  time.Duration
	edgeTypeTTL time.Duration
	objTTL      time.Duration
	edgeTTL     time.Duration

	keysMutex sync.Mutex
}

// NewClient creates a new authz client
// Web API base URL, e.g. "http://localhost:1234".
func NewClient(url string, opts ...jsonclient.Option) (*Client, error) {
	return NewCustomClient(DefaultObjTypeTTL, DefaultEdgeTypeTTL, DefaultObjTTL, DefaultEdgeTTL, url, opts...)
}

// NewCustomClient creates a new authz client with different cache defaults
// Web API base URL, e.g. "http://localhost:1234".
func NewCustomClient(objTypeTTL time.Duration, edgeTypeTTL time.Duration, objTTL time.Duration, edgeTTL time.Duration,
	url string, opts ...jsonclient.Option) (*Client, error) {
	cacheObjTypes := cache.New(defaultCacheTTL, gcInterval)
	cacheEdgeTypes := cache.New(defaultCacheTTL, gcInterval)
	cacheObjects := cache.New(defaultCacheTTL, gcInterval)
	cacheEdges := cache.New(defaultCacheTTL, gcInterval)

	c := &Client{
		client:         jsonclient.New(strings.TrimSuffix(url, "/"), opts...),
		cacheObjTypes:  cacheObjTypes,
		cacheEdgeTypes: cacheEdgeTypes,
		cacheObjects:   cacheObjects,
		cacheEdges:     cacheEdges,
		objTypeTTL:     objTypeTTL,
		edgeTypeTTL:    edgeTypeTTL,
		objTTL:         objTTL,
		edgeTTL:        edgeTTL,
	}

	if err := c.client.ValidateBearerTokenHeader(); err != nil {
		return nil, ucerr.Wrap(err)
	}
	return c, nil
}

// ErrObjectNotFound is returned if an object is not found.
var ErrObjectNotFound = ucerr.New("object not found")

// ErrRelationshipTypeNotFound is returned if a relationship type name
// (e.g. "editor") is not found.
var ErrRelationshipTypeNotFound = ucerr.New("relationship type not found")

// objectTypeKeyName returns key name for [objTypePrefix + TypeName] -> [ObjType] mapping
func objectTypeKeyName(typeName string) string {
	return objTypePrefix + typeName
}

// edgeTypeKeyName returns key name for [edgeTypePrefix + TypeName] -> [EdgeType] mapping
func edgeTypeKeyName(typeName string) string {
	return edgeTypePrefix + typeName
}

// objAliasKeyName returns key name for [TypeID + Alias] -> [Object] mapping
func objAliasKeyName(typeID uuid.UUID, alias string) string {
	return typeID.String() + alias
}

// edgesObjToObj returns key name for [SourceObjID _ TargetObjID] -> [Edge [] ] mapping
func edgesObjToObj(sourceObjID uuid.UUID, targetObjID uuid.UUID) string {
	return fmt.Sprintf("%v_%v", sourceObjID, targetObjID)
}

// edgeFullKeyNameFromEdge returns key name for [SourceObjID _ TargetObjID _ EdgeTypeID] -> [Edge] mapping
func edgeFullKeyNameFromEdge(edge *Edge) string {
	return fmt.Sprintf("%v_%v_%v", edge.SourceObjectID, edge.TargetObjectID.String(), edge.EdgeTypeID)
}

// edgeFullKeyNameFromIDs returns key name for [SourceObjID _ TargetObjID _ EdgeTypeID] -> [Edge] mapping
func edgeFullKeyNameFromIDs(sourceID uuid.UUID, targetID uuid.UUID, typeID uuid.UUID) string {
	return fmt.Sprintf("%v_%v_%v", sourceID, targetID, typeID)
}
func keyArrayKeyName(sourceID uuid.UUID) string {
	return keyArrrayPrefix + sourceID.String()
}

func (c *Client) saveObjectType(objType ObjectType) {
	c.cacheObjTypes.Set(objType.ID.String(), objType, c.objTypeTTL)
	c.cacheObjTypes.Set(objectTypeKeyName(objType.TypeName), objType, c.objTypeTTL)
}

func (c *Client) saveEdgeType(edgeType EdgeType) {
	c.cacheEdgeTypes.Set(edgeType.ID.String(), edgeType, c.edgeTypeTTL)
	c.cacheEdgeTypes.Set(edgeTypeKeyName(edgeType.TypeName), edgeType, c.edgeTypeTTL)
}

func (c *Client) saveObject(obj Object) {
	c.cacheObjects.Set(obj.ID.String(), obj, c.objTTL)
	if obj.Alias != nil {
		c.cacheObjects.Set(objAliasKeyName(obj.TypeID, *(obj.Alias)), obj, c.objTTL)
	}
}

func (c *Client) saveKeyArray(newKeys []string, sourceObjectID uuid.UUID) {
	c.keysMutex.Lock()
	defer c.keysMutex.Unlock()

	if x, found := c.cacheEdges.Get(keyArrayKeyName(sourceObjectID)); found {
		keyNames := x.([]string)
		for _, keyName := range keyNames {
			if _, found := c.cacheEdges.Get(keyName); found {
				newKeys = append(newKeys, keyName)
			}
		}
	}
	c.cacheEdges.Set(keyArrayKeyName(sourceObjectID), newKeys, c.edgeTTL)
}

func (c *Client) deleteKeyArray(sourceObjectID uuid.UUID) {
	c.keysMutex.Lock()
	defer c.keysMutex.Unlock()

	if x, found := c.cacheEdges.Get(keyArrayKeyName(sourceObjectID)); found {
		keyNames := x.([]string)

		for _, keyName := range keyNames {
			if x, found := c.cacheEdges.Get(keyName); found {
				if edge, ok := x.(Edge); ok {
					c.deleteEdgeFromCache(edge)
				} else {
					if edges, ok := x.([]Edge); ok {
						for _, edge := range edges {
							c.deleteEdgeFromCache(edge)
						}
					}
				}
			}
		}
	}
	c.cacheEdges.Delete(keyArrayKeyName(sourceObjectID))
}

func (c *Client) saveEdges(edges []Edge, sourceObjectID uuid.UUID, targetObjectID uuid.UUID) {
	// Make a copy of the edges for caching
	cEdges := make([]Edge, len(edges))
	copy(cEdges, edges)
	keyNames := make([]string, 0, len(edges)+1)
	if targetObjectID != uuid.Nil { // We are only saving edges between sourceObjectID and targetObjectID
		keyName := edgesObjToObj(sourceObjectID, targetObjectID)
		c.cacheEdges.Set(keyName, cEdges, c.edgeTTL)
		keyNames = append(keyNames, keyName)
	} else { // We are saving all edges incoming/outgoing from sourceObjectID
		c.cacheEdges.Set(sourceObjectID.String(), cEdges, c.edgeTTL)
	}
	for _, edge := range cEdges {
		keyNamesEdge := []string{edgeFullKeyNameFromEdge(&edge), edge.ID.String()}
		c.cacheEdges.Set(keyNamesEdge[0], edge, c.objTTL)
		c.cacheEdges.Set(keyNamesEdge[1], edge, c.objTTL)
		keyNames = append(keyNames, keyNamesEdge...)
	}
	c.saveKeyArray(keyNames, sourceObjectID)
}

func (c *Client) saveEdge(edge Edge) {
	// We could also append the edge to the object edges but it requires us to ensure that we don't reset the expiration time
	// We don't clear the KeyArray since we don't need to invalidate individual edges just the sets that now include newly created edge
	c.cacheEdges.Delete(edgesObjToObj(edge.SourceObjectID, edge.TargetObjectID))
	c.cacheEdges.Delete(edge.SourceObjectID.String())

	keyNames := []string{edgeFullKeyNameFromEdge(&edge), edge.ID.String()}
	c.cacheEdges.Set(keyNames[0], edge, c.objTTL)
	c.cacheEdges.Set(keyNames[1], edge, c.objTTL)
	c.saveKeyArray(keyNames, edge.SourceObjectID)
}

// deleteObjectromCache deletes all the cached values/collections in which the object may be present. It assumes that all edges into/out of the object are also
// invalid and need to flushed
func (c *Client) deleteObjectromCache(id uuid.UUID) {
	if x, found := c.cacheObjects.Get(id.String()); found {
		obj := x.(Object)
		c.cacheObjects.Delete(obj.ID.String())
		if obj.Alias != nil {
			c.cacheObjects.Delete(objAliasKeyName(obj.TypeID, *(obj.Alias)))
		}
	}
	// Clear all edges that have object as target or source
	c.deleteKeyArray(id)
}

// deleteEdgeFromCache deletes all the cached values/collections in which the edge may be present
func (c *Client) deleteEdgeFromCache(edge Edge) {
	c.cacheEdges.Delete(edgeFullKeyNameFromEdge(&edge))                          // Clear Source_Target_Type -> Edge mapping
	c.cacheEdges.Delete(edge.ID.String())                                        // Clear EdgeID -> Edge mapping
	c.cacheEdges.Delete(edgesObjToObj(edge.SourceObjectID, edge.TargetObjectID)) // Clear edge set between source and target object
	c.cacheEdges.Delete(edgesObjToObj(edge.TargetObjectID, edge.SourceObjectID)) // Clear edge set between target and source object
	c.cacheEdges.Delete(edge.SourceObjectID.String())                            // Clear edge set for incoming/outgoing for source object
	c.cacheEdges.Delete(edge.TargetObjectID.String())                            // Clear edge set for incoming/outgoing for target object
}

// FlushCache clears all contents of the cache
func (c *Client) FlushCache() {
	c.cacheObjTypes.Flush()
	c.cacheEdgeTypes.Flush()
	c.cacheObjects.Flush()
	c.cacheEdges.Flush()
}

// FlushCacheEdges clears the edge cache only.
func (c *Client) FlushCacheEdges() {
	c.cacheEdges.Flush()
}

// CreateObjectType creates a new type of object for the authz system.
func (c *Client) CreateObjectType(ctx context.Context, id uuid.UUID, typeName string) (*ObjectType, error) {
	req := ObjectType{
		BaseModel: ucdb.NewBaseWithID(id),
		TypeName:  typeName,
	}
	var resp ObjectType
	if err := c.client.Post(ctx, "/authz/objecttypes", req, &resp); err != nil {
		return nil, ucerr.Wrap(err)
	}

	c.saveObjectType(resp)
	return &resp, nil
}

// FindObjectTypeID resolves an object type name to an ID.
func (c *Client) FindObjectTypeID(ctx context.Context, typeName string) (uuid.UUID, error) {
	if x, found := c.cacheObjTypes.Get(objectTypeKeyName(typeName)); found {
		objType := x.(ObjectType)
		return objType.ID, nil
	}

	objTypes, err := c.ListObjectTypes(ctx)
	if err != nil {
		return uuid.Nil, ucerr.Wrap(err)
	}

	if x, found := c.cacheObjTypes.Get(objectTypeKeyName(typeName)); found {
		objType := x.(ObjectType)
		return objType.ID, nil
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
func (c *Client) GetObjectType(ctx context.Context, id uuid.UUID) (*ObjectType, error) {
	if x, found := c.cacheObjTypes.Get(id.String()); found {
		objType := x.(ObjectType)
		return &objType, nil
	}

	var resp ObjectType
	if err := c.client.Get(ctx, fmt.Sprintf("/authz/objecttypes/%v", id), &resp); err != nil {
		return nil, ucerr.Wrap(err)
	}
	c.saveObjectType(resp)
	return &resp, nil
}

// newPaginatedDecodeFunc is a temporary method to smooth over the transition from non-paginated API response
// to paginated API responses.
// TODO: remove this once new production services are pushed (which is necessarily after clients upgrade SDK).
func newPaginatedDecodeFunc(response interface{}, fallbackResponse interface{}) jsonclient.DecodeFunc {
	return func(ctx context.Context, body io.ReadCloser) error {
		b, err := io.ReadAll(body)
		if err != nil {
			return ucerr.Wrap(err)
		}
		err = json.NewDecoder(bytes.NewReader(b)).Decode(response)
		if err != nil {
			// Fallback to legacy format
			if fallbackErr := json.NewDecoder(bytes.NewReader(b)).Decode(fallbackResponse); fallbackErr != nil {
				// Return original error so it's not confusing
				return ucerr.Wrap(err)
			}
			// NOTE: if we use the fallback path, `HasNext` / `HasPrev` defaults to false, which makes sense since there are no more results.
		}
		return nil
	}
}

// ListObjectTypesResponse is the paginated response from listing object types.
type ListObjectTypesResponse struct {
	Data []ObjectType `json:"data"`
	pagination.ResponseFields
}

// ListObjectTypes lists all object types in the system
func (c *Client) ListObjectTypes(ctx context.Context) ([]ObjectType, error) {
	// Rebuild the cache while we build up the response
	c.cacheObjTypes.Flush()
	objTypes := make([]ObjectType, 0)

	// TODO: we should eventually support pagination arguments to this method, but for now we assume
	// there aren't that many object types and just fetch them all.

	pager, err := pagination.ApplyOptions()
	if err != nil {
		return nil, ucerr.Wrap(err)
	}

	for {
		query := pager.Query()

		var resp ListObjectTypesResponse
		if err := c.client.Get(ctx, fmt.Sprintf("/authz/objecttypes?%s", query.Encode()), &resp); err != nil {
			return nil, ucerr.Wrap(err)
		}

		objTypes = append(objTypes, resp.Data...)

		if !pager.AdvanceCursor(resp.ResponseFields) {
			break
		}
	}

	for _, objType := range objTypes {
		c.saveObjectType(objType)
	}
	return objTypes, nil
}

// DeleteObjectType deletes an object type by ID.
func (c *Client) DeleteObjectType(ctx context.Context, objectTypeID uuid.UUID) error {
	if err := c.client.Delete(ctx, fmt.Sprintf("/authz/objecttypes/%s", objectTypeID), nil); err != nil {
		return ucerr.Wrap(err)
	}

	// There are so many potential inconsistencies when object type is deleted so flush the whole cache
	c.FlushCache()
	return nil
}

// CreateEdgeType creates a new type of edge for the authz system.
func (c *Client) CreateEdgeType(ctx context.Context, id uuid.UUID, sourceObjectTypeID, targetObjectTypeID uuid.UUID, typeName string, attributes Attributes) (*EdgeType, error) {
	req := EdgeType{
		BaseModel:          ucdb.NewBaseWithID(id),
		TypeName:           typeName,
		SourceObjectTypeID: sourceObjectTypeID,
		TargetObjectTypeID: targetObjectTypeID,
		Attributes:         attributes,
	}

	var resp EdgeType
	if err := c.client.Post(ctx, "/authz/edgetypes", req, &resp); err != nil {
		return nil, ucerr.Wrap(err)
	}

	c.saveEdgeType(resp)
	return &resp, nil
}

// UpdateEdgeType updates an existing edge type in the authz system.
func (c *Client) UpdateEdgeType(ctx context.Context, id uuid.UUID, sourceObjectTypeID, targetObjectTypeID uuid.UUID, typeName string, attributes Attributes) (*EdgeType, error) {
	// TODO: use PUT/PATCH for the update operation instead
	et, err := c.CreateEdgeType(ctx, id, sourceObjectTypeID, targetObjectTypeID, typeName, attributes)
	return et, ucerr.Wrap(err)
}

// GetEdgeType gets an edge type (relationship) by its type ID.
func (c *Client) GetEdgeType(ctx context.Context, edgeTypeID uuid.UUID) (*EdgeType, error) {
	if x, found := c.cacheEdgeTypes.Get(edgeTypeID.String()); found {
		edgeType := x.(EdgeType)
		return &edgeType, nil
	}

	var resp EdgeType
	if err := c.client.Get(ctx, fmt.Sprintf("/authz/edgetypes/%s", edgeTypeID), &resp); err != nil {
		return nil, ucerr.Wrap(err)
	}
	c.saveEdgeType(resp)
	return &resp, nil
}

// FindEdgeTypeID resolves an edge type name to an ID.
func (c *Client) FindEdgeTypeID(ctx context.Context, typeName string) (uuid.UUID, error) {
	if x, found := c.cacheEdgeTypes.Get(edgeTypeKeyName(typeName)); found {
		edgeType := x.(EdgeType)
		return edgeType.ID, nil
	}

	edgeTypes, err := c.ListEdgeTypes(ctx)
	if err != nil {
		return uuid.Nil, ucerr.Wrap(err)
	}

	if x, found := c.cacheEdgeTypes.Get(edgeTypeKeyName(typeName)); found {
		edgeType := x.(EdgeType)
		return edgeType.ID, nil
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

// ListEdgeTypes lists all available edge types
func (c *Client) ListEdgeTypes(ctx context.Context) ([]EdgeType, error) {
	// Rebuild the cache while we build up the response
	c.cacheEdgeTypes.Flush()
	edgeTypes := make([]EdgeType, 0)

	// TODO: we should eventually support pagination arguments to this method, but for now we assume
	// there aren't that many edge types and just fetch them all.

	pager, err := pagination.ApplyOptions()
	if err != nil {
		return nil, ucerr.Wrap(err)
	}

	for {
		query := pager.Query()

		var resp ListEdgeTypesResponse
		if err := c.client.Get(ctx, fmt.Sprintf("/authz/edgetypes?%s", query.Encode()), &resp); err != nil {
			return nil, ucerr.Wrap(err)
		}

		edgeTypes = append(edgeTypes, resp.Data...)

		if !pager.AdvanceCursor(resp.ResponseFields) {
			break
		}
	}

	for _, edgeType := range edgeTypes {
		c.saveEdgeType(edgeType)
	}
	return edgeTypes, nil
}

// DeleteEdgeType deletes an edge type by ID.
func (c *Client) DeleteEdgeType(ctx context.Context, edgeTypeID uuid.UUID) error {
	if err := c.client.Delete(ctx, fmt.Sprintf("/authz/edgetypes/%s", edgeTypeID), nil); err != nil {
		return ucerr.Wrap(err)
	}
	// There are so many potential inconsistencies when edge type is deleted so flush the whole cache
	c.FlushCache()
	return nil
}

// CreateObject creates a new object with a given ID, name, and type.
func (c *Client) CreateObject(ctx context.Context, id, typeID uuid.UUID, alias string) (*Object, error) {
	obj := Object{
		BaseModel: ucdb.NewBaseWithID(id),
		Alias:     &alias,
		TypeID:    typeID,
	}
	if alias == "" { // TODO this avoids a breaking API change bit it introduces a change in contract in being able to store multiple objects with "" alias
		obj.Alias = nil
	}
	var resp Object
	if err := c.client.Post(ctx, "/authz/objects", obj, &resp); err != nil {
		return nil, ucerr.Wrap(err)
	}

	c.saveObject(obj)
	return &resp, nil
}

// GetObject returns an object by ID.
func (c *Client) GetObject(ctx context.Context, id uuid.UUID) (*Object, error) {
	if x, found := c.cacheObjects.Get(id.String()); found {
		obj := x.(Object)
		return &obj, nil
	}

	var resp Object
	if err := c.client.Get(ctx, fmt.Sprintf("/authz/objects/%s", id), &resp); err != nil {
		return nil, ucerr.Wrap(err)
	}

	c.saveObject(resp)
	return &resp, nil
}

// GetObjectForName returns an object with a given name.
func (c *Client) GetObjectForName(ctx context.Context, typeID uuid.UUID, name string) (*Object, error) {
	if x, found := c.cacheObjects.Get(objAliasKeyName(typeID, name)); found {
		edgeType := x.(Object)
		return &edgeType, nil
	}

	// TODO: support a name-based path, e.g. `/authz/objects/<objectname>`
	var resp ListObjectsResponse
	decodeFunc := newPaginatedDecodeFunc(&resp, &resp.Data)
	query := url.Values{}
	query.Add("type_id", typeID.String())
	query.Add("name", name)
	if err := c.client.Get(ctx, fmt.Sprintf("/authz/objects?%s", query.Encode()), nil, jsonclient.CustomDecoder(decodeFunc)); err != nil {
		return nil, ucerr.Wrap(err)
	}

	if len(resp.Data) > 0 {
		c.saveObject(resp.Data[0])
		return &resp.Data[0], nil
	}
	return nil, ErrObjectNotFound
}

// DeleteObject deletes an object by ID.
func (c *Client) DeleteObject(ctx context.Context, id uuid.UUID) error {
	c.deleteObjectromCache(id)
	return ucerr.Wrap(c.client.Delete(ctx, fmt.Sprintf("/authz/objects/%s", id), nil))
}

// DeleteEdgesByObject deletes all edges going in or  out of an object by ID.
func (c *Client) DeleteEdgesByObject(ctx context.Context, id uuid.UUID) error {
	if _, found := c.cacheEdges.Get(id.String()); found {
		c.cacheEdges.Delete(id.String())
		c.deleteKeyArray(id)
	}
	return ucerr.Wrap(c.client.Delete(ctx, fmt.Sprintf("/authz/objects/%s/edges", id), nil))
}

// ListObjectsResponse represents a paginated response from listing objects.
type ListObjectsResponse struct {
	Data []Object `json:"data"`
	pagination.ResponseFields
}

// TODO: get rid of sort.Interface code when the legacy path in ListObjects goes away
// Len implements sort.Interface
func (r ListObjectsResponse) Len() int {
	return len(r.Data)
}

// Swap implements sort.Interface
func (r ListObjectsResponse) Swap(left, right int) {
	tmp := r.Data[left]
	r.Data[left] = r.Data[right]
	r.Data[right] = tmp
}

// Less implements sort.Interface
func (r ListObjectsResponse) Less(left, right int) bool {
	return r.Data[left].ID.String() < r.Data[right].ID.String()
}

// ListObjects lists `limit` objects in sorted order with pagination, starting after a given ID (or uuid.Nil to start from the beginning).
func (c *Client) ListObjects(ctx context.Context, opts ...pagination.Option) (*ListObjectsResponse, error) {
	pager, err := pagination.ApplyOptions(opts...)
	if err != nil {
		return nil, ucerr.Wrap(err)
	}
	return c.ListObjectsFromQuery(ctx, pager.Query(), opts...)
}

// ListObjectsFromQuery takes in a query that can handle filters passed from console as well as the default method.
func (c *Client) ListObjectsFromQuery(ctx context.Context, query url.Values, opts ...pagination.Option) (*ListObjectsResponse, error) {
	var resp ListObjectsResponse
	legacyResult := []Object{}
	decodeFunc := newPaginatedDecodeFunc(&resp, &legacyResult)
	pager, err := pagination.ApplyOptions(opts...)
	if err != nil {
		return nil, ucerr.Wrap(err)
	}
	if err := c.client.Get(ctx, fmt.Sprintf("/authz/objects?%s", query.Encode()), nil, jsonclient.CustomDecoder(decodeFunc)); err != nil {
		return nil, ucerr.Wrap(err)
	}

	if numObjects := len(legacyResult); numObjects > 0 {
		cursorMaker := func(o Object) pagination.Cursor {
			return pagination.Cursor(fmt.Sprintf("id:%v", o.ID))
		}

		// We got a legacy response that's not paginated, so fix it on the client.
		// NOTE: it's obviously not efficient to "re-paginate" it but this makes it easier
		// to test the client behavior before/after the server change.
		// TODO: this code is not going to perform well longer term, but it's very temporary.
		// TODO: remove this code (and "COMPAT" methods) once we support more advanced filtering/sorting/traversal since it's not worth keeping.
		resp.Data = legacyResult
		sort.Sort(resp)
		firstElem := numObjects
		for i := range resp.Data {
			if string(cursorMaker(resp.Data[i])) > string(pager.GetCursor()) {
				firstElem = i
				break
			}
		}
		lastElem := firstElem + pager.GetLimit()
		if lastElem < numObjects {
			resp.HasNext = true
			resp.Next = cursorMaker(resp.Data[lastElem-1])
		} else if lastElem > numObjects {
			lastElem = numObjects
		}
		resp.Data = resp.Data[firstElem:lastElem]
	}

	for _, obj := range resp.Data {
		c.saveObject(obj)
	}

	return &resp, nil
}

// ListEdgesResponse is the paginated response from listing edges.
type ListEdgesResponse struct {
	Data []Edge `json:"data"`
	pagination.ResponseFields
}

// ListEdges lists `limit` edges.
func (c *Client) ListEdges(ctx context.Context, opts ...pagination.Option) (*ListEdgesResponse, error) {
	pager, err := pagination.ApplyOptions(opts...)
	if err != nil {
		return nil, ucerr.Wrap(err)
	}

	query := pager.Query()

	var resp ListEdgesResponse
	if err := c.client.Get(ctx, fmt.Sprintf("/authz/objects/edges?%s", query.Encode()), &resp); err != nil {
		return nil, ucerr.Wrap(err)
	}
	return &resp, nil
}

// ListEdgesOnObject lists `limit` edges (relationships) where the given object is a source or target.
func (c *Client) ListEdgesOnObject(ctx context.Context, objectID uuid.UUID, opts ...pagination.Option) (*ListEdgesResponse, error) {
	pager, err := pagination.ApplyOptions(opts...)
	if err != nil {
		return nil, ucerr.Wrap(err)
	}

	query := pager.Query()

	if x, found := c.cacheEdges.Get(objectID.String()); found {
		edges := x.([]Edge)
		// If the client requests smaller pages than what is stored in the cache - don't use the cache
		if len(edges) <= pager.GetLimit() {
			resp := ListEdgesResponse{Data: x.([]Edge), ResponseFields: pagination.ResponseFields{HasNext: false}}
			return &resp, nil
		}
	}

	var resp ListEdgesResponse
	if err := c.client.Get(ctx, fmt.Sprintf("/authz/objects/%s/edges?%s", objectID, query.Encode()), &resp); err != nil {
		return nil, ucerr.Wrap(err)
	}

	// Only cache the response if it fits on one page
	if !resp.HasNext && !resp.HasPrev {
		c.saveEdges(resp.Data, objectID, uuid.Nil)
	}
	return &resp, nil
}

// ListEdgesBetweenObjects lists all edges (relationships) with a given source & target objct.
func (c *Client) ListEdgesBetweenObjects(ctx context.Context, sourceObjectID, targetObjectID uuid.UUID) ([]Edge, error) {
	// If edges for source object are in the cache for all edges per object - filter them by target
	if x, found := c.cacheEdges.Get(sourceObjectID.String()); found {
		edges := x.([]Edge)
		filteredEdges := make([]Edge, 0)
		for _, edge := range edges {
			if edge.TargetObjectID == targetObjectID {
				filteredEdges = append(filteredEdges, edge)
			}
		}
		return filteredEdges, nil
	}
	// If the edges are in the cache by source->target - the value can be returned directly
	if x, found := c.cacheEdges.Get(edgesObjToObj(sourceObjectID, targetObjectID)); found {
		edges := x.([]Edge)
		return edges, nil
	}

	// NB: we don't currently offer any cached reads here because it's hard to know when a "list" is current?
	var resp []Edge
	query := url.Values{}
	query.Add("source_object_id", sourceObjectID.String())
	query.Add("target_object_id", targetObjectID.String())
	if err := c.client.Get(ctx, fmt.Sprintf("/authz/edges?%s", query.Encode()), &resp); err != nil {
		return nil, ucerr.Wrap(err)
	}

	c.saveEdges(resp, sourceObjectID, targetObjectID)
	return resp, nil
}

// GetEdge returns an edge by ID.
func (c *Client) GetEdge(ctx context.Context, id uuid.UUID) (*Edge, error) {
	if x, found := c.cacheEdges.Get(id.String()); found {
		edge := x.(Edge)
		return &edge, nil
	}

	var resp Edge
	if err := c.client.Get(ctx, fmt.Sprintf("/authz/edges/%s", id), &resp); err != nil {
		return nil, ucerr.Wrap(err)
	}

	c.saveEdge(resp)

	return &resp, nil
}

// FindEdge finds an existing edge (relationship) between two objects.
func (c *Client) FindEdge(ctx context.Context, sourceObjectID, targetObjectID, edgeTypeID uuid.UUID) (*Edge, error) {
	// If the edges are in the cache by source->target - iterate over that set first
	if x, found := c.cacheEdges.Get(edgesObjToObj(sourceObjectID, targetObjectID)); found {
		edges := x.([]Edge)
		for _, edge := range edges {
			if edge.EdgeTypeID == edgeTypeID {
				return &edge, nil
			}
		}
		// In theory we could return NotFound here but this is a rare enough case that it makes sense to try the server
	}

	if x, found := c.cacheEdges.Get(sourceObjectID.String()); found {
		edges := x.([]Edge)
		for _, edge := range edges {
			if edge.TargetObjectID == targetObjectID && edge.EdgeTypeID == edgeTypeID {
				return &edge, nil
			}
		}
	}

	if x, found := c.cacheEdges.Get(edgeFullKeyNameFromIDs(sourceObjectID, targetObjectID, edgeTypeID)); found {
		edge := x.(Edge)
		return &edge, nil
	}

	var resp []Edge
	query := url.Values{}
	query.Add("source_object_id", sourceObjectID.String())
	query.Add("target_object_id", targetObjectID.String())
	query.Add("edge_type_id", edgeTypeID.String())
	if err := c.client.Get(ctx, fmt.Sprintf("/authz/edges?%s", query.Encode()), &resp); err != nil {
		return nil, ucerr.Wrap(err)
	}
	if len(resp) != 1 {
		return nil, ucerr.Errorf("expected 1 edge from FindEdge, got %d", len(resp))
	}

	c.saveEdge(resp[0])
	return &resp[0], nil
}

// CreateEdge creates an edge (relationship) between two objects.
func (c *Client) CreateEdge(ctx context.Context, id, sourceObjectID, targetObjectID, edgeTypeID uuid.UUID) (*Edge, error) {
	req := Edge{
		BaseModel:      ucdb.NewBaseWithID(id),
		EdgeTypeID:     edgeTypeID,
		SourceObjectID: sourceObjectID,
		TargetObjectID: targetObjectID,
	}

	if err := c.client.Post(ctx, "/authz/edges", req, &req); err != nil {
		return nil, ucerr.Wrap(err)
	}

	c.saveEdge(req)

	// Clear edge set for incoming/outgoing for source and target objects
	c.cacheEdges.Delete(sourceObjectID.String())
	c.cacheEdges.Delete(targetObjectID.String())
	c.cacheEdges.Delete(edgesObjToObj(sourceObjectID, targetObjectID))
	c.cacheEdges.Delete(edgesObjToObj(targetObjectID, sourceObjectID))

	return &req, nil
}

// DeleteEdge deletes an edge by ID.
func (c *Client) DeleteEdge(ctx context.Context, edgeID uuid.UUID) error {
	if x, found := c.cacheEdges.Get(edgeID.String()); found {
		edge := x.(Edge)
		c.deleteEdgeFromCache(edge)
	}

	return ucerr.Wrap(c.client.Delete(ctx, fmt.Sprintf("/authz/edges/%s", edgeID), nil))
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
func (c *Client) CheckAttribute(ctx context.Context, sourceObjectID, targetObjectID uuid.UUID, attributeName string) (*CheckAttributeResponse, error) {
	var resp CheckAttributeResponse
	query := url.Values{}
	query.Add("source_object_id", sourceObjectID.String())
	query.Add("target_object_id", targetObjectID.String())
	query.Add("attribute", attributeName)
	if err := c.client.Get(ctx, fmt.Sprintf("/authz/checkattribute?%s", query.Encode()), &resp); err != nil {
		return nil, ucerr.Wrap(err)
	}

	return &resp, nil
}

// ListAttributes returns a list of attributes that the source object has on the target object.
func (c *Client) ListAttributes(ctx context.Context, sourceObjectID, targetObjectID uuid.UUID) ([]string, error) {
	var resp []string
	query := url.Values{}
	query.Add("source_object_id", sourceObjectID.String())
	query.Add("target_object_id", targetObjectID.String())
	if err := c.client.Get(ctx, fmt.Sprintf("/authz/listattributes?%s", query.Encode()), &resp); err != nil {
		return nil, ucerr.Wrap(err)
	}

	return resp, nil
}

// ListObjectsReachableWithAttribute returns a list of object IDs of a certain type that are reachable from the source object with the given attribute
func (c *Client) ListObjectsReachableWithAttribute(ctx context.Context, sourceObjectID uuid.UUID, targetObjectTypeID uuid.UUID, attributeName string) ([]uuid.UUID, error) {
	var resp []uuid.UUID
	query := url.Values{}
	query.Add("source_object_id", sourceObjectID.String())
	query.Add("target_object_type_id", targetObjectTypeID.String())
	query.Add("attribute", attributeName)
	if err := c.client.Get(ctx, fmt.Sprintf("/authz/listobjectsreachablewithattribute?%s", query.Encode()), &resp); err != nil {
		return nil, ucerr.Wrap(err)
	}

	return resp, nil
}

// ListOrganizationsResponse is the response from the ListOrganizations endpoint.
type ListOrganizationsResponse struct {
	Data []Organization `json:"data"`
	pagination.ResponseFields
}

// ListOrganizations lists all organizations for a tenant
func (c *Client) ListOrganizations(ctx context.Context) ([]Organization, error) {

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

		if !pager.AdvanceCursor(resp.ResponseFields) {
			break
		}
	}

	return orgs, nil
}
