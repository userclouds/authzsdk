// NOTE: automatically generated file -- DO NOT EDIT

package userstore

import "userclouds.com/infra/ucerr"

// MarshalText implements encoding.TextMarshaler (for JSON)
func (t DataType) MarshalText() ([]byte, error) {
	switch t {
	case DataTypeAddress:
		return []byte("address"), nil
	case DataTypeBirthdate:
		return []byte("birthdate"), nil
	case DataTypeBoolean:
		return []byte("boolean"), nil
	case DataTypeComposite:
		return []byte("composite"), nil
	case DataTypeDate:
		return []byte("date"), nil
	case DataTypeE164PhoneNumber:
		return []byte("e164_phonenumber"), nil
	case DataTypeEmail:
		return []byte("email"), nil
	case DataTypeInteger:
		return []byte("integer"), nil
	case DataTypeInvalid:
		return []byte(""), nil
	case DataTypePhoneNumber:
		return []byte("phonenumber"), nil
	case DataTypeSSN:
		return []byte("ssn"), nil
	case DataTypeString:
		return []byte("string"), nil
	case DataTypeTimestamp:
		return []byte("timestamp"), nil
	case DataTypeUUID:
		return []byte("uuid"), nil
	default:
		return nil, ucerr.Errorf("unknown DataType value '%s'", t)
	}
}

// UnmarshalText implements encoding.TextMarshaler (for JSON)
func (t *DataType) UnmarshalText(b []byte) error {
	s := string(b)
	switch s {
	case "address":
		*t = DataTypeAddress
	case "birthdate":
		*t = DataTypeBirthdate
	case "boolean":
		*t = DataTypeBoolean
	case "composite":
		*t = DataTypeComposite
	case "date":
		*t = DataTypeDate
	case "e164_phonenumber":
		*t = DataTypeE164PhoneNumber
	case "email":
		*t = DataTypeEmail
	case "integer":
		*t = DataTypeInteger
	case "":
		*t = DataTypeInvalid
	case "phonenumber":
		*t = DataTypePhoneNumber
	case "ssn":
		*t = DataTypeSSN
	case "string":
		*t = DataTypeString
	case "timestamp":
		*t = DataTypeTimestamp
	case "uuid":
		*t = DataTypeUUID
	default:
		return ucerr.Errorf("unknown DataType value '%s'", s)
	}
	return nil
}

// Validate implements Validateable
func (t *DataType) Validate() error {
	switch *t {
	case DataTypeAddress:
		return nil
	case DataTypeBirthdate:
		return nil
	case DataTypeBoolean:
		return nil
	case DataTypeComposite:
		return nil
	case DataTypeDate:
		return nil
	case DataTypeE164PhoneNumber:
		return nil
	case DataTypeEmail:
		return nil
	case DataTypeInteger:
		return nil
	case DataTypePhoneNumber:
		return nil
	case DataTypeSSN:
		return nil
	case DataTypeString:
		return nil
	case DataTypeTimestamp:
		return nil
	case DataTypeUUID:
		return nil
	default:
		return ucerr.Errorf("unknown DataType value '%s'", *t)
	}
}

// Enum implements Enum
func (t DataType) Enum() []interface{} {
	return []interface{}{
		"address",
		"birthdate",
		"boolean",
		"composite",
		"date",
		"e164_phonenumber",
		"email",
		"integer",
		"phonenumber",
		"ssn",
		"string",
		"timestamp",
		"uuid",
	}
}

// AllDataTypes is a slice of all DataType values
var AllDataTypes = []DataType{
	DataTypeAddress,
	DataTypeBirthdate,
	DataTypeBoolean,
	DataTypeComposite,
	DataTypeDate,
	DataTypeE164PhoneNumber,
	DataTypeEmail,
	DataTypeInteger,
	DataTypePhoneNumber,
	DataTypeSSN,
	DataTypeString,
	DataTypeTimestamp,
	DataTypeUUID,
}
