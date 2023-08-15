package authz

import (
	"fmt"
	"time"

	"github.com/gofrs/uuid"

	clientcache "userclouds.com/infra/cache/client"
)

const (
	objTypePrefix               = "OBJTYPE"     // Primary key for object type
	objTypeCollectionKeyString  = "OBJTYPECOL"  // Global collection for object type
	edgeTypePrefix              = "EDGETYPE"    // Primary key for edge type
	edgeTypeCollectionKeyString = "EDGETYPECOL" // Global collection for edge type
	objPrefix                   = "OBJ"         // Primary key for object
	objCollectionKeyString      = "OBJCOL"      // Global collection for object
	objEdgeCollection           = "OBJEDGES"    // Per object collection of all in/out edges
	perObjectEdgesPrefix        = "E"           // Per object collection of source/target edges
	perObjectPathPrefix         = "P"           // Per object collection containing path for a particular source/target/attribute
	edgePrefix                  = "EDGE"        // Primary key for edge
	edgeCollectionKeyString     = "EDGECOL"     // Global collection for edge
	orgPrefix                   = "ORG"         // Primary key for organization
	orgCollectionKeyString      = "ORGCOL"      // Global collection for organizations
	dependencylPrefix           = "DEP"         // Share dependency key prefix among all items
)

// authzCacheNameProvider is the base implementation of the CacheNameProvider interface
type authzCacheNameProvider struct {
	basePrefix string // Base prefix for all keys TenantID_OrgID
}

// newAuthZCacheNameProvider creates a new BasesCacheNameProvider
func newAuthZCacheNameProvider(basePrefix string) *authzCacheNameProvider {
	return &authzCacheNameProvider{basePrefix: basePrefix}
}

const (
	objTypeKeyID            = "ObjTypeKeyID"
	edgeTypeKeyID           = "EdgeTypeKeyID"
	objectKeyID             = "ObjectKeyID"
	edgeKeyID               = "EdgeKeyID"
	orgKeyID                = "OrgKeyID"
	edgeFullKeyID           = "EdgeFullKeyNameID"
	objTypeNameKeyID        = "ObjectTypeKeyNameID"
	objEdgesKeyID           = "ObjectEdgesKeyID"
	edgeTypeNameKeyID       = "EdgeTypeKeyNameID"
	objAliasNameKeyID       = "ObjAliasKeyNameID"
	edgesObjToObjID         = "EdgesObjToObjID"
	dependencyKeyID         = "DependencyKeyID"
	objTypeCollectionKeyID  = "ObjTypeCollectionKeyID"
	edgeTypeCollectionKeyID = "EdgeTypeCollectionKeyID"
	objCollectionKeyID      = "ObjCollectionKeyID"
	edgeCollectionKeyID     = "EdgeCollectionKeyID"
	orgCollectionKeyID      = "OrgCollectionKeyID"
	attributePathObjToObjID = "AttributePathObjToObjID"
)

// GetKeyNameForWithID is a shortcut for GetKeyName with a single uuid ID component
func (c *authzCacheNameProvider) GetKeyNameStatic(id clientcache.CacheKeyNameID) clientcache.CacheKey {
	return c.GetKeyName(id, []string{})
}

// GetKeyNameForWithID is a shortcut for GetKeyName with a single uuid ID component
func (c *authzCacheNameProvider) GetKeyNameWithID(id clientcache.CacheKeyNameID, itemID uuid.UUID) clientcache.CacheKey {
	return c.GetKeyName(id, []string{itemID.String()})
}

// GetKeyNameForWithID is a shortcut for GetKeyName with a single uuid ID component
func (c *authzCacheNameProvider) GetKeyNameWithString(id clientcache.CacheKeyNameID, itemName string) clientcache.CacheKey {
	return c.GetKeyName(id, []string{itemName})
}

// GetKeyName gets the key name for the given key name ID and components
func (c *authzCacheNameProvider) GetKeyName(id clientcache.CacheKeyNameID, components []string) clientcache.CacheKey {
	switch id {
	case objTypeKeyID:
		return c.objectTypeKey(components[0])
	case edgeTypeKeyID:
		return c.edgeTypeKey(components[0])
	case objectKeyID:
		return c.objectKey(components[0])
	case edgeKeyID:
		return c.edgeKey(components[0])
	case orgKeyID:
		return c.orgKey(components[0])
	case objTypeNameKeyID:
		return c.objectTypeKeyName(components[0])
	case edgeTypeNameKeyID:
		return c.edgeTypeKeyName(components[0])
	case objAliasNameKeyID:
		return c.objAliasKeyName(components[0], components[1])
	case edgesObjToObjID:
		return c.edgesObjToObj(components[0], components[1])

	case objTypeCollectionKeyID:
		return c.objTypeCollectionKey()
	case edgeTypeCollectionKeyID:
		return c.edgeTypeCollectionKey()
	case objCollectionKeyID:
		return c.objCollectionKey()
	case edgeCollectionKeyID:
		return c.edgeCollectionKey()
	case orgCollectionKeyID:
		return c.orgCollectionKey()
	case objEdgesKeyID:
		return c.objectEdgesKey(components[0])
	case dependencyKeyID:
		return c.dependencyKey(components[0])
	case edgeFullKeyID:
		return c.edgeFullKeyNameFromIDs(components[0], components[1], components[2])
	case attributePathObjToObjID:
		return c.attributePathObjToObj(components[0], components[1], components[2])
	}
	return ""
}

// objectTypeKey primary key for object type
func (c *authzCacheNameProvider) objectTypeKey(id string) clientcache.CacheKey {
	return clientcache.CacheKey(fmt.Sprintf("%v_%v_%v", c.basePrefix, objTypePrefix, id))
}

// edgeTypeKey primary key for edge type
func (c *authzCacheNameProvider) edgeTypeKey(id string) clientcache.CacheKey {
	return clientcache.CacheKey(fmt.Sprintf("%v_%v_%v", c.basePrefix, edgeTypePrefix, id))
}

// objectKey primary key for object
func (c *authzCacheNameProvider) objectKey(id string) clientcache.CacheKey {
	return clientcache.CacheKey(fmt.Sprintf("%v_%v_%v", c.basePrefix, objPrefix, id))
}

// edgeKey primary key for edge
func (c *authzCacheNameProvider) edgeKey(id string) clientcache.CacheKey {
	return clientcache.CacheKey(fmt.Sprintf("%v_%v_%v", c.basePrefix, edgePrefix, id))
}

// orgKey primary key for edge
func (c *authzCacheNameProvider) orgKey(id string) clientcache.CacheKey {
	return clientcache.CacheKey(fmt.Sprintf("%v_%v_%v", c.basePrefix, orgPrefix, id))
}

// objectTypeKeyName returns secondary key name for [objTypePrefix + TypeName] -> [ObjType] mapping
func (c *authzCacheNameProvider) objectTypeKeyName(typeName string) clientcache.CacheKey {
	return clientcache.CacheKey(fmt.Sprintf("%v_%v_%v", c.basePrefix, objTypePrefix, typeName))
}

// objectEdgesKey returns key name for per object edges collection
func (c *authzCacheNameProvider) objectEdgesKey(id string) clientcache.CacheKey {
	return clientcache.CacheKey(fmt.Sprintf("%v_%v_%v", c.basePrefix, objEdgeCollection, id))
}

// edgeTypeKeyName returns secondary key name for [edgeTypePrefix + TypeName] -> [EdgeType] mapping
func (c *authzCacheNameProvider) edgeTypeKeyName(typeName string) clientcache.CacheKey {
	return clientcache.CacheKey(fmt.Sprintf("%v_%v_%v", c.basePrefix, edgeTypePrefix, typeName))
}

// objAliasKeyName returns key name for [TypeID + Alias] -> [Object] mapping
func (c *authzCacheNameProvider) objAliasKeyName(typeID string, alias string) clientcache.CacheKey {
	return clientcache.CacheKey(fmt.Sprintf("%v_%v_%v_%v", c.basePrefix, objPrefix, typeID, alias))
}

// edgesObjToObj returns key name for [SourceObjID _ TargetObjID] -> [Edge [] ] mapping
func (c *authzCacheNameProvider) edgesObjToObj(sourceObjID string, targetObjID string) clientcache.CacheKey {
	return clientcache.CacheKey(fmt.Sprintf("%v_%v_%v_%v_%v", c.basePrefix, objPrefix, sourceObjID, perObjectEdgesPrefix, targetObjID))
}

// edgeFullKeyNameFromIDs returns key name for [SourceObjID _ TargetObjID _ EdgeTypeID] -> [Edge] mapping
func (c *authzCacheNameProvider) edgeFullKeyNameFromIDs(sourceID string, targetID string, typeID string) clientcache.CacheKey {
	return clientcache.CacheKey(fmt.Sprintf("%v_%v_%v_%v_%v", c.basePrefix, edgePrefix, sourceID, targetID, typeID))
}

// dependencyKey returns key name for dependency keys
func (c *authzCacheNameProvider) dependencyKey(id string) clientcache.CacheKey {
	return clientcache.CacheKey(fmt.Sprintf("%v_%v_%v", c.basePrefix, dependencylPrefix, id))
}

// objTypeCollectionKey returns key name for object type collection
func (c *authzCacheNameProvider) objTypeCollectionKey() clientcache.CacheKey {
	return clientcache.CacheKey(fmt.Sprintf("%v_%v", c.basePrefix, objTypeCollectionKeyString))
}

// edgeTypeCollectionKey returns key name for edge type collection
func (c *authzCacheNameProvider) edgeTypeCollectionKey() clientcache.CacheKey {
	return clientcache.CacheKey(fmt.Sprintf("%v_%v", c.basePrefix, edgeTypeCollectionKeyString))
}

// objCollectionKey returns key name for object collection
func (c *authzCacheNameProvider) objCollectionKey() clientcache.CacheKey {
	return clientcache.CacheKey(fmt.Sprintf("%v_%v", c.basePrefix, objCollectionKeyString))
}

// edgeCollectionKey returns key name for edge collection
func (c *authzCacheNameProvider) edgeCollectionKey() clientcache.CacheKey {
	return clientcache.CacheKey(c.basePrefix + edgeCollectionKeyString)
}

// orgCollectionKey returns key name for edge collection
func (c *authzCacheNameProvider) orgCollectionKey() clientcache.CacheKey {
	return clientcache.CacheKey(c.basePrefix + orgCollectionKeyString)
}

// attributePathObjToObj returns key name for attribute path
func (c *authzCacheNameProvider) attributePathObjToObj(sourceID string, targetID string, atributeName string) clientcache.CacheKey {
	return clientcache.CacheKey(fmt.Sprintf("%v_%v_%v_%v_%v_%v", c.basePrefix, objPrefix, sourceID, perObjectPathPrefix, targetID, atributeName))
}

// GetPrimaryKey returns the primary cache key name for object type
func (ot ObjectType) GetPrimaryKey(c clientcache.CacheKeyNameProvider) clientcache.CacheKey {
	return c.GetKeyNameWithID(objTypeKeyID, ot.ID)
}

// GetGlobalCollectionKey returns the global collection key name for object type
func (ot ObjectType) GetGlobalCollectionKey(c clientcache.CacheKeyNameProvider) clientcache.CacheKey {
	return c.GetKeyNameStatic(objTypeCollectionKeyID)
}

// GetSecondaryKeys returns the secondary cache key names for object type
func (ot ObjectType) GetSecondaryKeys(c clientcache.CacheKeyNameProvider) []clientcache.CacheKey {
	return []clientcache.CacheKey{c.GetKeyNameWithString(objTypeNameKeyID, ot.TypeName)}
}

// GetPerItemCollectionKey returns the per item collection key name for object type
func (ot ObjectType) GetPerItemCollectionKey(c clientcache.CacheKeyNameProvider) clientcache.CacheKey {
	return "" // Unused since there nothing stored per object type, could store objects of this type in the future
}

// GetDependenciesKey returns the dependencies key name for object type
func (ot ObjectType) GetDependenciesKey(c clientcache.CacheKeyNameProvider) clientcache.CacheKey {
	return "" // Unused since the whole cache is flushed on delete
}

// GetDependencyKeys returns the list of keys for object type dependencies
func (ot ObjectType) GetDependencyKeys(c clientcache.CacheKeyNameProvider) []clientcache.CacheKey {
	return []clientcache.CacheKey{} // ObjectTypes don't depend on anything
}

// TTL returns the TTL for object type
func (ot ObjectType) TTL(c clientcache.CacheTTLProvider) time.Duration {
	return c.TTL(objTypeTTL)
}

// GetPrimaryKey returns the primary cache key name for edge type
func (et EdgeType) GetPrimaryKey(c clientcache.CacheKeyNameProvider) clientcache.CacheKey {
	return c.GetKeyNameWithID(edgeTypeKeyID, et.ID)
}

// GetGlobalCollectionKey returns the global collection key name for edge type
func (et EdgeType) GetGlobalCollectionKey(c clientcache.CacheKeyNameProvider) clientcache.CacheKey {
	return c.GetKeyNameStatic(edgeTypeCollectionKeyID)
}

// GetPerItemCollectionKey returns the per item collection key name for edge type
func (et EdgeType) GetPerItemCollectionKey(c clientcache.CacheKeyNameProvider) clientcache.CacheKey {
	return "" // Unused since there nothing stored per edge type, could store edges of this type in the future
}

// GetSecondaryKeys returns the secondary cache key names for edge type
func (et EdgeType) GetSecondaryKeys(c clientcache.CacheKeyNameProvider) []clientcache.CacheKey {
	return []clientcache.CacheKey{c.GetKeyNameWithString(edgeTypeNameKeyID, et.TypeName)}
}

// GetDependenciesKey returns the dependencies key name for edge type
func (et EdgeType) GetDependenciesKey(c clientcache.CacheKeyNameProvider) clientcache.CacheKey {
	return "" // Unused since the whole cache is flushed on delete
}

// GetDependencyKeys returns the list of keys for edge type dependencies
func (et EdgeType) GetDependencyKeys(c clientcache.CacheKeyNameProvider) []clientcache.CacheKey {
	// EdgeTypes depend on source/target object types but we don't store that dependency because we currently flush the whole cache on object type delete
	return []clientcache.CacheKey{}
}

// TTL returns the TTL for edge type
func (et EdgeType) TTL(c clientcache.CacheTTLProvider) time.Duration {
	return c.TTL(edgeTypeTTL)
}

// GetPrimaryKey returns the primary cache key name for object
func (o Object) GetPrimaryKey(c clientcache.CacheKeyNameProvider) clientcache.CacheKey {
	return c.GetKeyNameWithID(objectKeyID, o.ID)
}

// GetSecondaryKeys returns the secondary cache key names for object
func (o Object) GetSecondaryKeys(c clientcache.CacheKeyNameProvider) []clientcache.CacheKey {
	if o.Alias != nil {
		return []clientcache.CacheKey{c.GetKeyName(objAliasNameKeyID, []string{o.TypeID.String(), *o.Alias})}
	}
	return []clientcache.CacheKey{}
}

// GetGlobalCollectionKey returns the global collection key name for object
func (o Object) GetGlobalCollectionKey(c clientcache.CacheKeyNameProvider) clientcache.CacheKey {
	return c.GetKeyNameStatic(objCollectionKeyID)
}

// GetPerItemCollectionKey returns the per item collection key name for object
func (o Object) GetPerItemCollectionKey(c clientcache.CacheKeyNameProvider) clientcache.CacheKey {
	return c.GetKeyNameWithID(objEdgesKeyID, o.ID)
}

// GetDependenciesKey return dependencies cache key name for object
func (o Object) GetDependenciesKey(c clientcache.CacheKeyNameProvider) clientcache.CacheKey {
	return c.GetKeyNameWithID(dependencyKeyID, o.ID)
}

// GetDependencyKeys returns the list of keys for object dependencies
func (o Object) GetDependencyKeys(c clientcache.CacheKeyNameProvider) []clientcache.CacheKey {
	// Objects depend on object types but we don't store that dependency because we currently flush the whole cache on object type delete
	return []clientcache.CacheKey{}
}

// TTL returns the TTL for object
func (o Object) TTL(c clientcache.CacheTTLProvider) time.Duration {
	return c.TTL(objTTL)
}

// GetPrimaryKey returns the primary cache key name for edge
func (e Edge) GetPrimaryKey(c clientcache.CacheKeyNameProvider) clientcache.CacheKey {
	return c.GetKeyNameWithID(edgeKeyID, e.ID)
}

// GetGlobalCollectionKey returns the global collection cache key names for edge
func (e Edge) GetGlobalCollectionKey(c clientcache.CacheKeyNameProvider) clientcache.CacheKey {
	return c.GetKeyNameStatic(edgeCollectionKeyID)
}

// GetPerItemCollectionKey returns the per item collection key name for edge
func (e Edge) GetPerItemCollectionKey(c clientcache.CacheKeyNameProvider) clientcache.CacheKey {
	return ""
}

// GetDependenciesKey return  dependencies cache key name for edge
func (e Edge) GetDependenciesKey(c clientcache.CacheKeyNameProvider) clientcache.CacheKey {
	return c.GetKeyNameWithID(dependencyKeyID, e.ID)
}

// GetDependencyKeys returns the list of keys for edge dependencies
func (e Edge) GetDependencyKeys(c clientcache.CacheKeyNameProvider) []clientcache.CacheKey {
	// Edges depend on objects and edge types. We don't store edgetype dependency because we currently flush the whole cache on edge type delete
	return []clientcache.CacheKey{c.GetKeyNameWithID(dependencyKeyID, e.SourceObjectID), c.GetKeyNameWithID(dependencyKeyID, e.TargetObjectID)}
}

// GetSecondaryKeys returns the secondary cache key names for edge
func (e Edge) GetSecondaryKeys(c clientcache.CacheKeyNameProvider) []clientcache.CacheKey {
	return []clientcache.CacheKey{c.GetKeyName(edgeFullKeyID, []string{e.SourceObjectID.String(), e.TargetObjectID.String(), e.EdgeTypeID.String()})}
}

// TTL returns the TTL for edge
func (e Edge) TTL(c clientcache.CacheTTLProvider) time.Duration {
	return c.TTL(edgeTTL)
}

// GetPrimaryKey returns the primary cache key name for edge
func (e AttributePathNode) GetPrimaryKey(c clientcache.CacheKeyNameProvider) clientcache.CacheKey {
	return "" // Unused since  AttributePathNode is not stored in cache directly
}

// GetGlobalCollectionKey returns the global collection cache key names for  path node
func (e AttributePathNode) GetGlobalCollectionKey(c clientcache.CacheKeyNameProvider) clientcache.CacheKey {
	return ""
}

// GetPerItemCollectionKey returns the per item collection key name for  path node
func (e AttributePathNode) GetPerItemCollectionKey(c clientcache.CacheKeyNameProvider) clientcache.CacheKey {
	return ""
}

// GetDependenciesKey return  dependencies cache key name for  path node
func (e AttributePathNode) GetDependenciesKey(c clientcache.CacheKeyNameProvider) clientcache.CacheKey {
	return ""
}

// GetDependencyKeys returns the list of keys for path node dependencies
func (e AttributePathNode) GetDependencyKeys(c clientcache.CacheKeyNameProvider) []clientcache.CacheKey {
	//  Path node depend on objects and edges.
	if e.EdgeID != uuid.Nil {
		return []clientcache.CacheKey{c.GetKeyNameWithID(dependencyKeyID, e.EdgeID), c.GetKeyNameWithID(dependencyKeyID, e.ObjectID)}
	}
	return []clientcache.CacheKey{c.GetKeyNameWithID(dependencyKeyID, e.ObjectID)}
}

// GetSecondaryKeys returns the secondary cache key names for path node
func (e AttributePathNode) GetSecondaryKeys(c clientcache.CacheKeyNameProvider) []clientcache.CacheKey {
	return []clientcache.CacheKey{}
}

// TTL returns the TTL for  path node
func (e AttributePathNode) TTL(c clientcache.CacheTTLProvider) time.Duration {
	return c.TTL(edgeTTL) // Same TTL as edge
}

// GetPrimaryKey returns the primary cache key name for organization
func (o Organization) GetPrimaryKey(c clientcache.CacheKeyNameProvider) clientcache.CacheKey {
	return c.GetKeyNameWithID(orgKeyID, o.ID)
}

// GetGlobalCollectionKey returns the global collection cache key names for organization
func (o Organization) GetGlobalCollectionKey(c clientcache.CacheKeyNameProvider) clientcache.CacheKey {
	return c.GetKeyNameStatic(orgCollectionKeyID)
}

// GetPerItemCollectionKey returns the per item collection key name for organization (none)
func (o Organization) GetPerItemCollectionKey(c clientcache.CacheKeyNameProvider) clientcache.CacheKey {
	return ""
}

// GetDependenciesKey return  dependencies cache key name for organization
func (o Organization) GetDependenciesKey(c clientcache.CacheKeyNameProvider) clientcache.CacheKey {
	return c.GetKeyNameWithID(dependencyKeyID, o.ID)
}

// GetDependencyKeys returns the list of keys for organization dependencies
func (o Organization) GetDependencyKeys(c clientcache.CacheKeyNameProvider) []clientcache.CacheKey {
	return []clientcache.CacheKey{}
}

// GetSecondaryKeys returns the secondary cache key names for organization (none)
func (o Organization) GetSecondaryKeys(c clientcache.CacheKeyNameProvider) []clientcache.CacheKey {
	return []clientcache.CacheKey{}
}

// TTL returns the TTL for edge
func (o Organization) TTL(c clientcache.CacheTTLProvider) time.Duration {
	return c.TTL(orgTTL)
}
