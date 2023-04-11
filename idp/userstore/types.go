package userstore

import (
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/gofrs/uuid"

	"userclouds.com/infra/ucerr"
)

// ColumnType is an enum for supported column types
type ColumnType int

// ColumnType constants (leaving gaps intentionally to allow future related types to be grouped)
// NOTE: keep in sync with mapColumnType defined in TenantUserStoreConfig.tsx
const (
	ColumnTypeInvalid ColumnType = 0

	ColumnTypeBoolean ColumnType = 1

	ColumnTypeString ColumnType = 100

	ColumnTypeTimestamp ColumnType = 200

	ColumnTypeUUID ColumnType = 300
)

//go:generate genconstant ColumnType

// ColumnIndexType is an enum for supported column index types
type ColumnIndexType int

const (
	// ColumnIndexTypeNone is the default value
	ColumnIndexTypeNone ColumnIndexType = iota

	// ColumnIndexTypeIndexed indicates that the column should be indexed
	ColumnIndexTypeIndexed

	// ColumnIndexTypeUnique indicates that the column should be indexed and unique
	ColumnIndexTypeUnique
)

//go:generate genconstant ColumnIndexType

// Column represents a single field/column/value to be collected/stored/managed
// in the user data store of a tenant.
type Column struct {
	// Columns may be renamed, but their ID cannot be changed.
	ID           uuid.UUID       `json:"id"`
	Name         string          `json:"name" validate:"notempty"`
	Type         ColumnType      `json:"type"`
	DefaultValue string          `json:"default_value"`
	IndexType    ColumnIndexType `json:"index_type"`
}

var validIdentifier = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_-]*$`)

const maxIdentifierLength = 128

func (c *Column) extraValidate() error {

	if len(c.Name) > maxIdentifierLength || !validIdentifier.MatchString(string(c.Name)) {
		return ucerr.Friendlyf(nil, `"%s" is not a valid column name`, c.Name)
	}

	return nil
}

//go:generate genvalidate Column

// Equals returns true if the two columns are equal
func (c *Column) Equals(other *Column) bool {
	return (c.ID == other.ID || c.ID == uuid.Nil || other.ID == uuid.Nil) &&
		c.Name == other.Name &&
		c.Type == other.Type &&
		c.DefaultValue == other.DefaultValue &&
		c.IndexType == other.IndexType
}

// Record is a single "row" of data containing 0 or more Columns from userstore's schema
// The key is the name of the column
type Record map[string]interface{}

//go:generate gendbjson Record

// GetColumnType returns the ColumnType for the given value
func GetColumnType(i interface{}) ColumnType {
	switch i.(type) {
	case string:
		return ColumnTypeString
	case time.Time:
		return ColumnTypeTimestamp
	case bool:
		return ColumnTypeBoolean
	case uuid.UUID:
		return ColumnTypeUUID
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
			if t := GetColumnType(i); t == ColumnTypeInvalid {
				return ucerr.Errorf("unknown type for Record[%s]: %T", k, i)
			}
		}
	}
	return nil
}

func stringArraysEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}

	sort.Strings(a)
	sort.Strings(b)

	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}

	return true
}

// Accessor represents a customer-defined view / permissions policy on a column
type Accessor struct {
	ID uuid.UUID `json:"id"`

	// Friendly ID, must be unique?
	Name string `json:"name" validate:"notempty"`

	// Description of the accessor
	Description string `json:"description"`

	// Version of the accessor
	Version int `json:"version"`

	// the columns that are accessed here
	ColumnNames []string `json:"column_names" validate:"skip"`

	// makes decisions about who can access this particular view of this field
	AccessPolicyID uuid.UUID `json:"access_policy_id" validate:"notnil"`

	// transforms the value of this field before it is returned to the client
	TransformationPolicyID uuid.UUID `json:"transformation_policy_id" validate:"notnil"`

	// Configuration for selectors for this accessor
	SelectorConfig UserSelectorConfig `json:"selector_config"`
}

func (o *Accessor) extraValidate() error {

	if len(o.Name) > maxIdentifierLength || !validIdentifier.MatchString(string(o.Name)) {
		return ucerr.Friendlyf(nil, `"%s" is not a valid accessor name`, o.Name)
	}

	if len(o.ColumnNames) == 0 {
		return ucerr.Errorf("Accessor.ColumnNames (%v) can't be empty", o.ID)
	}
	columnNameMap := map[string]bool{}
	for i := range o.ColumnNames {
		if _, found := columnNameMap[o.ColumnNames[i]]; found {
			return ucerr.Errorf("duplicate name '%v' in ColumnNames", o.ColumnNames[i])
		}
		columnNameMap[o.ColumnNames[i]] = true
	}
	return nil
}

// Equals returns true if the two accessors are equal
func (o *Accessor) Equals(other *Accessor) bool {
	if o == nil && other == nil {
		return true
	}
	if o == nil || other == nil {
		return false
	}
	return (o.ID == other.ID || o.ID == uuid.Nil || other.ID == uuid.Nil) &&
		o.Name == other.Name &&
		o.Description == other.Description &&
		o.Version == other.Version &&
		stringArraysEqual(o.ColumnNames, other.ColumnNames) &&
		o.AccessPolicyID == other.AccessPolicyID &&
		o.TransformationPolicyID == other.TransformationPolicyID &&
		o.SelectorConfig.WhereClause == other.SelectorConfig.WhereClause
}

//go:generate genvalidate Accessor

// Mutator represents a customer-defined permissions policy for updating columns in userstore
type Mutator struct {
	ID uuid.UUID `json:"id"`

	// Friendly ID, must be unique
	Name string `json:"name" validate:"notempty"`

	// Description of the mutator
	Description string `json:"description"`

	// Version of the mutator
	Version int `json:"version"`

	// The columns that are updated here
	ColumnNames []string `json:"column_names" validate:"skip"`

	// Decides who can update these columns
	AccessPolicyID uuid.UUID `json:"access_policy_id" validate:"notnil"`

	// Validates the data before it is written to the userstore
	ValidationPolicyID uuid.UUID `json:"validation_policy_id" validate:"notnil"`

	// Configuration for selectors for this mutator
	SelectorConfig UserSelectorConfig `json:"selector_config"`
}

func (o *Mutator) extraValidate() error {

	if len(o.Name) > maxIdentifierLength || !validIdentifier.MatchString(string(o.Name)) {
		return ucerr.Friendlyf(nil, `"%s" is not a valid mutator name`, o.Name)
	}

	if len(o.ColumnNames) == 0 {
		return ucerr.Errorf("Mutator.ColumnNames (%v) can't be empty", o.ID)
	}
	columnNameMap := map[string]bool{}
	for i := range o.ColumnNames {
		if _, found := columnNameMap[o.ColumnNames[i]]; found {
			return ucerr.Errorf("duplicate name '%v' in ColumnNames", o.ColumnNames[i])
		}
		columnNameMap[o.ColumnNames[i]] = true
	}
	return nil
}

// Equals returns true if the two mutators are equal
func (o *Mutator) Equals(other *Mutator) bool {
	if o == nil && other == nil {
		return true
	}
	if o == nil || other == nil {
		return false
	}
	return (o.ID == other.ID || o.ID == uuid.Nil || other.ID == uuid.Nil) &&
		o.Name == other.Name &&
		o.Description == other.Description &&
		o.Version == other.Version &&
		stringArraysEqual(o.ColumnNames, other.ColumnNames) &&
		o.AccessPolicyID == other.AccessPolicyID &&
		o.ValidationPolicyID == other.ValidationPolicyID &&
		o.SelectorConfig.WhereClause == other.SelectorConfig.WhereClause
}

//go:generate genvalidate Mutator

// UserSelectorValues are the values passed for the UserSelector of an accessor or mutator
type UserSelectorValues []interface{}

// UserSelectorConfig is the configuration for a UserSelector
type UserSelectorConfig struct {
	WhereClause string `json:"where_clause" validate:"notempty"`
}

func (u UserSelectorConfig) extraValidate() error {
	// make sure the where clause only contains tokens for clauses of the form "{column_id} operator ? [conjunction {column_id} operator ?]*"
	// e.g. "{id} = ANY (?) OR {phone_number} LIKE ?"
	columnsRE := regexp.MustCompile(`{[a-zA-Z0-9_-]+}`)
	operatorRE := regexp.MustCompile(`(?i) (=|<|>|<=|>=|!=|LIKE|ANY)`)
	valuesRE := regexp.MustCompile(`\?|\(\?\)`)
	conjunctionRE := regexp.MustCompile(`(?i) (OR|AND) `)
	if s := strings.TrimSpace(conjunctionRE.ReplaceAllString(operatorRE.ReplaceAllString(valuesRE.ReplaceAllString(columnsRE.ReplaceAllString(u.WhereClause, ""), ""), ""), "")); s != "" {
		return ucerr.Friendlyf(nil, `invalid or unsupported SQL in UserSelectorConfig.WhereClause: "%s", near "%s"`, u.WhereClause, strings.Split(s, " ")[0])
	}
	return nil
}

//go:generate gendbjson UserSelectorConfig

//go:generate genvalidate UserSelectorConfig
