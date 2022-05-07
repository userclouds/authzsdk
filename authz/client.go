package authz

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"sync"

	"github.com/gofrs/uuid"

	"userclouds.com/infra/jsonclient"
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
	if err := c.client.RefreshBearerToken(); err != nil {
		return nil, ucerr.Wrap(err)
	}

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
	if err := c.client.RefreshBearerToken(); err != nil {
		return uuid.Nil, ucerr.Wrap(err)
	}

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
	id := c.objectTypes[typeName].ID
	if id == uuid.Nil {
		return uuid.Nil, ucerr.Errorf("authz object type '%s' not found", typeName)
	}

	return id, nil
}

// ListObjectTypes lists all object types in the system
func (c *Client) ListObjectTypes(ctx context.Context) ([]ObjectType, error) {
	if err := c.client.RefreshBearerToken(); err != nil {
		return nil, ucerr.Wrap(err)
	}

	var resp []ObjectType
	if err := c.client.Get(ctx, "/authz/objecttypes", &resp); err != nil {
		return nil, ucerr.Wrap(err)
	}

	// reset the cache
	newCache := make(map[string]ObjectType)
	for _, objType := range resp {
		newCache[objType.TypeName] = objType
	}
	c.mObjectTypes.Lock()
	defer c.mObjectTypes.Unlock()
	c.objectTypes = newCache

	return resp, nil
}

// CreateEdgeType creates a new type of edge for the authz system.
func (c *Client) CreateEdgeType(ctx context.Context, id uuid.UUID, sourceObjectTypeID, targetObjectTypeID uuid.UUID, typeName string) (*EdgeType, error) {
	if err := c.client.RefreshBearerToken(); err != nil {
		return nil, ucerr.Wrap(err)
	}

	req := EdgeType{
		BaseModel:          ucdb.NewBaseWithID(id),
		TypeName:           typeName,
		SourceObjectTypeID: sourceObjectTypeID,
		TargetObjectTypeID: targetObjectTypeID,
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

// GetEdgeType gets an edge type (relationship) by its type ID.
func (c *Client) GetEdgeType(ctx context.Context, edgeTypeID uuid.UUID) (*EdgeType, error) {
	if err := c.client.RefreshBearerToken(); err != nil {
		return nil, ucerr.Wrap(err)
	}

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
	if err := c.client.RefreshBearerToken(); err != nil {
		return uuid.Nil, ucerr.Wrap(err)
	}

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
	id := c.edgeTypes[typeName].ID

	if id == uuid.Nil {
		return uuid.Nil, ucerr.Errorf("authz edge type '%s' not found", typeName)
	}
	return id, nil
}

// ListEdgeTypes lists all available edge types
func (c *Client) ListEdgeTypes(ctx context.Context) ([]EdgeType, error) {
	if err := c.client.RefreshBearerToken(); err != nil {
		return nil, ucerr.Wrap(err)
	}

	var resp []EdgeType
	if err := c.client.Get(ctx, "/authz/edgetypes", &resp); err != nil {
		return nil, ucerr.Wrap(err)
	}

	newCache := make(map[string]EdgeType) // clear and reset cache
	for _, edgeType := range resp {
		newCache[edgeType.TypeName] = edgeType
	}

	c.mEdgeTypes.Lock()
	defer c.mEdgeTypes.Unlock()
	c.edgeTypes = newCache

	return resp, nil
}

// CreateObject creates a new object with a given ID, name, and type.
func (c *Client) CreateObject(ctx context.Context, id, typeID uuid.UUID, alias string) (*Object, error) {
	if err := c.client.RefreshBearerToken(); err != nil {
		return nil, ucerr.Wrap(err)
	}

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
	if err := c.client.RefreshBearerToken(); err != nil {
		return nil, ucerr.Wrap(err)
	}

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
	if err := c.client.RefreshBearerToken(); err != nil {
		return nil, ucerr.Wrap(err)
	}

	c.mObjects.RLock()
	for _, obj := range c.objects {
		if obj.TypeID == typeID && obj.Alias == name {
			c.mObjects.RUnlock()
			return &obj, nil
		}
	}
	c.mObjects.RUnlock()

	// TODO: support a name-based path, e.g. `/authz/objects/<objectname>`
	var resp []Object
	query := url.Values{}
	query.Add("type_id", typeID.String())
	query.Add("name", name)
	if err := c.client.Get(ctx, fmt.Sprintf("/authz/objects?%s", query.Encode()), &resp); err != nil {
		return nil, ucerr.Wrap(err)
	}

	c.mObjects.Lock()
	defer c.mObjects.Unlock()
	for _, obj := range resp {
		c.objects[obj.ID] = obj
	}

	if len(resp) > 0 {
		return &resp[0], nil
	}
	return nil, ErrObjectNotFound
}

// DeleteObject deletes an object by ID.
func (c *Client) DeleteObject(ctx context.Context, id uuid.UUID) error {
	if err := c.client.RefreshBearerToken(); err != nil {
		return ucerr.Wrap(err)
	}

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

	return ucerr.Wrap(c.client.Delete(ctx, fmt.Sprintf("/authz/objects/%s", id)))
}

// ListObjects lists all the objects
func (c *Client) ListObjects(ctx context.Context) ([]Object, error) {
	if err := c.client.RefreshBearerToken(); err != nil {
		return nil, ucerr.Wrap(err)
	}

	var resp []Object
	if err := c.client.Get(ctx, "/authz/objects", &resp); err != nil {
		return nil, ucerr.Wrap(err)
	}

	newCache := make(map[uuid.UUID]Object)
	for _, obj := range resp {
		newCache[obj.ID] = obj
	}

	c.mObjects.Lock()
	defer c.mObjects.Unlock()
	c.objects = newCache

	return resp, nil
}

// ListEdges lists all edges (relationships) where the given object
// is a source or target.
func (c *Client) ListEdges(ctx context.Context, objectID uuid.UUID) ([]Edge, error) {
	if err := c.client.RefreshBearerToken(); err != nil {
		return nil, ucerr.Wrap(err)
	}

	// NB: we don't currently offer any cached reads here because it's hard to know when a "list" is current?
	var resp []Edge
	if err := c.client.Get(ctx, fmt.Sprintf("/authz/objects/%s/edges", objectID), &resp); err != nil {
		return nil, ucerr.Wrap(err)
	}

	c.mEdges.Lock()
	defer c.mEdges.Unlock()
	for _, e := range resp {
		c.edges[e.ID] = e
	}

	return resp, nil
}

// ListEdgesBetweenObjects lists all edges (relationships) with a given source & target objct.
func (c *Client) ListEdgesBetweenObjects(ctx context.Context, sourceObjectID, targetObjectID uuid.UUID) ([]Edge, error) {
	if err := c.client.RefreshBearerToken(); err != nil {
		return nil, ucerr.Wrap(err)
	}

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
	if err := c.client.RefreshBearerToken(); err != nil {
		return nil, ucerr.Wrap(err)
	}

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
func (c *Client) CreateEdge(ctx context.Context, id, sourceObjectID, targetObjectID, edgeTypeID uuid.UUID) (uuid.UUID, error) {
	if err := c.client.RefreshBearerToken(); err != nil {
		return uuid.Nil, ucerr.Wrap(err)
	}

	req := Edge{
		BaseModel:      ucdb.NewBaseWithID(id),
		EdgeTypeID:     edgeTypeID,
		SourceObjectID: sourceObjectID,
		TargetObjectID: targetObjectID,
	}

	if err := c.client.Post(ctx, "/authz/edges", req, &req); err != nil {
		return uuid.Nil, ucerr.Wrap(err)
	}

	c.mEdges.Lock()
	defer c.mEdges.Unlock()
	c.edges[req.ID] = req

	return req.ID, nil
}

// DeleteEdge deletes an edge by ID.
func (c *Client) DeleteEdge(ctx context.Context, edgeID uuid.UUID) error {
	if err := c.client.RefreshBearerToken(); err != nil {
		return ucerr.Wrap(err)
	}

	c.mEdges.Lock()
	delete(c.edges, edgeID)
	c.mEdges.Unlock()

	return ucerr.Wrap(c.client.Delete(ctx, fmt.Sprintf("/authz/edges/%s", edgeID)))
}
