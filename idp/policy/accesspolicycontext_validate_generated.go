// NOTE: automatically generated file -- DO NOT EDIT

package policy

import (
	"userclouds.com/infra/ucerr"
)

// Validate implements Validateable
func (o AccessPolicyContext) Validate() error {
	if err := o.Server.Validate(); err != nil {
		return ucerr.Wrap(err)
	}
	return nil
}
