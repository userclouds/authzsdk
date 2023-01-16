package pagination

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/gofrs/uuid"

	"userclouds.com/infra/ucerr"
)

// Paginator represents a configured paginator, based on a set of Options and defaults
// derived from those options
type Paginator struct {
	cursor               Cursor       // set via StartingAfter or EndingBefore option, defaults to CursorBegin
	direction            Direction    // set via StartingAfter or EndingBefore option, defaults to DirectionForward
	backwardDirectionSet bool         // set via StartingAfter option
	forwardDirectionSet  bool         // set via EndingBefore option
	hasResultType        bool         // set if type of result has been specified
	limit                int          // set via Limit option or defaulted to DefaultLimit
	sortKey              Key          // set via SortKey option, defaults to "id"
	sortOrder            Order        // set via SortOrder option, defaults to SortAscending
	filter               string       // set via Filter option
	supportedKeys        SortableKeys // set based on type of result
	anyDuplicateKeys     bool         // set as part of initialization and validation of sort keys
	anyUnsupportedKeys   bool         // set as part of initialization and validation of sort keys
	options              []Option     // collection of options used to produce the Paginator
	version              Version      // the pagination request version
}

// ApplyOptions initializes and validates a Paginator from a series of Option objects
func ApplyOptions(options ...Option) (*Paginator, error) {
	p := Paginator{
		sortKey:       Key("id"),
		sortOrder:     OrderAscending,
		supportedKeys: SortableKeys{},
		version:       Version2,
	}

	for _, option := range options {
		option.apply(&p)
	}

	if p.limit == 0 {
		p.limit = DefaultLimit
	}

	if !p.backwardDirectionSet && !p.forwardDirectionSet {
		p.forwardDirectionSet = true
		p.cursor = CursorBegin
	}

	if p.forwardDirectionSet {
		p.direction = DirectionForward
	} else {
		p.direction = DirectionBackward
	}

	if p.version == Version1 {
		// for this version, cursor must either be CursorBegin or a raw UUID
		if p.cursor != CursorBegin {
			if _, err := uuid.FromString(string(p.cursor)); err == nil {
				p.cursor = Cursor(fmt.Sprintf("id:%v", p.cursor))
			} else {
				// the cursor is invalid
				p.cursor = Cursor(fmt.Sprintf("invalid_cursor%v", p.cursor))
			}
		}
	}

	uniqueKeys := map[string]bool{}
	supportedKeys := SortableKeys{}
	for _, key := range strings.Split(string(p.sortKey), ",") {
		if uniqueKeys[key] {
			p.anyDuplicateKeys = true
		} else if p.HasResultType() {
			if validator, found := p.supportedKeys[key]; found {
				supportedKeys[key] = validator
			} else {
				p.anyUnsupportedKeys = true
			}
		}
		uniqueKeys[key] = true
	}
	p.supportedKeys = supportedKeys

	if err := p.Validate(); err != nil {
		return nil, err
	}

	return &p, nil
}

// AdvanceCursor will advance the currrent cursor based on the direction of iteration;
// if we are moving forward, it will use the next cursor if one exists, or otherwise
// will attempt to use the prev cursor. True is returned if we were able to advance in
// the desired direction.
func (p *Paginator) AdvanceCursor(rf ResponseFields) bool {
	if p.IsForward() {
		if rf.HasNext {
			p.cursor = rf.Next
			return true
		}
	} else if rf.HasPrev {
		p.cursor = rf.Prev
		return true
	}

	return false
}

// GetCursor returns the current Cursor
func (p Paginator) GetCursor() Cursor {
	return p.cursor
}

// GetLimit returns the specified limit
func (p Paginator) GetLimit() int {
	return p.limit
}

// GetOptions returns the underlying options used to initialize the paginator
func (p Paginator) GetOptions() []Option {
	return p.options
}

// GetVersion returns the version of the pagination request
func (p Paginator) GetVersion() Version {
	return p.version
}

// HasResultType returns true if the result type has been provided
func (p Paginator) HasResultType() bool {
	return p.hasResultType
}

// IsForward returns true if the paginator is configured to page forward
func (p Paginator) IsForward() bool {
	return p.direction == DirectionForward
}

// Query converts the paginator settings into HTTP GET query parameters.
func (p Paginator) Query() url.Values {
	query := url.Values{}

	if p.IsForward() {
		query.Add("starting_after", string(p.cursor))
	} else {
		query.Add("ending_before", string(p.cursor))
	}

	if p.limit > 0 {
		query.Add("limit", strconv.Itoa(p.limit))
	}

	query.Add("sort_key", string(p.sortKey))

	query.Add("sort_order", string(p.sortOrder))

	query.Add("version", fmt.Sprintf("%v", p.version))

	return query
}

// ValidateCursor validates the passed in Cursor, making sure that each key:value pair key is unique
// and supported, and that the associated value is valid
func (p Paginator) ValidateCursor(c Cursor) error {
	if c == CursorBegin {
		if !p.IsForward() {
			return ucerr.New("CursorBegin is not a valid cursor when paginating backwards")
		}
		return nil
	}

	if c == CursorEnd {
		if p.IsForward() {
			return ucerr.New("CursorEnd is not a valid cursor when paginating forwards")
		}
		return nil
	}

	uniqueKeys := map[string]bool{}
	for _, keyValue := range strings.Split(string(c), ",") {
		pair := strings.Split(keyValue, ":")

		if len(pair) != 2 {
			return ucerr.Errorf("cursor key:value pair is invalid: '%s'", keyValue)
		}

		if uniqueKeys[pair[0]] {
			return ucerr.Errorf("cursor key:value key is a duplicate: '%s'", keyValue)
		}
		uniqueKeys[pair[0]] = true

		if p.HasResultType() {
			validator, found := p.supportedKeys[pair[0]]
			if !found {
				return ucerr.Errorf("cursor key:value pair key is unsupported: '%s'", keyValue)
			}

			if !validator(pair[1]) {
				return ucerr.Errorf("cursor key:value pair value is invalid: '%s'", keyValue)
			}
		}
	}

	return nil
}

// Validate implements the Validatable interface for the Paginator type
func (p Paginator) Validate() error {
	if err := p.version.Validate(); err != nil {
		return ucerr.Wrap(err)
	}

	if p.limit <= 0 {
		return ucerr.Errorf("limit '%d' must be greater than zero", p.limit)
	}

	if p.limit > MaxLimit {
		return ucerr.Errorf("limit '%d' cannot be greater than '%d'", p.limit, MaxLimit)
	}

	if err := p.sortOrder.Validate(); err != nil {
		return ucerr.Wrap(err)
	}

	if p.forwardDirectionSet == p.backwardDirectionSet {
		return ucerr.New("we must either page forward or page backward, but not both")
	}

	if p.sortKey == "" {
		return ucerr.New("no sort keys specified")
	}

	if p.anyUnsupportedKeys {
		return ucerr.Errorf("specified sort key contains unsupported keys: %v", p.sortKey)
	}

	if p.anyDuplicateKeys {
		return ucerr.Errorf("specified sort key contains duplicate keys: %v", p.sortKey)
	}

	if err := p.ValidateCursor(p.cursor); err != nil {
		return ucerr.Wrap(err)
	}

	return nil
}
