// NOTE: automatically generated file -- DO NOT EDIT

package tokenizer

import (
	"userclouds.com/infra/ucerr"
)

// Validate implements Validateable
func (o TestAccessPolicyTemplateRequest) Validate() error {
	if err := o.AccessPolicyTemplate.Validate(); err != nil {
		return ucerr.Wrap(err)
	}
	if err := o.Context.Validate(); err != nil {
		return ucerr.Wrap(err)
	}
	return nil
}
