// NOTE: automatically generated file -- DO NOT EDIT

package idp

import (
	"userclouds.com/infra/ucerr"
)

// Validate implements Validateable
func (o *UserProfile) Validate() error {
	// .extraValidate() lets you do any validation you can't express in codegen tags yet
	if err := o.extraValidate(); err != nil {
		return ucerr.Wrap(err)
	}
	if o.Email == "" {
		return ucerr.Errorf("UserProfile.Email can't be empty")
	}
	return nil
}
