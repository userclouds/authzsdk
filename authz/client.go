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

	"github.com/gofrs/uuid"

	"userclouds.com/infra/jsonclient"
	"userclouds.com/infra/pagination"
	"userclouds.com/infra/ucdb"
	"userclouds.com/infra/ucerr"
)

// Client is a client for the authz service
type Client struct {
	client *jsonclient.Client

	// TODO: these should timeout at some point :)
	// TODO: slightly more abstract cache interface here?
	objectTypes  map[string]ObjectType
	mObjectTypes sync.RWMutex

	edgeTypes  map[string]EdgeType
	mEdgeTypes sync.RWMutex

	objects  map[uuid.UUID]Object
	mObjects sync.RWMutex

	edges  map[uuid.UUID]Edge
	mEdges sync.RWMutex
}

// NewClient creates a new authz client
// Web API base URL, e.g. "http://localhost:1234".
func NewClient(url string, opts ...jsonclient.Option) (*Client, error) {
	c := &Client{
		client:      jsonclient.New(strings.TrimSuffix(url, "/"), opts...),
		objectTypes: make(map[string]ObjectType),
		edgeTypes:   make(map[string]EdgeType),
		objects:     make(map[uuid.UUID]Object),
		edges:       make(map[uuid.UUID]Edge),
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

	c.mObjectTypes.Lock()
	defer c.mObjectTypes.Unlock()
	c.objectTypes[resp.TypeName] = resp

	return &resp, nil
}

// FindObjectTypeID resolves an object type name to an ID.
func (c *Client) FindObjectTypeID(ctx context.Context, typeName string) (uuid.UUID, error) {
	c.mObjectTypes.RLock()
	if ot, ok := c.objectTypes[typeName]; ok {
		c.mObjectTypes.RUnlock()
		return ot.ID, nil
	}
	c.mObjectTypes.RUnlock()

	if _, err := c.ListObjectTypes(ctx); err != nil {
		return uuid.Nil, ucerr.Wrap(err)
	}

	// take advantage of the cache update in ListObjectTypes
	objType, ok := c.objectTypes[typeName]
	if !ok {
		return uuid.Nil, ucerr.Errorf("authz object type '%s' not found", typeName)
	}

	return objType.ID, nil
}

// GetObjectType returns an object type by ID.
func (c *Client) GetObjectType(ctx context.Context, id uuid.UUID) (*ObjectType, error) {
	c.mObjectTypes.RLock()
	for _, ot := range c.objectTypes {
		if ot.ID == id {
			c.mObjectTypes.RUnlock()
			cpy := c.objectTypes[ot.TypeName]
			return &cpy, nil
		}
	}
	c.mObjectTypes.RUnlock()

	var resp ObjectType
	if err := c.client.Get(ctx, fmt.Sprintf("/authz/objecttypes/%v", id), &resp); err != nil {
		return nil, ucerr.Wrap(err)
	}

	c.mObjectTypes.Lock()
	c.objectTypes[resp.TypeName] = resp
	c.mObjectTypes.Unlock()

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
	newCache := make(map[string]ObjectType)
	ots := make([]ObjectType, 0)

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

		for _, objType := range resp.Data {
			newCache[objType.TypeName] = objType
		}
		ots = append(ots, resp.Data...)

		if !pager.AdvanceCursor(resp.ResponseFields) {
			break
		}
	}

	// Swap to new cache on success
	c.mObjectTypes.Lock()
	defer c.mObjectTypes.Unlock()
	c.objectTypes = newCache

	return ots, nil
}

// DeleteObjectType deletes an object type by ID.
func (c *Client) DeleteObjectType(ctx context.Context, objectTypeID uuid.UUID) error {
	c.mObjectTypes.Lock()
	for _, v := range c.objectTypes {
		if v.ID == objectTypeID {
			delete(c.objectTypes, v.TypeName)
			break
		}
	}
	c.mObjectTypes.Unlock()

	return ucerr.Wrap(c.client.Delete(ctx, fmt.Sprintf("/authz/objecttypes/%s", objectTypeID), nil))
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

	c.mEdgeTypes.Lock()
	defer c.mEdgeTypes.Unlock()
	c.edgeTypes[resp.TypeName] = resp

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
	c.mEdgeTypes.RLock()
	for _, v := range c.edgeTypes {
		if v.ID == edgeTypeID {
			rv := v
			c.mEdgeTypes.RUnlock()
			return &rv, nil
		}
	}
	c.mEdgeTypes.RUnlock()

	var resp EdgeType
	if err := c.client.Get(ctx, fmt.Sprintf("/authz/edgetypes/%s", edgeTypeID), &resp); err != nil {
		return nil, ucerr.Wrap(err)
	}

	c.mEdgeTypes.Lock()
	defer c.mEdgeTypes.Unlock()
	c.edgeTypes[resp.TypeName] = resp

	return &resp, nil
}

// FindEdgeTypeID resolves an edge type name to an ID.
func (c *Client) FindEdgeTypeID(ctx context.Context, typeName string) (uuid.UUID, error) {
	c.mEdgeTypes.RLock()
	if et, ok := c.edgeTypes[typeName]; ok {
		c.mEdgeTypes.RUnlock()
		return et.ID, nil
	}
	c.mEdgeTypes.RUnlock()

	if _, err := c.ListEdgeTypes(ctx); err != nil {
		return uuid.Nil, ucerr.Wrap(err)
	}

	// take advantage of the fact that ListEdgeTypes updated the cache
	c.mEdgeTypes.RLock()
	defer c.mEdgeTypes.RUnlock()
	edgeType, ok := c.edgeTypes[typeName]

	if !ok {
		return uuid.Nil, ucerr.Errorf("authz edge type '%s' not found", typeName)
	}
	return edgeType.ID, nil
}

// ListEdgeTypesResponse is the paginated response from listing edge types.
type ListEdgeTypesResponse struct {
	Data []EdgeType `json:"data"`
	pagination.ResponseFields
}

// ListEdgeTypes lists all available edge types
func (c *Client) ListEdgeTypes(ctx context.Context) ([]EdgeType, error) {
	// Rebuild the cache while we build up the response
	newCache := make(map[string]EdgeType)
	ets := make([]EdgeType, 0)

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

		for _, edgeType := range resp.Data {
			newCache[edgeType.TypeName] = edgeType
		}
		ets = append(ets, resp.Data...)

		if !pager.AdvanceCursor(resp.ResponseFields) {
			break
		}
	}

	// Swap to new cache on success
	c.mEdgeTypes.Lock()
	defer c.mEdgeTypes.Unlock()
	c.edgeTypes = newCache

	return ets, nil
}

// DeleteEdgeType deletes an edge type by ID.
func (c *Client) DeleteEdgeType(ctx context.Context, edgeTypeID uuid.UUID) error {
	c.mEdgeTypes.Lock()
	for _, v := range c.edgeTypes {
		if v.ID == edgeTypeID {
			delete(c.edgeTypes, v.TypeName)
			break
		}
	}
	c.mEdgeTypes.Unlock()

	return ucerr.Wrap(c.client.Delete(ctx, fmt.Sprintf("/authz/edgetypes/%s", edgeTypeID), nil))
}

// CreateObject creates a new object with a given ID, name, and type.
func (c *Client) CreateObject(ctx context.Context, id, typeID uuid.UUID, alias string) (*Object, error) {
	obj := Object{
		BaseModel: ucdb.NewBaseWithID(id),
		Alias:     alias,
		TypeID:    typeID,
	}
	var resp Object
	if err := c.client.Post(ctx, "/authz/objects", obj, &resp); err != nil {
		return nil, ucerr.Wrap(err)
	}

	c.mObjects.Lock()
	defer c.mObjects.Unlock()
	c.objects[resp.ID] = resp

	return &resp, nil
}

// GetObject returns an object by ID.
func (c *Client) GetObject(ctx context.Context, id uuid.UUID) (*Object, error) {
	c.mObjects.RLock()
	if obj, ok := c.objects[id]; ok {
		c.mObjects.RUnlock()
		return &obj, nil
	}
	c.mObjects.RUnlock()

	var resp Object
	if err := c.client.Get(ctx, fmt.Sprintf("/authz/objects/%s", id), &resp); err != nil {
		return nil, ucerr.Wrap(err)
	}

	c.mObjects.Lock()
	defer c.mObjects.Unlock()
	c.objects[resp.ID] = resp

	return &resp, nil
}

// GetObjectForName returns an object with a given name.
func (c *Client) GetObjectForName(ctx context.Context, typeID uuid.UUID, name string) (*Object, error) {
	c.mObjects.RLock()
	for _, obj := range c.objects {
		if obj.TypeID == typeID && obj.Alias == name {
			c.mObjects.RUnlock()
			return &obj, nil
		}
	}
	c.mObjects.RUnlock()

	// TODO: support a name-based path, e.g. `/authz/objects/<objectname>`
	var resp ListObjectsResponse
	decodeFunc := newPaginatedDecodeFunc(&resp, &resp.Data)
	query := url.Values{}
	query.Add("type_id", typeID.String())
	query.Add("name", name)
	if err := c.client.Get(ctx, fmt.Sprintf("/authz/objects?%s", query.Encode()), nil, jsonclient.CustomDecoder(decodeFunc)); err != nil {
		return nil, ucerr.Wrap(err)
	}

	c.mObjects.Lock()
	defer c.mObjects.Unlock()
	for _, obj := range resp.Data {
		c.objects[obj.ID] = obj
	}

	if len(resp.Data) > 0 {
		return &resp.Data[0], nil
	}
	return nil, ErrObjectNotFound
}

// DeleteObject deletes an object by ID.
func (c *Client) DeleteObject(ctx context.Context, id uuid.UUID) error {
	// TODO: this might be a bit too "understanding" of the server behavior, is
	// there a safer way to separate these responsibilities?
	c.mEdges.Lock()
	for _, e := range c.edges {
		if e.SourceObjectID == id || e.TargetObjectID == id {
			// NB: deleting under a range is explicitly safe in golang
			delete(c.edges, e.ID)
		}
	}
	c.mEdges.Unlock()

	c.mObjects.Lock()
	delete(c.objects, id)
	c.mObjects.Unlock()

	return ucerr.Wrap(c.client.Delete(ctx, fmt.Sprintf("/authz/objects/%s", id), nil))
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

	var resp ListObjectsResponse
	legacyResult := []Object{}
	decodeFunc := newPaginatedDecodeFunc(&resp, &legacyResult)
	query := pager.Query()
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

	c.mObjects.Lock()
	defer c.mObjects.Unlock()
	for _, obj := range resp.Data {
		c.objects[obj.ID] = obj
	}

	return &resp, nil
}

// ListEdgesResponse is the paginated response from listing edges.
type ListEdgesResponse struct {
	Data []Edge `json:"data"`
	pagination.ResponseFields
}

// ListEdgesOnObject lists `limit` edges (relationships) where the given object is a source or target.
func (c *Client) ListEdgesOnObject(ctx context.Context, objectID uuid.UUID, opts ...pagination.Option) (*ListEdgesResponse, error) {
	pager, err := pagination.ApplyOptions(opts...)
	if err != nil {
		return nil, ucerr.Wrap(err)
	}

	query := pager.Query()

	var resp ListEdgesResponse
	if err := c.client.Get(ctx, fmt.Sprintf("/authz/objects/%s/edges?%s", objectID, query.Encode()), &resp); err != nil {
		return nil, ucerr.Wrap(err)
	}

	c.mEdges.Lock()
	defer c.mEdges.Unlock()
	for _, e := range resp.Data {
		c.edges[e.ID] = e
	}

	return &resp, nil
}

// ListEdgesBetweenObjects lists all edges (relationships) with a given source & target objct.
func (c *Client) ListEdgesBetweenObjects(ctx context.Context, sourceObjectID, targetObjectID uuid.UUID) ([]Edge, error) {
	// NB: we don't currently offer any cached reads here because it's hard to know when a "list" is current?
	var resp []Edge
	query := url.Values{}
	query.Add("source_object_id", sourceObjectID.String())
	query.Add("target_object_id", targetObjectID.String())
	if err := c.client.Get(ctx, fmt.Sprintf("/authz/edges?%s", query.Encode()), &resp); err != nil {
		return nil, ucerr.Wrap(err)
	}

	c.mEdges.Lock()
	defer c.mEdges.Unlock()
	for _, e := range resp {
		c.edges[e.ID] = e
	}

	return resp, nil
}

// FindEdge finds an existing edge (relationship) between two objects.
func (c *Client) FindEdge(ctx context.Context, sourceObjectID, targetObjectID, edgeTypeID uuid.UUID) (*Edge, error) {
	c.mEdges.RLock()
	for _, edge := range c.edges {
		if edge.SourceObjectID == sourceObjectID &&
			edge.TargetObjectID == targetObjectID &&
			edge.EdgeTypeID == edgeTypeID {
			rv := edge
			c.mEdges.RUnlock()
			return &rv, nil
		}
	}
	c.mEdges.RUnlock()

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

	c.mEdges.Lock()
	defer c.mEdges.Unlock()
	c.edges[resp[0].ID] = resp[0]

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

	c.mEdges.Lock()
	defer c.mEdges.Unlock()
	c.edges[req.ID] = req

	return &req, nil
}

// DeleteEdge deletes an edge by ID.
func (c *Client) DeleteEdge(ctx context.Context, edgeID uuid.UUID) error {
	c.mEdges.Lock()
	delete(c.edges, edgeID)
	c.mEdges.Unlock()

	return ucerr.Wrap(c.client.Delete(ctx, fmt.Sprintf("/authz/edges/%s", edgeID), nil))
}

// AttributePathNode is a node in a path list from source to target, if CheckAttribute succeeds.
type AttributePathNode struct {
	ObjectID uuid.UUID `json:"object_id"`
	EdgeID   uuid.UUID `json:"edge_id"`
}

// CheckAttributeResponse is returned by the check_attribute endpoint.
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
	if err := c.client.Get(ctx, fmt.Sprintf("/authz/check_attribute?%s", query.Encode()), &resp); err != nil {
		return nil, ucerr.Wrap(err)
	}

	return &resp, nil
}
