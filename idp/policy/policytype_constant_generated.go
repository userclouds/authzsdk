// NOTE: automatically generated file -- DO NOT EDIT

package policy

import "userclouds.com/infra/ucerr"

// MarshalText implements encoding.TextMarshaler (for JSON)
func (t PolicyType) MarshalText() ([]byte, error) {
	switch t {
	case PolicyTypeCompositeIntersection:
		return []byte("compositeintersection"), nil
	case PolicyTypeCompositeUnion:
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
	case "compositeintersection":
		*t = PolicyTypeCompositeIntersection
	case "compositeunion":
		*t = PolicyTypeCompositeUnion
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
	case PolicyTypeCompositeIntersection:
		return nil
	case PolicyTypeCompositeUnion:
		return nil
	default:
		return ucerr.Errorf("unknown PolicyType value '%s'", *t)
	}
}

// Enum implements Enum
func (t PolicyType) Enum() []interface{} {
	return []interface{}{
		"compositeintersection",
		"compositeunion",
	}
}

// AllPolicyTypes is a slice of all PolicyType values
var AllPolicyTypes = []PolicyType{
	PolicyTypeCompositeIntersection,
	PolicyTypeCompositeUnion,
}
