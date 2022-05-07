// NOTE: automatically generated file -- DO NOT EDIT

package userstore

import "userclouds.com/infra/ucerr"

// MarshalText implements encoding.TextMarshaler (for JSON)
func (t FieldType) MarshalText() ([]byte, error) {
	switch t {
	case FieldTypeInvalid:
		return []byte("invalid"), nil
	case FieldTypeString:
		return []byte("string"), nil
	case FieldTypeTimestamp:
		return []byte("timestamp"), nil
	default:
		return nil, ucerr.Errorf("unknown value %d", t)
	}
}

// UnmarshalText implements encoding.TextMarshaler (for JSON)
func (t *FieldType) UnmarshalText(b []byte) error {
	s := string(b)
	switch s {
	case "invalid":
		*t = FieldTypeInvalid
	case "string":
		*t = FieldTypeString
	case "timestamp":
		*t = FieldTypeTimestamp
	default:
		return ucerr.Errorf("unknown value %s", s)
	}
	return nil
}

// just here for easier debugging
func (t FieldType) String() string {
	bs, err := t.MarshalText()
	if err != nil {
		return err.Error()
	}
	return string(bs)
}
