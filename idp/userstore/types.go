package userstore

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/gofrs/uuid"

	"userclouds.com/idp/userstore/selectorconfigparser"
	"userclouds.com/infra/pagination"
	"userclouds.com/infra/ucerr"
	"userclouds.com/infra/uctypes/set"
)

// DataType is an enum for supported data types
type DataType string

// DataType constants
// NOTE: keep in sync with mapDataType defined in TenantUserStoreConfig.tsx
const (
	DataTypeInvalid         DataType = ""
	DataTypeAddress         DataType = "address"
	DataTypeBirthdate       DataType = "birthdate"
	DataTypeBoolean         DataType = "boolean"
	DataTypeDate            DataType = "date"
	DataTypeEmail           DataType = "email"
	DataTypeInteger         DataType = "integer"
	DataTypeE164PhoneNumber DataType = "e164_phonenumber"
	DataTypePhoneNumber     DataType = "phonenumber"
	DataTypeSSN             DataType = "ssn"
	DataTypeString          DataType = "string"
	DataTypeTimestamp       DataType = "timestamp"
	DataTypeUUID            DataType = "uuid"
)

//go:generate genconstant DataType

// Address is a native userstore type that represents a physical address
type Address struct {
	Country            string `json:"country,omitempty"`
	Name               string `json:"name,omitempty"`
	Organization       string `json:"organization,omitempty"`
	StreetAddressLine1 string `json:"street_address_line_1,omitempty"`
	StreetAddressLine2 string `json:"street_address_line_2,omitempty"`
	DependentLocality  string `json:"dependent_locality,omitempty"`
	Locality           string `json:"locality,omitempty"`
	AdministrativeArea string `json:"administrative_area,omitempty"`
	PostCode           string `json:"post_code,omitempty"`
	SortingCode        string `json:"sorting_code,omitempty"`
}

// NewAddressSet returns a set of addresses
func NewAddressSet(items ...Address) set.Set[Address] {
	return set.New(
		func(items []Address) {
			sort.Slice(items, func(i, j int) bool {
				return fmt.Sprintf("%+v", items[i]) < fmt.Sprintf("%+v", items[j])
			})
		},
		items...,
	)
}

//go:generate gendbjson Address

// ColumnIndexType is an enum for supported column index types
type ColumnIndexType string

const (
	// ColumnIndexTypeNone is the default value
	ColumnIndexTypeNone ColumnIndexType = "none"

	// ColumnIndexTypeIndexed indicates that the column should be indexed
	ColumnIndexTypeIndexed ColumnIndexType = "indexed"

	// ColumnIndexTypeUnique indicates that the column should be indexed and unique
	ColumnIndexTypeUnique ColumnIndexType = "unique"
)

//go:generate genconstant ColumnIndexType

// Column represents a single field/column/value to be collected/stored/managed
// in the user data store of a tenant.
type Column struct {
	// Columns may be renamed, but their ID cannot be changed.
	ID           uuid.UUID       `json:"id"`
	Name         string          `json:"name" validate:"length:1,128" required:"true"`
	Type         DataType        `json:"type" required:"true"`
	IsArray      bool            `json:"is_array" required:"true"`
	DefaultValue string          `json:"default_value"`
	IndexType    ColumnIndexType `json:"index_type" required:"true"`
	IsSystem     bool            `json:"is_system" description:"Whether this column is a system column. System columns cannot be deleted or modified. This property cannot be changed."`
}

var validIdentifier = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_-]*$`)

func (c *Column) extraValidate() error {

	if !validIdentifier.MatchString(string(c.Name)) {
		return ucerr.Friendlyf(nil, `"%s" is not a valid column name`, c.Name)
	}

	return nil
}

//go:generate genvalidate Column

// GetPaginationKeys is part of the pagination.PageableType interface
func (Column) GetPaginationKeys() pagination.KeyTypes {
	return pagination.KeyTypes{
		"id":      pagination.UUIDKeyType,
		"name":    pagination.StringKeyType,
		"created": pagination.TimestampKeyType,
		"updated": pagination.TimestampKeyType,
	}
}

// Equals returns true if the two columns are equal
func (c *Column) Equals(other *Column) bool {
	return (c.ID == other.ID || c.ID.IsNil() || other.ID.IsNil()) &&
		strings.EqualFold(c.Name, other.Name) &&
		c.Type == other.Type &&
		c.IsArray == other.IsArray &&
		c.DefaultValue == other.DefaultValue &&
		c.IndexType == other.IndexType &&
		c.IsSystem == other.IsSystem
}

// Record is a single "row" of data containing 0 or more Columns from userstore's schema
// The key is the name of the column
type Record map[string]interface{}

func typedValue[T any](r Record, key string, defaultValue T) T {
	if r[key] != nil {
		if value, ok := r[key].(T); ok {
			return value
		}
	}

	return defaultValue
}

// BoolValue returns a boolean value for the specified key
func (r Record) BoolValue(key string) bool {
	return typedValue(r, key, false)
}

// StringValue returns a string value for the specified key
func (r Record) StringValue(key string) string {
	return typedValue(r, key, "")
}

// UUIDValue returns a UUID value for the specified key
func (r Record) UUIDValue(key string) uuid.UUID {
	value, err := uuid.FromString(r.StringValue(key))
	if err != nil {
		return uuid.Nil
	}
	return value
}

//go:generate gendbjson Record

// ResourceID is a struct that contains a name and ID, only one of which is required to be set
type ResourceID struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

// Validate implements Validateable
func (r ResourceID) Validate() error {
	if r.ID.IsNil() && r.Name == "" {
		return ucerr.Friendlyf(nil, "either ID or Name must be set")
	}
	return nil
}

// ColumnOutputConfig is a struct that contains a column and the transformer to apply to that column
type ColumnOutputConfig struct {
	Column      ResourceID `json:"column"`
	Transformer ResourceID `json:"transformer"`
}

// GetRetentionTimeoutImmediateDeletion returns the immediate deletion retention timeout
func GetRetentionTimeoutImmediateDeletion() time.Time {
	return time.Time{}
}

// GetRetentionTimeoutIndefinite returns the indefinite retention timeout
func GetRetentionTimeoutIndefinite() time.Time {
	return time.Time{}
}

// DataLifeCycleState identifies the life-cycle state for a piece of data - either
// live or soft-deleted.
type DataLifeCycleState string

// Supported data life cycle states
const (
	DataLifeCycleStateDefault     DataLifeCycleState = ""
	DataLifeCycleStateLive        DataLifeCycleState = "live"
	DataLifeCycleStateSoftDeleted DataLifeCycleState = "softdeleted"

	// maps to softdeleted
	DataLifeCycleStatePostDelete DataLifeCycleState = "postdelete"

	// maps to live
	DataLifeCycleStatePreDelete DataLifeCycleState = "predelete"
)

//go:generate genconstant DataLifeCycleState

// GetConcrete returns the concrete data life cycle state for the given data life cycle state
func (dlcs DataLifeCycleState) GetConcrete() DataLifeCycleState {
	switch dlcs {
	case DataLifeCycleStateDefault, DataLifeCycleStatePreDelete:
		return DataLifeCycleStateLive
	case DataLifeCycleStatePostDelete:
		return DataLifeCycleStateSoftDeleted
	default:
		return dlcs
	}
}

// GetDefaultRetentionTimeout returns the default retention timeout for the data life cycle state
func (dlcs DataLifeCycleState) GetDefaultRetentionTimeout() time.Time {
	if dlcs.GetConcrete() == DataLifeCycleStateLive {
		return GetRetentionTimeoutIndefinite()
	}

	return GetRetentionTimeoutImmediateDeletion()
}

// IsLive return true if the concrete data life cycle state is live
func (dlcs DataLifeCycleState) IsLive() bool {
	return dlcs.GetConcrete() == DataLifeCycleStateLive
}

// Accessor represents a customer-defined view and permissions policy on userstore data
type Accessor struct {
	ID uuid.UUID `json:"id"`

	// Name of accessor, must be unique
	Name string `json:"name" validate:"length:1,128" required:"true"`

	// Description of the accessor
	Description string `json:"description"`

	// Version of the accessor
	Version int `json:"version"`

	// Specify whether to access live or soft-deleted data
	DataLifeCycleState DataLifeCycleState `json:"data_life_cycle_state"`

	// Configuration for which user records to return
	SelectorConfig UserSelectorConfig `json:"selector_config" required:"true"`

	// Purposes for which this accessor is used
	Purposes []ResourceID `json:"purposes" validate:"skip" required:"true"`

	// List of userstore columns being accessed and the transformers to apply to each column
	Columns []ColumnOutputConfig `json:"columns" validate:"skip" required:"true"`

	// Policy for what data is returned by this accessor, based on properties of the caller and the user records
	AccessPolicy ResourceID `json:"access_policy" validate:"skip" required:"true"`

	// Policy for token resolution in the case of transformers that tokenize data
	TokenAccessPolicy ResourceID `json:"token_access_policy,omitempty" validate:"skip"`

	IsSystem bool `json:"is_system" description:"Whether this accessor is a system accessor. System accessors cannot be deleted or modified. This property cannot be changed."`
}

func (o *Accessor) extraValidate() error {

	if !validIdentifier.MatchString(string(o.Name)) {
		return ucerr.Friendlyf(nil, `"%s" is not a valid accessor name`, o.Name)
	}

	if len(o.Columns) == 0 {
		return ucerr.Friendlyf(nil, "Accessor.Columns (%v) can't be empty", o.ID)
	}

	for _, ct := range o.Columns {
		if err := ct.Column.Validate(); err != nil {
			return ucerr.Friendlyf(err, "Each element of Accessor.Columns (%v) must have a column ID or name", o.ID)
		}

		if err := ct.Transformer.Validate(); err != nil {
			return ucerr.Friendlyf(err, "Each element of Accessor.Columns (%v) must have a transformer ID or name", o.ID)
		}
	}

	if err := o.AccessPolicy.Validate(); err != nil {
		return ucerr.Friendlyf(err, "Accessor.AccessPolicy (%v) must have an ID or name", o.ID)
	}

	if len(o.Purposes) == 0 {
		return ucerr.Friendlyf(nil, "Accessor.Purposes (%v) can't be empty", o.ID)
	}

	return nil
}

//go:generate genvalidate Accessor

// GetPaginationKeys is part of the pagination.PageableType interface
func (Accessor) GetPaginationKeys() pagination.KeyTypes {
	return pagination.KeyTypes{
		"id":      pagination.UUIDKeyType,
		"name":    pagination.StringKeyType,
		"created": pagination.TimestampKeyType,
		"updated": pagination.TimestampKeyType,
	}
}

// ColumnInputConfig is a struct that contains a column and the normalizer to use for that column
type ColumnInputConfig struct {
	Column     ResourceID `json:"column"`
	Normalizer ResourceID `json:"normalizer"`

	// Validator is deprecated in favor of Normalizer
	Validator ResourceID `json:"validator"`
}

// Mutator represents a customer-defined scope and permissions policy for updating userstore data
type Mutator struct {
	ID uuid.UUID `json:"id"`

	// Name of mutator, must be unique
	Name string `json:"name" validate:"length:1,128" required:"true"`

	// Description of the mutator
	Description string `json:"description"`

	// Version of the mutator
	Version int `json:"version"`

	// Configuration for which user records to modify
	SelectorConfig UserSelectorConfig `json:"selector_config" required:"true"`

	// The set of userstore columns to modify for each user record
	Columns []ColumnInputConfig `json:"columns" validate:"skip" required:"true"`

	// Policy for whether the data for each user record can be updated
	AccessPolicy ResourceID `json:"access_policy" validate:"skip" required:"true"`

	IsSystem bool `json:"is_system" description:"Whether this mutator is a system mutator. System mutators cannot be deleted or modified. This property cannot be changed."`
}

func (o *Mutator) extraValidate() error {

	if !validIdentifier.MatchString(string(o.Name)) {
		return ucerr.Friendlyf(nil, `"%s" is not a valid mutator name`, o.Name)
	}

	totalColumns := len(o.Columns)
	if totalColumns == 0 && !o.IsSystem {
		return ucerr.Friendlyf(nil, "Mutator with ID (%v) can't have empty Columns", o.ID)
	}

	totalValidNormalizers := 0
	totalValidValidators := 0
	for _, cv := range o.Columns {
		if err := cv.Column.Validate(); err != nil {
			return ucerr.Friendlyf(err, "Mutator with ID (%v): each element of Columns must have a column ID or name", o.ID)
		}

		if err := cv.Normalizer.Validate(); err == nil {
			totalValidNormalizers++
		}

		if err := cv.Validator.Validate(); err == nil {
			totalValidValidators++
		}
	}

	if totalValidNormalizers != totalColumns && totalValidValidators != totalColumns {
		return ucerr.Friendlyf(nil, "Mutator with ID (%v): each element of Columns must have either a normalizer or validator ID or name", o.ID)
	}

	if err := o.AccessPolicy.Validate(); err != nil {
		return ucerr.Friendlyf(err, "Mutator with ID (%v): AccessPolicy must have an ID or name", o.ID)
	}

	return nil
}

//go:generate genvalidate Mutator

// UserSelectorValues are the values passed for the UserSelector of an accessor or mutator
type UserSelectorValues []interface{}

// UserSelectorConfig is the configuration for a UserSelector
type UserSelectorConfig struct {
	WhereClause string `json:"where_clause" validate:"notempty" example:"{id} = ANY (?)"`
}

func (u UserSelectorConfig) extraValidate() error {
	return ucerr.Wrap(selectorconfigparser.ParseWhereClause(u.WhereClause))
}

//go:generate gendbjson UserSelectorConfig

//go:generate genvalidate UserSelectorConfig

// Purpose represents a customer-defined purpose for userstore columns
type Purpose struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name" validate:"length:1,128" required:"true"`
	Description string    `json:"description"`
	IsSystem    bool      `json:"is_system" description:"Whether this purpose is a system purpose. System purposes cannot be deleted or modified. This property cannot be changed."`
}

func (p *Purpose) extraValidate() error {

	if !validIdentifier.MatchString(string(p.Name)) {
		return ucerr.Friendlyf(nil, `"%s" is not a valid purpose name`, p.Name)
	}

	return nil
}

//go:generate genvalidate Purpose
