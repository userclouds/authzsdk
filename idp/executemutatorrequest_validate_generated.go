// NOTE: automatically generated file -- DO NOT EDIT

package idp

import (
	"userclouds.com/infra/ucerr"
)

// Validate implements Validateable
func (o ExecuteMutatorRequest) Validate() error {
	if o.MutatorID.IsNil() {
		return ucerr.Friendlyf(nil, "ExecuteMutatorRequest.MutatorID can't be nil")
	}
	return nil
}
