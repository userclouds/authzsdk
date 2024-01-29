package authz

import (
	"fmt"
	"time"

	"github.com/gofrs/uuid"

	clientcache "userclouds.com/infra/cache/client"
	cache "userclouds.com/infra/cache/shared"
)

const (
	// CachePrefix is the prefix for all keys in authz cache
	CachePrefix                 = "authz"
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
	dependencyPrefix            = "DEP"         // Shared dependency key prefix among all items
	isModifiedPrefix            = "MOD"         // Shared is modified key prefix among all items
)

// CacheNameProvider is the base implementation of the CacheNameProvider interface
type CacheNameProvider struct {
	basePrefix string // Base prefix for all keys TenantID_OrgID
}

// NewCacheNameProvider creates a new BasesCacheNameProvider
func NewCacheNameProvider(basePrefix string) *CacheNameProvider {
	return &CacheNameProvider{basePrefix: basePrefix}
}

const (
	// ObjectTypeKeyID is the primary key for object type
	ObjectTypeKeyID = "ObjTypeKeyID"
	// EdgeTypeKeyID is the primary key for edge type
	EdgeTypeKeyID = "EdgeTypeKeyID"
	// ObjectKeyID is the primary key for object
	ObjectKeyID = "ObjectKeyID"
	// EdgeKeyID is the primary key for edge
	EdgeKeyID = "EdgeKeyID"
	// OrganizationKeyID is the primary key for organization
	OrganizationKeyID = "OrgKeyID"
	// EdgeFullKeyID is the secondary key for edge
	EdgeFullKeyID = "EdgeFullKeyNameID"
	// ObjectTypeNameKeyID is the secondary key for object type
	ObjectTypeNameKeyID = "ObjectTypeKeyNameID"
	// ObjEdgesKeyID is the key for collection of edges of an object
	ObjEdgesKeyID = "ObjectEdgesKeyID"
	// EdgeTypeNameKeyID is the secondary key for edge type
	EdgeTypeNameKeyID = "EdgeTypeKeyNameID"
	// ObjAliasNameKeyID is the secondary key for object
	ObjAliasNameKeyID = "ObjAliasKeyNameID"
	// OrganizationNameKeyID is the secondary key for organization
	OrganizationNameKeyID = "OrgCollectionKeyNameID"
	// EdgesObjToObjID is the key for collection of edges between two objects
	EdgesObjToObjID = "EdgesObjToObjID"
	// DependencyKeyID is the key for list of dependencies
	DependencyKeyID = "DependencyKeyID"
	// IsModifiedKeyID is the key value indicating change in last TTL
	IsModifiedKeyID = "IsModifiedKeyID"
	// ObjectTypeCollectionKeyID is the key for global collection of object types
	ObjectTypeCollectionKeyID = "ObjTypeCollectionKeyID"
	// EdgeTypeCollectionKeyID is the key for global collection of edge types
	EdgeTypeCollectionKeyID = "EdgeTypeCollectionKeyID"
	// ObjectCollectionKeyID is the key for global collection of objects
	ObjectCollectionKeyID = "ObjCollectionKeyID"
	// EdgeCollectionKeyID is the key for global collection of edges
	EdgeCollectionKeyID = "EdgeCollectionKeyID"
	// OrganizationCollectionKeyID is the key for global collection of organizations
	OrganizationCollectionKeyID = "OrgCollectionKeyID"
	// AttributePathObjToObjID is the primary key for attribute path
	AttributePathObjToObjID = "AttributePathObjToObjID"
)

// GetKeyNameStatic is a shortcut for GetKeyName with without component
func (c *CacheNameProvider) GetKeyNameStatic(id clientcache.CacheKeyNameID) cache.CacheKey {
	return c.GetKeyName(id, []string{})
}

// GetKeyNameWithID is a shortcut for GetKeyName with a single uuid ID component
func (c *CacheNameProvider) GetKeyNameWithID(id clientcache.CacheKeyNameID, itemID uuid.UUID) cache.CacheKey {
	return c.GetKeyName(id, []string{itemID.String()})
}

// GetKeyNameWithString is a shortcut for GetKeyName with a single string component
func (c *CacheNameProvider) GetKeyNameWithString(id clientcache.CacheKeyNameID, itemName string) cache.CacheKey {
	return c.GetKeyName(id, []string{itemName})
}

// GetKeyName gets the key name for the given key name ID and components
func (c *CacheNameProvider) GetKeyName(id clientcache.CacheKeyNameID, components []string) cache.CacheKey {
	switch id {
	case ObjectTypeKeyID:
		return c.objectTypeKey(components[0])
	case EdgeTypeKeyID:
		return c.edgeTypeKey(components[0])
	case ObjectKeyID:
		return c.objectKey(components[0])
	case EdgeKeyID:
		return c.edgeKey(components[0])
	case OrganizationKeyID:
		return c.orgKey(components[0])
	case ObjectTypeNameKeyID:
		return c.objectTypeKeyName(components[0])
	case EdgeTypeNameKeyID:
		return c.edgeTypeKeyName(components[0])
	case ObjAliasNameKeyID:
		return c.objAliasKeyName(components[0], components[1], components[2])
	case OrganizationNameKeyID:
		return c.orgKeyName(components[0])
	case EdgesObjToObjID:
		return c.edgesObjToObj(components[0], components[1])

	case ObjectTypeCollectionKeyID:
		return c.objTypeCollectionKey()
	case EdgeTypeCollectionKeyID:
		return c.edgeTypeCollectionKey()
	case ObjectCollectionKeyID:
		return c.objCollectionKey()
	case EdgeCollectionKeyID:
		return c.edgeCollectionKey()
	case OrganizationCollectionKeyID:
		return c.orgCollectionKey()
	case ObjEdgesKeyID:
		return c.objectEdgesKey(components[0])
	case DependencyKeyID:
		return c.dependencyKey(components[0])
	case IsModifiedKeyID:
		return c.isModifiedKey(components[0])
	case EdgeFullKeyID:
		return c.edgeFullKeyNameFromIDs(components[0], components[1], components[2])
	case AttributePathObjToObjID:
		return c.attributePathObjToObj(components[0], components[1], components[2])
	}
	return ""
}

// objectTypeKey primary key for object type
func (c *CacheNameProvider) objectTypeKey(id string) cache.CacheKey {
	return cache.CacheKey(fmt.Sprintf("%v_%v_%v", c.basePrefix, objTypePrefix, id))
}

// edgeTypeKey primary key for edge type
func (c *CacheNameProvider) edgeTypeKey(id string) cache.CacheKey {
	return cache.CacheKey(fmt.Sprintf("%v_%v_%v", c.basePrefix, edgeTypePrefix, id))
}

// objectKey primary key for object
func (c *CacheNameProvider) objectKey(id string) cache.CacheKey {
	return cache.CacheKey(fmt.Sprintf("%v_%v_%v", c.basePrefix, objPrefix, id))
}

// edgeKey primary key for edge
func (c *CacheNameProvider) edgeKey(id string) cache.CacheKey {
	return cache.CacheKey(fmt.Sprintf("%v_%v_%v", c.basePrefix, edgePrefix, id))
}

// orgKey primary key for edge
func (c *CacheNameProvider) orgKey(id string) cache.CacheKey {
	return cache.CacheKey(fmt.Sprintf("%v_%v_%v", c.basePrefix, orgPrefix, id))
}

// objectTypeKeyName returns secondary key name for [objTypePrefix + TypeName] -> [ObjType] mapping
func (c *CacheNameProvider) objectTypeKeyName(typeName string) cache.CacheKey {
	return cache.CacheKey(fmt.Sprintf("%v_%v_%v", c.basePrefix, objTypePrefix, typeName))
}

// objectEdgesKey returns key name for per object edges collection
func (c *CacheNameProvider) objectEdgesKey(id string) cache.CacheKey {
	return cache.CacheKey(fmt.Sprintf("%v_%v_%v", c.basePrefix, objEdgeCollection, id))
}

// edgeTypeKeyName returns secondary key name for [edgeTypePrefix + TypeName] -> [EdgeType] mapping
func (c *CacheNameProvider) edgeTypeKeyName(typeName string) cache.CacheKey {
	return cache.CacheKey(fmt.Sprintf("%v_%v_%v", c.basePrefix, edgeTypePrefix, typeName))
}

// objAliasKeyName returns key name for [TypeID + Alias] -> [Object] mapping
func (c *CacheNameProvider) objAliasKeyName(typeID string, alias string, orgID string) cache.CacheKey {
	return cache.CacheKey(fmt.Sprintf("%v_%v_%v_%v_%v", c.basePrefix, objPrefix, typeID, alias, orgID))
}

// edgesObjToObj returns key name for [SourceObjID _ TargetObjID] -> [Edge [] ] mapping
func (c *CacheNameProvider) edgesObjToObj(sourceObjID string, targetObjID string) cache.CacheKey {
	return cache.CacheKey(fmt.Sprintf("%v_%v_%v_%v_%v", c.basePrefix, objPrefix, sourceObjID, perObjectEdgesPrefix, targetObjID))
}

// edgeFullKeyNameFromIDs returns key name for [SourceObjID _ TargetObjID _ EdgeTypeID] -> [Edge] mapping
func (c *CacheNameProvider) edgeFullKeyNameFromIDs(sourceID string, targetID string, typeID string) cache.CacheKey {
	return cache.CacheKey(fmt.Sprintf("%v_%v_%v_%v_%v", c.basePrefix, edgePrefix, sourceID, targetID, typeID))
}

// orgKeyName returns secondary key name for [orgPrefix + Name] -> [Organization] mapping
func (c *CacheNameProvider) orgKeyName(orgName string) cache.CacheKey {
	return cache.CacheKey(fmt.Sprintf("%v_%v_%v", c.basePrefix, orgPrefix, orgName))
}

// dependencyKey returns key name for dependency keys
func (c *CacheNameProvider) dependencyKey(id string) cache.CacheKey {
	return cache.CacheKey(fmt.Sprintf("%v_%v_%v", c.basePrefix, dependencyPrefix, id))
}

// isModifiedKey returns key name for isModified key
func (c *CacheNameProvider) isModifiedKey(id string) cache.CacheKey {
	return cache.CacheKey(fmt.Sprintf("%v_%v_%v", c.basePrefix, isModifiedPrefix, id))
}

// objTypeCollectionKey returns key name for object type collection
func (c *CacheNameProvider) objTypeCollectionKey() cache.CacheKey {
	return cache.CacheKey(fmt.Sprintf("%v_%v", c.basePrefix, objTypeCollectionKeyString))
}

// edgeTypeCollectionKey returns key name for edge type collection
func (c *CacheNameProvider) edgeTypeCollectionKey() cache.CacheKey {
	return cache.CacheKey(fmt.Sprintf("%v_%v", c.basePrefix, edgeTypeCollectionKeyString))
}

// objCollectionKey returns key name for object collection
func (c *CacheNameProvider) objCollectionKey() cache.CacheKey {
	return cache.CacheKey(fmt.Sprintf("%v_%v", c.basePrefix, objCollectionKeyString))
}

// edgeCollectionKey returns key name for edge collection
func (c *CacheNameProvider) edgeCollectionKey() cache.CacheKey {
	return cache.CacheKey(c.basePrefix + edgeCollectionKeyString)
}

// orgCollectionKey returns key name for edge collection
func (c *CacheNameProvider) orgCollectionKey() cache.CacheKey {
	return cache.CacheKey(c.basePrefix + orgCollectionKeyString)
}

// attributePathObjToObj returns key name for attribute path
func (c *CacheNameProvider) attributePathObjToObj(sourceID string, targetID string, atributeName string) cache.CacheKey {
	return cache.CacheKey(fmt.Sprintf("%v_%v_%v_%v_%v_%v", c.basePrefix, objPrefix, sourceID, perObjectPathPrefix, targetID, atributeName))
}

// GetPrimaryKey returns the primary cache key name for object type
func (ot ObjectType) GetPrimaryKey(c clientcache.CacheKeyNameProvider) cache.CacheKey {
	return c.GetKeyNameWithID(ObjectTypeKeyID, ot.ID)
}

// GetGlobalCollectionKey returns the global collection key name for object type
func (ot ObjectType) GetGlobalCollectionKey(c clientcache.CacheKeyNameProvider) cache.CacheKey {
	return c.GetKeyNameStatic(ObjectTypeCollectionKeyID)
}

// GetSecondaryKeys returns the secondary cache key names for object type
func (ot ObjectType) GetSecondaryKeys(c clientcache.CacheKeyNameProvider) []cache.CacheKey {
	return []cache.CacheKey{c.GetKeyNameWithString(ObjectTypeNameKeyID, ot.TypeName)}
}

// GetPerItemCollectionKey returns the per item collection key name for object type
func (ot ObjectType) GetPerItemCollectionKey(c clientcache.CacheKeyNameProvider) cache.CacheKey {
	return "" // Unused since there nothing stored per object type, could store objects of this type in the future
}

// GetDependenciesKey returns the dependencies key name for object type
func (ot ObjectType) GetDependenciesKey(c clientcache.CacheKeyNameProvider) cache.CacheKey {
	return "" // Unused since the whole cache is flushed on delete
}

// GetIsModifiedKey returns the isModifiedKey key name for object type
func (ot ObjectType) GetIsModifiedKey(c clientcache.CacheKeyNameProvider) cache.CacheKey {
	return c.GetKeyNameWithID(IsModifiedKeyID, ot.ID)
}

// GetDependencyKeys returns the list of keys for object type dependencies
func (ot ObjectType) GetDependencyKeys(c clientcache.CacheKeyNameProvider) []cache.CacheKey {
	return []cache.CacheKey{} // ObjectTypes don't depend on anything
}

// TTL returns the TTL for object type
func (ot ObjectType) TTL(c clientcache.CacheTTLProvider) time.Duration {
	return c.TTL(ObjectTypeTTL)
}

// GetPrimaryKey returns the primary cache key name for edge type
func (et EdgeType) GetPrimaryKey(c clientcache.CacheKeyNameProvider) cache.CacheKey {
	return c.GetKeyNameWithID(EdgeTypeKeyID, et.ID)
}

// GetGlobalCollectionKey returns the global collection key name for edge type
func (et EdgeType) GetGlobalCollectionKey(c clientcache.CacheKeyNameProvider) cache.CacheKey {
	return c.GetKeyNameStatic(EdgeTypeCollectionKeyID)
}

// GetPerItemCollectionKey returns the per item collection key name for edge type
func (et EdgeType) GetPerItemCollectionKey(c clientcache.CacheKeyNameProvider) cache.CacheKey {
	return "" // Unused since there nothing stored per edge type, could store edges of this type in the future
}

// GetSecondaryKeys returns the secondary cache key names for edge type
func (et EdgeType) GetSecondaryKeys(c clientcache.CacheKeyNameProvider) []cache.CacheKey {
	return []cache.CacheKey{c.GetKeyNameWithString(EdgeTypeNameKeyID, et.TypeName)}
}

// GetDependenciesKey returns the dependencies key name for edge type
func (et EdgeType) GetDependenciesKey(c clientcache.CacheKeyNameProvider) cache.CacheKey {
	return "" // Unused since the whole cache is flushed on delete
}

// GetIsModifiedKey returns the isModifiedKey key name for edge type
func (et EdgeType) GetIsModifiedKey(c clientcache.CacheKeyNameProvider) cache.CacheKey {
	return c.GetKeyNameWithID(IsModifiedKeyID, et.ID)
}

// GetDependencyKeys returns the list of keys for edge type dependencies
func (et EdgeType) GetDependencyKeys(c clientcache.CacheKeyNameProvider) []cache.CacheKey {
	// EdgeTypes depend on source/target object types but we don't store that dependency because we currently flush the whole cache on object type delete
	return []cache.CacheKey{}
}

// TTL returns the TTL for edge type
func (et EdgeType) TTL(c clientcache.CacheTTLProvider) time.Duration {
	return c.TTL(EdgeTypeTTL)
}

// GetPrimaryKey returns the primary cache key name for object
func (o Object) GetPrimaryKey(c clientcache.CacheKeyNameProvider) cache.CacheKey {
	return c.GetKeyNameWithID(ObjectKeyID, o.ID)
}

// GetSecondaryKeys returns the secondary cache key names for object
func (o Object) GetSecondaryKeys(c clientcache.CacheKeyNameProvider) []cache.CacheKey {
	if o.Alias != nil {
		return []cache.CacheKey{c.GetKeyName(ObjAliasNameKeyID, []string{o.TypeID.String(), *o.Alias, o.OrganizationID.String()})}
	}
	return []cache.CacheKey{}
}

// GetGlobalCollectionKey returns the global collection key name for object
func (o Object) GetGlobalCollectionKey(c clientcache.CacheKeyNameProvider) cache.CacheKey {
	return c.GetKeyNameStatic(ObjectCollectionKeyID)
}

// GetPerItemCollectionKey returns the per item collection key name for object
func (o Object) GetPerItemCollectionKey(c clientcache.CacheKeyNameProvider) cache.CacheKey {
	return c.GetKeyNameWithID(ObjEdgesKeyID, o.ID)
}

// GetDependenciesKey return dependencies cache key name for object
func (o Object) GetDependenciesKey(c clientcache.CacheKeyNameProvider) cache.CacheKey {
	return c.GetKeyNameWithID(DependencyKeyID, o.ID)
}

// GetIsModifiedKey returns the isModifiedKey key name for object
func (o Object) GetIsModifiedKey(c clientcache.CacheKeyNameProvider) cache.CacheKey {
	return c.GetKeyNameWithID(IsModifiedKeyID, o.ID)
}

// GetDependencyKeys returns the list of keys for object dependencies
func (o Object) GetDependencyKeys(c clientcache.CacheKeyNameProvider) []cache.CacheKey {
	// Objects depend on object types but we don't store that dependency because we currently flush the whole cache on object type delete
	return []cache.CacheKey{}
}

// TTL returns the TTL for object
func (o Object) TTL(c clientcache.CacheTTLProvider) time.Duration {
	return c.TTL(ObjectTTL)
}

// GetPrimaryKey returns the primary cache key name for edge
func (e Edge) GetPrimaryKey(c clientcache.CacheKeyNameProvider) cache.CacheKey {
	return c.GetKeyNameWithID(EdgeKeyID, e.ID)
}

// GetGlobalCollectionKey returns the global collection cache key names for edge
func (e Edge) GetGlobalCollectionKey(c clientcache.CacheKeyNameProvider) cache.CacheKey {
	return c.GetKeyNameStatic(EdgeCollectionKeyID)
}

// GetPerItemCollectionKey returns the per item collection key name for edge
func (e Edge) GetPerItemCollectionKey(c clientcache.CacheKeyNameProvider) cache.CacheKey {
	return ""
}

// GetDependenciesKey return  dependencies cache key name for edge
func (e Edge) GetDependenciesKey(c clientcache.CacheKeyNameProvider) cache.CacheKey {
	return c.GetKeyNameWithID(DependencyKeyID, e.ID)
}

// GetIsModifiedKey returns the isModifiedKey key name for edge
func (e Edge) GetIsModifiedKey(c clientcache.CacheKeyNameProvider) cache.CacheKey {
	return c.GetKeyNameWithID(IsModifiedKeyID, e.ID)
}

// GetDependencyKeys returns the list of keys for edge dependencies
func (e Edge) GetDependencyKeys(c clientcache.CacheKeyNameProvider) []cache.CacheKey {
	// Edges depend on objects and edge types. We don't store edgetype dependency because we currently flush the whole cache on edge type delete
	return []cache.CacheKey{c.GetKeyNameWithID(DependencyKeyID, e.SourceObjectID), c.GetKeyNameWithID(DependencyKeyID, e.TargetObjectID)}
}

// GetSecondaryKeys returns the secondary cache key names for edge
func (e Edge) GetSecondaryKeys(c clientcache.CacheKeyNameProvider) []cache.CacheKey {
	return []cache.CacheKey{c.GetKeyName(EdgeFullKeyID, []string{e.SourceObjectID.String(), e.TargetObjectID.String(), e.EdgeTypeID.String()})}
}

// TTL returns the TTL for edge
func (e Edge) TTL(c clientcache.CacheTTLProvider) time.Duration {
	return c.TTL(EdgeTTL)
}

// GetPrimaryKey returns the primary cache key name for edge
func (e AttributePathNode) GetPrimaryKey(c clientcache.CacheKeyNameProvider) cache.CacheKey {
	return "" // Unused since  AttributePathNode is not stored in cache directly
}

// GetGlobalCollectionKey returns the global collection cache key names for  path node
func (e AttributePathNode) GetGlobalCollectionKey(c clientcache.CacheKeyNameProvider) cache.CacheKey {
	return ""
}

// GetPerItemCollectionKey returns the per item collection key name for  path node
func (e AttributePathNode) GetPerItemCollectionKey(c clientcache.CacheKeyNameProvider) cache.CacheKey {
	return ""
}

// GetDependenciesKey return  dependencies cache key name for  path node
func (e AttributePathNode) GetDependenciesKey(c clientcache.CacheKeyNameProvider) cache.CacheKey {
	return ""
}

// GetIsModifiedKey returns the isModifiedKey key name for attribute path
func (e AttributePathNode) GetIsModifiedKey(c clientcache.CacheKeyNameProvider) cache.CacheKey {
	return ""
}

// GetDependencyKeys returns the list of keys for path node dependencies
func (e AttributePathNode) GetDependencyKeys(c clientcache.CacheKeyNameProvider) []cache.CacheKey {
	//  Path node depend on objects and edges.
	if e.EdgeID != uuid.Nil {
		return []cache.CacheKey{c.GetKeyNameWithID(DependencyKeyID, e.EdgeID), c.GetKeyNameWithID(DependencyKeyID, e.ObjectID)}
	}
	return []cache.CacheKey{c.GetKeyNameWithID(DependencyKeyID, e.ObjectID)}
}

// GetSecondaryKeys returns the secondary cache key names for path node
func (e AttributePathNode) GetSecondaryKeys(c clientcache.CacheKeyNameProvider) []cache.CacheKey {
	return []cache.CacheKey{}
}

// TTL returns the TTL for  path node
func (e AttributePathNode) TTL(c clientcache.CacheTTLProvider) time.Duration {
	return c.TTL(EdgeTTL) // Same TTL as edge
}

// GetPrimaryKey returns the primary cache key name for organization
func (o Organization) GetPrimaryKey(c clientcache.CacheKeyNameProvider) cache.CacheKey {
	return c.GetKeyNameWithID(OrganizationKeyID, o.ID)
}

// GetGlobalCollectionKey returns the global collection cache key names for organization
func (o Organization) GetGlobalCollectionKey(c clientcache.CacheKeyNameProvider) cache.CacheKey {
	return c.GetKeyNameStatic(OrganizationCollectionKeyID)
}

// GetPerItemCollectionKey returns the per item collection key name for organization (none)
func (o Organization) GetPerItemCollectionKey(c clientcache.CacheKeyNameProvider) cache.CacheKey {
	return ""
}

// GetDependenciesKey return  dependencies cache key name for organization
func (o Organization) GetDependenciesKey(c clientcache.CacheKeyNameProvider) cache.CacheKey {
	return c.GetKeyNameWithID(DependencyKeyID, o.ID)
}

// GetIsModifiedKey returns the isModifiedKey key name for organization
func (o Organization) GetIsModifiedKey(c clientcache.CacheKeyNameProvider) cache.CacheKey {
	return c.GetKeyNameWithID(IsModifiedKeyID, o.ID)
}

// GetDependencyKeys returns the list of keys for organization dependencies
func (o Organization) GetDependencyKeys(c clientcache.CacheKeyNameProvider) []cache.CacheKey {
	return []cache.CacheKey{}
}

// GetSecondaryKeys returns the secondary cache key names for organization (none)
func (o Organization) GetSecondaryKeys(c clientcache.CacheKeyNameProvider) []cache.CacheKey {
	return []cache.CacheKey{c.GetKeyNameWithString(OrganizationNameKeyID, o.Name)}
}

// TTL returns the TTL for edge
func (o Organization) TTL(c clientcache.CacheTTLProvider) time.Duration {
	return c.TTL(OrganizationTTL)
}
