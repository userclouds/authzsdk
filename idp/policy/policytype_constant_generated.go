// NOTE: automatically generated file -- DO NOT EDIT

package policy

import "userclouds.com/infra/ucerr"

// MarshalText implements encoding.TextMarshaler (for JSON)
func (t PolicyType) MarshalText() ([]byte, error) {
	switch t {
	case PolicyTypeCompositeAnd:
		return []byte("composite_and"), nil
	case PolicyTypeCompositeIntersectionDeprecated:
		return []byte("compositeintersection"), nil
	case PolicyTypeCompositeOr:
		return []byte("composite_or"), nil
	case PolicyTypeCompositeUnionDeprecated:
		return []byte("compositeunion"), nil
	case PolicyTypeInvalid:
		return []byte("invalid"), nil
	default:
		return nil, ucerr.Errorf("unknown PolicyType value '%s'", t)
	}
}

// UnmarshalText implements encoding.TextMarshaler (for JSON)
func (t *PolicyType) UnmarshalText(b []byte) error {
	s := string(b)
	switch s {
	case "composite_and":
		*t = PolicyTypeCompositeAnd
	case "compositeintersection":
		*t = PolicyTypeCompositeIntersectionDeprecated
	case "composite_or":
		*t = PolicyTypeCompositeOr
	case "compositeunion":
		*t = PolicyTypeCompositeUnionDeprecated
	case "invalid":
		*t = PolicyTypeInvalid
	default:
		return ucerr.Errorf("unknown PolicyType value '%s'", s)
	}
	return nil
}

// Validate implements Validateable
func (t *PolicyType) Validate() error {
	switch *t {
	case PolicyTypeCompositeAnd:
		return nil
	case PolicyTypeCompositeIntersectionDeprecated:
		return nil
	case PolicyTypeCompositeOr:
		return nil
	case PolicyTypeCompositeUnionDeprecated:
		return nil
	default:
		return ucerr.Errorf("unknown PolicyType value '%s'", *t)
	}
}

// Enum implements Enum
func (t PolicyType) Enum() []interface{} {
	return []interface{}{
		"composite_and",
		"compositeintersection",
		"composite_or",
		"compositeunion",
	}
}

// AllPolicyTypes is a slice of all PolicyType values
var AllPolicyTypes = []PolicyType{
	PolicyTypeCompositeAnd,
	PolicyTypeCompositeIntersectionDeprecated,
	PolicyTypeCompositeOr,
	PolicyTypeCompositeUnionDeprecated,
}
