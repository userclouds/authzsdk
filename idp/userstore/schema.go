package userstore

import (
	"time"

	"github.com/gofrs/uuid"

	"userclouds.com/infra/ucerr"
)

// Schema defines the format of the User Data Store/Vault for a given tenant.
type Schema struct {
	Columns []Column `json:"columns"`
}

//go:generate gendbjson Schema

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

// Validate implements Validateable
func (ft ColumnType) Validate() error {
	switch ft {
	case ColumnTypeString:
		fallthrough
	case ColumnTypeTimestamp:
		return nil
	}
	return ucerr.Errorf("invalid ColumnType: %s", ft.String())
}

// Column represents a single field/column/value to be collected/stored/managed
// in the user data store of a tenant.
type Column struct {
	// Columns may be renamed, but their ID cannot be changed.
	ID   uuid.UUID  `json:"id" validate:"notnil"`
	Name string     `json:"name" validate:"notempty"`
	Type ColumnType `json:"type"`
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
		if t := getColumnType(i); t == ColumnTypeInvalid {
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
		actualType := getColumnType(i)
		// In JSON, times get [de-]serialized to/from strings, so we need to handle special types.
		// Other future types will need similar conversion (e.g. lat/long, phone numbers, etc)
		if col.Type == ColumnTypeTimestamp && actualType == ColumnTypeString {
			t, err := time.Parse(time.RFC3339, i.(string))
			if err == nil {
				r[k] = t
				actualType = getColumnType(r[k])
			}
		}
		if actualType != col.Type {
			return ucerr.Errorf("expected Record[%s] to have type %s, got %s instead", k, col.Type, actualType)
		}
	}
	return nil
}
