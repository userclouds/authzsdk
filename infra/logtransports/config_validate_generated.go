// NOTE: automatically generated file -- DO NOT EDIT

package logtransports

import (
	"userclouds.com/infra/ucerr"
)

// Validate implements Validateable
func (o Config) Validate() error {
	for _, item := range o.Transports {
		if err := item.Validate(); err != nil {
			return ucerr.Wrap(err)
		}
	}
	// .extraValidate() lets you do any validation you can't express in codegen tags yet
	if err := o.extraValidate(); err != nil {
		return ucerr.Wrap(err)
	}
	return nil
}
