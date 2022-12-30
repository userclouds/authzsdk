package userstore

import (
	"github.com/gofrs/uuid"

	"userclouds.com/infra/ucerr"
)

// ColumnAccessor represents a customer-defined view / permissions policy on a column
type ColumnAccessor struct {
	ID uuid.UUID `json:"id" validate:"notnil"`

	// Friendly ID, must be unique?
	Name string `json:"name" validate:"notempty"`

	// the columns that are accessed here
	ColumnIDs []uuid.UUID `json:"column_ids" validate:"notnil"`

	// makes decisions about who can access this particular view of this field
	AccessPolicyID uuid.UUID `json:"access_policy_id" validate:"notnil"`

	// transforms the value of this field before it is returned to the client
	TransformationPolicyID uuid.UUID `json:"transformation_policy_id" validate:"notnil"`
}

func (o *ColumnAccessor) extraValidate() error {
	if len(o.ColumnIDs) == 0 {
		return ucerr.Errorf("ColumnAccessor.ColumnIDs (%v) can't be empty", o.ID)
	}
	return nil
}

//go:generate genvalidate ColumnAccessor

//go:generate gendbjson ColumnAccessor

// ColumnAccessors is simply a slice of ColumnAccessor, and this type exists
// soley so that we can hang Scan() and Value() on it to make the db layer work.
type ColumnAccessors []ColumnAccessor

//go:generate gendbjson ColumnAccessors
