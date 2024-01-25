// NOTE: automatically generated file -- DO NOT EDIT

package policy

import (
	"userclouds.com/infra/ucerr"
)

// Validate implements Validateable
func (o AccessPolicy) Validate() error {
	fieldLen := len(o.Name)
	if fieldLen < 1 || fieldLen > 128 {
		return ucerr.Friendlyf(nil, "AccessPolicy.Name length has to be between 1 and 128 (length: %v)", fieldLen)
	}
	if err := o.PolicyType.Validate(); err != nil {
		return ucerr.Wrap(err)
	}
	// .extraValidate() lets you do any validation you can't express in codegen tags yet
	if err := o.extraValidate(); err != nil {
		return ucerr.Wrap(err)
	}
	return nil
}
