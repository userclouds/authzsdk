package pagination

import (
	"net/url"
	"strconv"
)

// Option defines a method of passing optional args to paginated List APIs
type Option interface {
	apply(*Options)
}

// Order is a direction in which a view on a collection can be sorted, mapping to ASC/DESC in SQL.
type Order int

// Order can be either ascending or descending.
const (
	OrderAscending Order = 0 // Default
	// OrderDescending Order = 1 // TOOD: implement
)

// Key is a field in the collection in which a view can be sorted or filtered.
type Key string

// TODO: we only support iterating by ID right now, which is the default for all collections.
const (
	KeyDefault Key = ""
)

// Direction indicates that results should be fetched forward through the view starting after the cursor (not including the cursor),
// or backward up to (but not including) the cursor.
type Direction int

// Direction can either be forward or backward.
const (
	DirectionForward Direction = 0 // Default
	// DirectionBackward Direction = 1 // TODO: implement
)

// Cursor is an opaque string that represents a place to start iterating from.
type Cursor string

// Cursor sentinel values.
const (
	CursorBegin Cursor = ""    // Default cursor value which indicates the beginning of a collection
	CursorEnd   Cursor = "end" // Special cursor value which indicates the end of the collection
)

// Options represents a baked set of 'Option's.
type Options struct {
	// Parameters that describe how a collection should be viewed.
	sortKey   Key
	sortOrder Order
	// TODO: implement filter as a set of key-value predicates, e.g. a triple of ( key name, operation ['prefix match', 'equals', etc], value ).
	// filters []Filter

	// Parameters that describe where in a view & how to fetch results, i.e. the iterator.
	cursor    Cursor
	limit     int
	direction Direction
}

type optFunc func(*Options)

func (o optFunc) apply(opts *Options) { o(opts) }

// SortKey optionally specifies which field of the collection should be used to sort results in the view.
func SortKey(key Key) Option {
	return optFunc(func(os *Options) { os.sortKey = key })
}

// SortOrder optionally specifies which way a view on a collection should be sorted.
func SortOrder(order Order) Option {
	return optFunc(func(os *Options) { os.sortOrder = order })
}

// StartingAfter iterates the collection starting after (but not including) the value at the cursor.
// It can be used to fetch the "current" page starting at a cursor, as well as to iterate the next page
// by passing in the cursor of the last item on teh page.
func StartingAfter(cursor Cursor) Option {
	return optFunc(func(os *Options) { os.cursor = cursor; os.direction = DirectionForward })
}

// EndingBefore iterates the collection's values before (but not including) the value at the cursor.
// It is commonly used to implement "Prev" functionality.
// TODO: implement backward iteration.
// func EndingBefore(cursor string) Option {
// 	return optFunc(func(os *Options) { os.cursor = cursor; os.direction = DirectionBackward })
// }

// Limit specifies how many results to fetch at once.
func Limit(limit int) Option {
	return optFunc(func(os *Options) { os.limit = limit })
}

// ApplyOptions applies a series of Option objects into an Options struct.
func ApplyOptions(os []Option) Options {
	opts := Options{}
	for _, o := range os {
		o.apply(&opts)
	}
	return opts
}

// Query converts the options into HTTP GET query parameters.
func (opts Options) Query() url.Values {
	query := url.Values{}
	if opts.cursor != CursorBegin && opts.direction == DirectionForward {
		query.Add("starting_after", string(opts.cursor))
	}
	// TODO: handle backward iteration
	// else if opts.direction == DirectionBackward {
	// 	query.Add("ending_before", opts.cursor)
	// }

	if opts.limit > 0 {
		query.Add("limit", strconv.Itoa(opts.limit))
	}

	// TODO: support sortKey, sortOrder, etc.

	return query
}

// LimitCOMPAT gets the specified limit, or DefaultLimit if not specified.
// TODO: remove when we pull the compat code from authz client; this code is
// possibly fragile if the server constant disagrees with the client constant.
func (opts Options) LimitCOMPAT() int {
	if opts.limit == 0 {
		return DefaultLimit
	}
	return opts.limit
}

// StartingAfterCOMPAT gets the starting cursor value.
// TODO: remove when we pull the compat code from authz client; this is really fragile
// and assumes the cursor is an ID
func (opts Options) StartingAfterCOMPAT() string {
	return string(opts.cursor)
}
