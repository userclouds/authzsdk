// NOTE: automatically generated file -- DO NOT EDIT

package authz

import (
	"github.com/gofrs/uuid"

	"userclouds.com/infra/ucerr"
)

// Validate implements Validateable
func (o *Edge) Validate() error {
	if err := o.BaseModel.Validate(); err != nil {
		return ucerr.Wrap(err)
	}
	if o.EdgeTypeID == uuid.Nil {
		return ucerr.Errorf("Edge.EdgeTypeID (%v) can't be nil", o.ID)
	}
	if o.SourceObjectID == uuid.Nil {
		return ucerr.Errorf("Edge.SourceObjectID (%v) can't be nil", o.ID)
	}
	if o.TargetObjectID == uuid.Nil {
		return ucerr.Errorf("Edge.TargetObjectID (%v) can't be nil", o.ID)
	}
	return nil
}
