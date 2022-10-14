package pagination

import (
	"net/url"
	"strconv"

	"github.com/gofrs/uuid"

	"userclouds.com/infra/ucerr"
)

// ParseQuery parses the standard HTTP GET parameters from the URL's query,
// `starting_after` (cursor) and `limit` (# results).
func ParseQuery(query url.Values) (uuid.UUID, int, error) {
	var err error

	startingAfter := StartingID
	limit := DefaultLimit

	if startingAfterStr := query.Get("starting_after"); startingAfterStr != "" {
		if startingAfter, err = uuid.FromString(startingAfterStr); err != nil {
			return uuid.Nil, 0, ucerr.Friendlyf(err, "error parsing 'starting_after' argument")
		}
	}

	if limitStr := query.Get("limit"); limitStr != "" {
		if limit, err = strconv.Atoi(limitStr); err != nil {
			return uuid.Nil, 0, ucerr.Friendlyf(err, "error parsing 'limit' argument")
		}

		if limit <= 0 || limit > MaxLimit {
			return uuid.Nil, 0, ucerr.Friendlyf(nil, "'limit' argument must be greater than 0 and less than %d", MaxLimit)
		}
	}

	return startingAfter, limit, nil
}

// NewOptionsFromQuery parses the query and returns Option values.
func NewOptionsFromQuery(query url.Values) ([]Option, error) {
	startingAfter, limit, err := ParseQuery(query)
	if err != nil {
		return nil, ucerr.Wrap(err)
	}
	cursor := CursorBegin
	if startingAfter != uuid.Nil {
		cursor = Cursor(startingAfter.String())
	}

	return []Option{StartingAfter(cursor), Limit(limit)}, nil
}
