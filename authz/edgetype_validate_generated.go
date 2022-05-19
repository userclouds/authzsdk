// NOTE: automatically generated file -- DO NOT EDIT

package authz

import (
	"github.com/gofrs/uuid"

	"userclouds.com/infra/ucerr"
)

// Validate implements Validateable
func (o *EdgeType) Validate() error {
	if err := o.BaseModel.Validate(); err != nil {
		return ucerr.Wrap(err)
	}
	if o.TypeName == "" {
		return ucerr.Errorf("EdgeType.TypeName (%v) can't be empty", o.ID)
	}
	if o.SourceObjectTypeID == uuid.Nil {
		return ucerr.Errorf("EdgeType.SourceObjectTypeID (%v) can't be nil", o.ID)
	}
	if o.TargetObjectTypeID == uuid.Nil {
		return ucerr.Errorf("EdgeType.TargetObjectTypeID (%v) can't be nil", o.ID)
	}
	for i := range o.Attributes {
		if err := o.Attributes[i].Validate(); err != nil {
			return ucerr.Wrap(err)
		}
	}
	return nil
}
