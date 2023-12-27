package authz

import (
	"github.com/gofrs/uuid"

	"userclouds.com/infra/namespace/region"
	"userclouds.com/infra/pagination"
	"userclouds.com/infra/ucdb"
	"userclouds.com/infra/ucerr"
)

// ObjectType represents the type definition of an AuthZ object.
type ObjectType struct {
	ucdb.BaseModel

	TypeName string `db:"type_name" json:"type_name" validate:"notempty" required:"true"`
}

// GetPaginationKeys is part of the pagination.PageableType interface
func (ObjectType) GetPaginationKeys() pagination.KeyTypes {
	return pagination.KeyTypes{
		"id":        pagination.UUIDKeyType,
		"type_name": pagination.StringKeyType,
		"created":   pagination.TimestampKeyType,
		"updated":   pagination.TimestampKeyType,
	}
}

//go:generate genvalidate ObjectType

// Attribute represents a named attribute on an Edge Type.
type Attribute struct {
	Name string `db:"name" json:"name" validate:"notempty" required:"true"`

	// Direct = true means that this attribute applies directly from the source to the target, or
	// alternately stated that "the source object 'has' the attribute on the target".
	// e.g. given an edge {Source: Alice, Target: Readme.txt, Type: Viewer} with attribute {Name:"read", Direct: true},
	// then Alice directly 'has' the "read" attribute on Readme.txt
	Direct bool `db:"direct" json:"direct"`

	// Inherit = true means that, if the target object 'has' (or inherits) the attribute on some other object X,
	// then the source object "inherits" that attribute on X as well. This applies transitively across
	// multiple consecutive Inherit edges.
	// e.g. given an edge {Source: Alice, Target: RootUsersGroup, Type: Member} with attribute {Name:"read", Inherit: true},
	// and another edge {Source: RootUsersGroup, Target: Readme.txt, Type: Viewer} with attribute {Name:"read", Direct: true},
	// then the Root Users group has direct read permissions on Readme.txt and Alice inherits the read permission
	// on Readme.txt through its connection to the RootUsersGroup.
	// This flag is typically used when some objects (e.g. users, files) should inherit attributes
	// that a "grouping" object has on some final target object without requiring direct edges between
	// every source and every target (e.g. between Alice and Readme.txt, in this example).
	// The Inherit flag would be used on attributes that associate the source objects with the grouping object.
	// This is like a "pull" model for permissions, while Propagate represents a "push" model.
	Inherit bool `db:"inherit" json:"inherit"`

	// Propagate = true means that some object X which has an attribute on the source object will also have the same
	// attribute on the target object. This is effectively the inverse of Inherit, and "propagates" attributes forward.
	// e.g. given an edge {Source: Alice, Target: HomeDirectory, Type: Viewer} with attribute {Name: "read", Direct: true},
	// and another edge {Source: HomeDirectory, Target: Readme.txt, Type: Contains} with attribute {Name: "read", Propagate: true},
	// then Alice's read permission on the HomeDirectory propagates to Readme.txt since that is (presumably) contained in the
	// Home directory.
	// This is like a "push" model for permissions, while Inherit represents a "pull" model.
	// This is different from Direct = true because it doesn't make sense for the Home directory to have
	// direct "read" attributes on files within it, but simply propagate the permissions down the tree.
	// Permissions don't propagate through Direct links; if Alice has a 'direct' "friend" relationship to Bob,
	// and Bob has a 'direct' "friend" relationship to Charlie,
	// that wouldn't imply Alice has a 'direct' "friend" relationship to Charlie (direct != propagate).
	Propagate bool `db:"propagate" json:"propagate"`
}

func (a *Attribute) extraValidate() error {
	if (a.Direct && !a.Inherit && !a.Propagate) ||
		(!a.Direct && a.Inherit && !a.Propagate) ||
		(!a.Direct && !a.Inherit && a.Propagate) {
		return nil
	}
	return ucerr.Errorf("exactly 1 of Attribute.{Direct, Inherit, Propagate} must be true; got {%t, %t, %t} instead", a.Direct, a.Inherit, a.Propagate)
}

//go:generate genvalidate Attribute

// Attributes is a collection of Attribute, used as a column/field in EdgeType
type Attributes []Attribute

//go:generate gendbjson Attributes

// EdgeType defines a single, strongly-typed relationship
// that a "source" object type can have to a "target" object type.
type EdgeType struct {
	ucdb.BaseModel

	TypeName           string     `db:"type_name" json:"type_name"  validate:"notempty" required:"true"`
	SourceObjectTypeID uuid.UUID  `db:"source_object_type_id,immutable" json:"source_object_type_id"  validate:"notnil" required:"true"`
	TargetObjectTypeID uuid.UUID  `db:"target_object_type_id,immutable" json:"target_object_type_id"  validate:"notnil" required:"true"`
	Attributes         Attributes `db:"attributes" json:"attributes"`

	OrganizationID uuid.UUID `db:"organization_id" json:"organization_id"`
}

// Equals returns true if the two edges are equal, ignoring the ID field
func (e *EdgeType) Equals(other *EdgeType, includeOrg bool) bool {
	if e.TypeName == other.TypeName && e.SourceObjectTypeID == other.SourceObjectTypeID && e.TargetObjectTypeID == other.TargetObjectTypeID && (!includeOrg || e.OrganizationID == other.OrganizationID) {
		if len(e.Attributes) != len(other.Attributes) {
			return false
		}
		for i := range e.Attributes {
			if e.Attributes[i] != other.Attributes[i] {
				return false
			}
		}
		return true
	}
	return false
}

//go:generate genvalidate EdgeType

// GetPaginationKeys is part of the pagination.PageableType interface
func (EdgeType) GetPaginationKeys() pagination.KeyTypes {
	return pagination.KeyTypes{
		"id":                    pagination.UUIDKeyType,
		"organization_id":       pagination.UUIDKeyType,
		"source_object_type_id": pagination.UUIDKeyType,
		"target_object_type_id": pagination.UUIDKeyType,
		"created":               pagination.TimestampKeyType,
		"updated":               pagination.TimestampKeyType,
	}
}

// Object represents an instance of an AuthZ object used for modeling permissions.
type Object struct {
	ucdb.BaseModel

	Alias  *string   `db:"alias" json:"alias,omitempty" validate:"allownil"`
	TypeID uuid.UUID `db:"type_id,immutable" json:"type_id" validate:"notnil" required:"true"`

	OrganizationID uuid.UUID `db:"organization_id" json:"organization_id"`
}

// Equals returns true if the two objects are equal, ignoring the ID field
func (o *Object) Equals(other *Object, includeOrg bool) bool {
	if o.Alias == nil && other.Alias == nil {
		return o.TypeID == other.TypeID && (!includeOrg || o.OrganizationID == other.OrganizationID)
	}
	if o.Alias == nil || other.Alias == nil {
		return false
	}
	return *o.Alias == *other.Alias && o.TypeID == other.TypeID && (!includeOrg || o.OrganizationID == other.OrganizationID)
}

//go:generate genvalidate Object

// GetPaginationKeys is part of the pagination.PageableType interface
func (Object) GetPaginationKeys() pagination.KeyTypes {
	return pagination.KeyTypes{
		"id":              pagination.UUIDKeyType,
		"alias":           pagination.StringKeyType,
		"organization_id": pagination.UUIDKeyType,
		"type_id":         pagination.UUIDKeyType,
		"created":         pagination.TimestampKeyType,
		"updated":         pagination.TimestampKeyType,
	}
}

// Edge represents a directional relationship between a "source"
// object and a "target" object.
type Edge struct {
	ucdb.BaseModel

	// This must be a valid EdgeType.ID value
	EdgeTypeID uuid.UUID `db:"edge_type_id" json:"edge_type_id" validate:"notnil" required:"true"`
	// These must be valid ObjectType.ID values
	SourceObjectID uuid.UUID `db:"source_object_id" json:"source_object_id" validate:"notnil" required:"true"`
	TargetObjectID uuid.UUID `db:"target_object_id" json:"target_object_id" validate:"notnil" required:"true"`
}

//go:generate genvalidate Edge

// GetPaginationKeys is part of the pagination.PageableType interface
func (Edge) GetPaginationKeys() pagination.KeyTypes {
	return pagination.KeyTypes{
		"id":               pagination.UUIDKeyType,
		"source_object_id": pagination.UUIDKeyType,
		"target_object_id": pagination.UUIDKeyType,
		"created":          pagination.TimestampKeyType,
		"updated":          pagination.TimestampKeyType,
	}
}

// Organization defines a collection of objects inside of a single AuthZ namespace.
// Uniqueness (of eg. Object aliases) is enforced by organization, rather than globally in a tenant
type Organization struct {
	ucdb.BaseModel

	Name   string        `db:"name" json:"name" validate:"notempty" required:"true"`
	Region region.Region `db:"region" json:"region"`
}

//go:generate genvalidate Organization

func (o Organization) extraValidate() error {
	if o.Region != "" && !region.IsValid(o.Region) {
		return ucerr.Friendlyf(nil, "invalid region specified: %s", o.Region)
	}
	return nil
}

// GetPaginationKeys is part of the pagination.PageableType interface
func (Organization) GetPaginationKeys() pagination.KeyTypes {
	return pagination.KeyTypes{
		"id":      pagination.UUIDKeyType,
		"name":    pagination.StringKeyType,
		"created": pagination.TimestampKeyType,
		"updated": pagination.TimestampKeyType,
	}
}

//go:generate genvalidate CreateObjectTypeRequest

//go:generate genvalidate CreateEdgeTypeRequest

//go:generate genvalidate UpdateEdgeTypeRequest

//go:generate genvalidate CreateObjectRequest

//go:generate genvalidate CreateEdgeRequest

//go:generate genvalidate CreateOrganizationRequest

//go:generate genvalidate UpdateOrganizationRequest
