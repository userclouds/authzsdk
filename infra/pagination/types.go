package pagination

import "userclouds.com/infra/ucerr"

// Cursor is an opaque string that represents a place to start iterating from.
type Cursor string

// Cursor sentinel values.
const (
	CursorBegin Cursor = ""    // Default cursor value which indicates the beginning of a collection
	CursorEnd   Cursor = "end" // Special cursor value which indicates the end of the collection
)

// Direction indicates that results should be fetched forward through the view starting after the cursor
// (not including the cursor), or backward up to (but not including) the cursor.
type Direction int

// Direction can either be forward or backward.
const (
	DirectionForward  Direction = 0 // Default
	DirectionBackward Direction = 1
)

// Key is a comma-separated list of fields in the collection in which a view can be sorted
type Key string

// KeyValueValidator is a function that returns true if the passed in string is a valid value
type KeyValueValidator func(string) bool

// Order is a direction in which a view on a collection can be sorted, mapping to ASC/DESC in SQL.
type Order string

// Order can be either ascending or descending.
const (
	OrderAscending  Order = "ascending" // Default
	OrderDescending Order = "descending"
)

// Validate implements the Validatable interface for the Order type
func (o Order) Validate() error {
	if o != OrderAscending && o != OrderDescending {
		return ucerr.Errorf("Order is unrecognized: %d", o)
	}

	return nil
}

// SortableKeys is a map from supported keys to associated KeyValueValidators
type SortableKeys map[string]KeyValueValidator

// Version represents the version of the pagination request and reply wire format. It will
// be incremented any time that the wire format has changed.
type Version int

// Supported pagination versions
const (
	Version1 Version = 1 // cursor format is "id"
	Version2 Version = 2 // cursor format is "key1:id1,...,keyN:idN"
)

// Validate implements the Validatable interface for the Version type
func (v Version) Validate() error {
	if v != Version1 && v != Version2 {
		return ucerr.Errorf("version '%v' is unsupported", v)
	}

	return nil
}
