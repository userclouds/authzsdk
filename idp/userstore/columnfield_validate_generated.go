// NOTE: automatically generated file -- DO NOT EDIT

package userstore

import (
	"userclouds.com/infra/ucerr"
)

// Validate implements Validateable
func (o ColumnField) Validate() error {
	if err := o.Type.Validate(); err != nil {
		return ucerr.Wrap(err)
	}
	fieldLen := len(o.Name)
	if fieldLen < 1 || fieldLen > 128 {
		return ucerr.Friendlyf(nil, "ColumnField.Name length has to be between 1 and 128 (length: %v)", fieldLen)
	}
	return nil
}
