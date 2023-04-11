// NOTE: automatically generated file -- DO NOT EDIT

package userstore

import "userclouds.com/infra/ucerr"

// MarshalText implements encoding.TextMarshaler (for JSON)
func (t ColumnType) MarshalText() ([]byte, error) {
	switch t {
	case ColumnTypeBoolean:
		return []byte("boolean"), nil
	case ColumnTypeInvalid:
		return []byte("invalid"), nil
	case ColumnTypeString:
		return []byte("string"), nil
	case ColumnTypeTimestamp:
		return []byte("timestamp"), nil
	case ColumnTypeUUID:
		return []byte("uuid"), nil
	default:
		return nil, ucerr.Errorf("unknown value %d", t)
	}
}

// UnmarshalText implements encoding.TextMarshaler (for JSON)
func (t *ColumnType) UnmarshalText(b []byte) error {
	s := string(b)
	switch s {
	case "boolean":
		*t = ColumnTypeBoolean
	case "invalid":
		*t = ColumnTypeInvalid
	case "string":
		*t = ColumnTypeString
	case "timestamp":
		*t = ColumnTypeTimestamp
	case "uuid":
		*t = ColumnTypeUUID
	default:
		return ucerr.Errorf("unknown value %s", s)
	}
	return nil
}

// Validate implements Validateable
func (t *ColumnType) Validate() error {
	switch *t {
	case ColumnTypeBoolean:
		return nil
	case ColumnTypeString:
		return nil
	case ColumnTypeTimestamp:
		return nil
	case ColumnTypeUUID:
		return nil
	default:
		return ucerr.Errorf("unknown ColumnType value %d", *t)
	}
}

// AllColumnTypes is a slice of all ColumnType values
var AllColumnTypes = []ColumnType{
	ColumnTypeBoolean,
	ColumnTypeString,
	ColumnTypeTimestamp,
	ColumnTypeUUID,
}

// just here for easier debugging
func (t ColumnType) String() string {
	bs, err := t.MarshalText()
	if err != nil {
		return err.Error()
	}
	return string(bs)
}
