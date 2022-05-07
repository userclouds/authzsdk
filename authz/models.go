package authz

import (
	"github.com/gofrs/uuid"

	"userclouds.com/infra/ucdb"
	"userclouds.com/infra/ucerr"
)

// ObjectType represents the type definition of an AuthZ object.
type ObjectType struct {
	ucdb.BaseModel

	TypeName string `db:"type_name" json:"type_name"`
}

// Validate implements Validateable
func (o ObjectType) Validate() error {
	if o.TypeName == "" {
		return ucerr.New("TypeName can't be empty")
	}
	return ucerr.Wrap(o.BaseModel.Validate())
}

// EdgeType defines a single, strongly-typed relationship
// that a "source" object type can have to a "target" object type.
type EdgeType struct {
	ucdb.BaseModel

	TypeName           string    `db:"type_name" json:"type_name"`
	SourceObjectTypeID uuid.UUID `db:"source_object_type_id,immutable" json:"source_object_type_id"`
	TargetObjectTypeID uuid.UUID `db:"target_object_type_id,immutable" json:"target_object_type_id"`
}

// Validate implements Validateable
func (e EdgeType) Validate() error {
	if e.TypeName == "" {
		return ucerr.New("TypeName can't be empty")
	}
	if e.SourceObjectTypeID == uuid.Nil {
		return ucerr.New("SourceObjectTypeID can't have nil ID")
	}
	if e.TargetObjectTypeID == uuid.Nil {
		return ucerr.New("TargetObjectTypeID can't have nil ID")
	}
	return nil
}

// Object represents an instance of an AuthZ object used for modeling permissions.
type Object struct {
	ucdb.BaseModel

	Alias  string    `db:"alias" json:"alias"`
	TypeID uuid.UUID `db:"type_id,immutable" json:"type_id"`
}

// Validate implements Validateable
func (o Object) Validate() error {
	if o.Alias == "" {
		return ucerr.New("Alias can't be empty")
	}
	if o.TypeID == uuid.Nil {
		return ucerr.New("TypeID can't have nil ID")
	}
	return ucerr.Wrap(o.BaseModel.Validate())
}

// Edge represents a directional relationship between a "source"
// object and a "target" object.
type Edge struct {
	ucdb.BaseModel

	// This must be a valid EdgeType.ID value
	EdgeTypeID uuid.UUID `db:"edge_type_id" json:"edge_type_id"`
	// These must be valid ObjectType.ID values
	SourceObjectID uuid.UUID `db:"source_object_id" json:"source_object_id"`
	TargetObjectID uuid.UUID `db:"target_object_id" json:"target_object_id"`
}

// Validate implements Validateable
func (o Edge) Validate() error {
	if o.EdgeTypeID == uuid.Nil {
		return ucerr.New("EdgeTypeID can't have nil ID")
	}
	if o.SourceObjectID == uuid.Nil {
		return ucerr.New("SourceObjectID can't have nil ID")
	}
	if o.TargetObjectID == uuid.Nil {
		return ucerr.New("TargetObjectID can't have nil ID")
	}
	return ucerr.Wrap(o.BaseModel.Validate())
}

// UserObject is a limited view of the `users` table used by the AuthZ service.
// To avoid a dependency on IDP packages, AuthZ uses this stub structure to load a limited slice of User
// objects for authz purposes instead of depending on `idp/internal/storage`.`
type UserObject struct {
	ucdb.BaseModel
}
