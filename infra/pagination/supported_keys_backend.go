package pagination

import (
	"regexp"
	"strconv"
	"time"

	"github.com/gofrs/uuid"

	"userclouds.com/infra/ucerr"
)

func getValidatedBoolean(s string) (interface{}, error) {
	b, err := strconv.ParseBool(s)
	if err != nil {
		return nil, ucerr.Wrap(err)
	}

	return b, nil
}

var unescapedQuotePattern = regexp.MustCompile(`([^\\]'|[^\\]")`)

func getValidatedString(s string) (interface{}, error) {
	// we don't allow unescaped single or double quotes in the string
	if unescapedQuotePattern.MatchString(s) {
		return nil, ucerr.Errorf("string '%s' cannot have any unescaped single or double-quotes", s)
	}

	return s, nil
}

func getValidatedTimestamp(s string) (interface{}, error) {
	t, err := time.Parse(TimestampKeyTypeLayout, s)
	if err != nil {
		return nil, ucerr.Wrap(err)
	}

	return t, nil
}

func getValidatedUUID(uuidAsString string) (interface{}, error) {
	id, err := uuid.FromString(uuidAsString)
	if err != nil {
		return nil, ucerr.Wrap(err)
	}
	return id, nil
}

func supportedKeysForResult(result interface{}) KeyTypes {
	pageableType, ok := result.(PageableType)
	if ok {
		return pageableType.GetPaginationKeys()
	}

	return KeyTypes{"id": UUIDKeyType}
}

// getValidExactValue returns a value appropriate for testing for an exact match
func (kt KeyTypes) getValidExactValue(key string, value string) (interface{}, error) {
	t, found := kt[key]
	if !found {
		return nil, ucerr.Errorf("key '%s' is unsupported", key)
	}

	switch t {
	case BoolKeyType:
		return getValidatedBoolean(value)
	case StringKeyType:
		return getValidatedString(value)
	case TimestampKeyType:
		return getValidatedTimestamp(value)
	case UUIDKeyType:
		return getValidatedUUID(value)
	default:
		return nil, ucerr.Errorf("key '%s' has an unsupported key type '%v'", key, t)
	}
}

// getValidNonExactValue returns a value appropriate for testing for a non-exact match;
// this is only supported for the string type, which supports pattern matching filters
func (kt KeyTypes) getValidNonExactValue(key string, value string) (interface{}, error) {
	t, found := kt[key]
	if !found {
		return nil, ucerr.Errorf("key '%s' is unsupported", key)
	}

	switch t {
	case StringKeyType:
		return getValidatedString(value)
	case BoolKeyType:
	case TimestampKeyType:
	case UUIDKeyType:
	default:
		return nil, ucerr.Errorf("key '%s' has an unsupported key type '%v'", key, t)
	}

	return nil, ucerr.Errorf("key '%s' is of type '%v' which does not support non-exact values", key, t)
}

func (kt KeyTypes) isValidExactValue(key string, value string) error {
	_, err := kt.getValidExactValue(key, value)
	return ucerr.Wrap(err)
}

func (kt KeyTypes) isValidNonExactValue(key string, value string) error {
	_, err := kt.getValidNonExactValue(key, value)
	return ucerr.Wrap(err)
}
