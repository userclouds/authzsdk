package userstore

import (
	"time"

	"github.com/gofrs/uuid"

	"userclouds.com/infra/ucerr"
)

// Schema defines the format of the User Data Store/Vault for a given tenant.
type Schema struct {
	Columns []Column `json:"columns,omitempty"` // the omitempty will cause us to *not* serialize `columns: null` in the JSON for an empty-and-not-initialized array
}

//go:generate genvalidate Schema

// ColumnType is an enum for supported column types
type ColumnType int

// ColumnType constants (leaving gaps intentionally to allow future related types to be grouped)
// NOTE: keep in sync with mapColumnType defined in TenantUserStoreConfig.tsx
const (
	ColumnTypeInvalid ColumnType = 0

	ColumnTypeString ColumnType = 100

	ColumnTypeTimestamp ColumnType = 200
)

//go:generate genconstant ColumnType

// Column represents a single field/column/value to be collected/stored/managed
// in the user data store of a tenant.
type Column struct {
	// Columns may be renamed, but their ID cannot be changed.
	ID           uuid.UUID  `json:"id" validate:"notnil"`
	Name         string     `json:"name" validate:"notempty"`
	Type         ColumnType `json:"type"`
	DefaultValue string     `json:"default_value,omitempty"`
	Unique       bool       `json:"unique"`
}

//go:generate genvalidate Column

// Record is a single "row" of data containing 0 or more Columns that adhere to a Schema.
// The key is the Column UUID, since names can change but IDs are stable.
type Record map[uuid.UUID]interface{}

//go:generate gendbjson Record

func getColumnType(i interface{}) ColumnType {
	switch i.(type) {
	case string:
		return ColumnTypeString
	case time.Time:
		return ColumnTypeTimestamp
	default:
		return ColumnTypeInvalid
	}
}

// Validate implements Validateable and ensures that a Record has columns
// which consist only of valid ColumnTypes.
// TODO: need a Validation method that validates against a particular schema.
func (r Record) Validate() error {
	for k, i := range r {
		if i != nil {
			if t := getColumnType(i); t == ColumnTypeInvalid {
				return ucerr.Errorf("unknown type for Record[%s]: %T", k, i)
			}
		}
	}
	return nil
}

// ValidateAgainstSchema validates a record against a schema
func (r Record) ValidateAgainstSchema(s *Schema) error {
	for k, i := range r {
		var col *Column
		for idx := range s.Columns {
			if k == s.Columns[idx].ID {
				col = &s.Columns[idx]
				break
			}
		}
		if col == nil {
			return ucerr.Errorf("no Column in Schema with matching ID for Record[%s]", k)
		}
		if i == nil {
			continue
		}
		actualType := getColumnType(i)
		if col.Type == ColumnTypeTimestamp && actualType == ColumnTypeString {
			if _, err := time.Parse(time.RFC3339, i.(string)); err == nil {
				actualType = ColumnTypeTimestamp
			}
		}
		if actualType != col.Type {
			return ucerr.Errorf("expected Record[%s] to have type %s, got %s instead", k, col.Type, actualType)
		}
	}
	return nil
}

// Accessor represents a customer-defined view / permissions policy on a column
type Accessor struct {
	ID uuid.UUID `json:"id" validate:"notnil"`

	// Friendly ID, must be unique?
	Name string `json:"name" validate:"notempty"`

	// Description of the accessor
	Description string `json:"description"`

	// Version of the accessor
	Version int `json:"version"`

	// the columns that are accessed here
	ColumnIDs []uuid.UUID `json:"column_ids" validate:"notnil"`

	// makes decisions about who can access this particular view of this field
	AccessPolicyID uuid.UUID `json:"access_policy_id" validate:"notnil"`

	// transforms the value of this field before it is returned to the client
	TransformationPolicyID uuid.UUID `json:"transformation_policy_id" validate:"notnil"`
}

func (o *Accessor) extraValidate() error {
	if len(o.ColumnIDs) == 0 {
		return ucerr.Errorf("Accessor.ColumnIDs (%v) can't be empty", o.ID)
	}
	return nil
}

//go:generate genvalidate Accessor

// Mutator represents a customer-defined permissions policy for updating columns in userstore
type Mutator struct {
	ID uuid.UUID `json:"id" validate:"notnil"`

	// Friendly ID, must be unique
	Name string `json:"name" validate:"notempty"`

	// Description of the mutator
	Description string `json:"description"`

	// Version of the mutator
	Version int `json:"version"`

	// The columns that are updated here
	ColumnIDs []uuid.UUID `json:"column_ids" validate:"notnil"`

	// Decides who can update these columns
	AccessPolicyID uuid.UUID `json:"access_policy_id" validate:"notnil"`

	// Validates the data before it is written to the userstore
	ValidationPolicyID uuid.UUID `json:"validation_policy_id" validate:"notnil"`
}

func (o *Mutator) extraValidate() error {
	if len(o.ColumnIDs) == 0 {
		return ucerr.Errorf("Mutator.ColumnIDs (%v) can't be empty", o.ID)
	}
	return nil
}

//go:generate genvalidate Mutator
