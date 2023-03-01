// NOTE: automatically generated file -- DO NOT EDIT

package authz

import (
	"github.com/gofrs/uuid"

	"userclouds.com/infra/ucerr"
)

// Validate implements Validateable
func (o *Object) Validate() error {
	if err := o.BaseModel.Validate(); err != nil {
		return ucerr.Wrap(err)
	}
	if o.TypeID == uuid.Nil {
		return ucerr.Errorf("Object.TypeID (%v) can't be nil", o.ID)
	}
	return nil
}
