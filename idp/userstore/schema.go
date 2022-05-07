package userstore

import (
	"time"

	"github.com/gofrs/uuid"

	"userclouds.com/infra/ucerr"
)

// Schema defines the format of the User Data Store/Vault for a given tenant.
type Schema struct {
	Fields []Field `json:"fields"`
}

//go:generate gendbjson Schema

//go:generate genvalidate Schema

// FieldType is an enum for supported field types
type FieldType int

// FieldType constants (leaving gaps intentionally to allow future related types to be grouped)
// NOTE: keep in sync with mapFieldType defined in TenantUserStoreConfig.tsx
const (
	FieldTypeInvalid FieldType = 0

	FieldTypeString FieldType = 100

	FieldTypeTimestamp FieldType = 200
)

//go:generate genconstant FieldType

// Validate implements Validateable
func (ft FieldType) Validate() error {
	switch ft {
	case FieldTypeString:
		fallthrough
	case FieldTypeTimestamp:
		return nil
	}
	return ucerr.Errorf("invalid FieldType: %s", ft.String())
}

// Field represents a single field/column/value to be collected/stored/managed
// in the user data store of a tenant.
type Field struct {
	// Fields may be renamed, but their ID cannot be changed.
	ID   uuid.UUID `json:"id" validate:"notnil"`
	Name string    `json:"name" validate:"notempty"`
	Type FieldType `json:"type"`
}

//go:generate genvalidate Field

// Record is a single "row" of data containing 0 or more Fields that adhere to a Schema.
// The key is the Field UUID, since names can change but IDs are stable.
type Record map[uuid.UUID]interface{}

//go:generate gendbjson Record

func getFieldType(i interface{}) FieldType {
	switch i.(type) {
	case string:
		return FieldTypeString
	case time.Time:
		return FieldTypeTimestamp
	default:
		return FieldTypeInvalid
	}
}

// Validate implements Validateable and ensures that a Record has fields
// which consist only of valid Field Types.
// TODO: need a Validation method that validates against a particular schema.
func (r Record) Validate() error {
	for k, i := range r {
		if t := getFieldType(i); t == FieldTypeInvalid {
			return ucerr.Errorf("unknown type for Record[%s]: %T", k, i)
		}
	}
	return nil
}

// FixupAndValidate validates a record against a schema, and fixes up types in the record
// to match the schema when possible (e.g. since JSON and other serde formats don't preserve Go types,
// this attempts to parse rich types from strings as needed).
// This method should be called whenever a Record is deserialized.
func (r Record) FixupAndValidate(s *Schema) error {
	for k, i := range r {
		var field *Field
		for idx := range s.Fields {
			if k == s.Fields[idx].ID {
				field = &s.Fields[idx]
				break
			}
		}
		if field == nil {
			return ucerr.Errorf("no Field in Schema with matching ID for Record[%s]", k)
		}
		actualType := getFieldType(i)
		// In JSON, times get [de-]serialized to/from strings, so we need to handle special types.
		// Other future types will need similar conversion (e.g. lat/long, phone numbers, etc)
		if field.Type == FieldTypeTimestamp && actualType == FieldTypeString {
			t, err := time.Parse(time.RFC3339, i.(string))
			if err == nil {
				r[k] = t
				actualType = getFieldType(r[k])
			}
		}
		if actualType != field.Type {
			return ucerr.Errorf("expected Record[%s] to have type %s, got %s instead", k, field.Type, actualType)
		}
	}
	return nil
}
