// NOTE: automatically generated file -- DO NOT EDIT

package policy

import (
	"userclouds.com/infra/ucerr"
)

// Validate implements Validateable
func (o ServerContext) Validate() error {
	if err := o.Resolver.Validate(); err != nil {
		return ucerr.Wrap(err)
	}
	return nil
}
