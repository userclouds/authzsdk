// NOTE: automatically generated file -- DO NOT EDIT

package tokenizer

import (
	"userclouds.com/infra/ucerr"
)

// Validate implements Validateable
func (o LookupOrCreateTokensRequest) Validate() error {
	for i := range o.TransformerRIDs {
		if err := o.TransformerRIDs[i].Validate(); err != nil {
			return ucerr.Wrap(err)
		}
	}
	for i := range o.AccessPolicyRIDs {
		if err := o.AccessPolicyRIDs[i].Validate(); err != nil {
			return ucerr.Wrap(err)
		}
	}
	// .extraValidate() lets you do any validation you can't express in codegen tags yet
	if err := o.extraValidate(); err != nil {
		return ucerr.Wrap(err)
	}
	return nil
}
