package pagination

import (
	"net/url"
	"strconv"

	"userclouds.com/infra/ucerr"
)

// NewPaginatorFromQuery applies any default options and any additional options from parsing
// the query to produce a Paginator instance, validates that instance, and returns it if valid
func NewPaginatorFromQuery(query url.Values, defaultOptions ...Option) (*Paginator, error) {
	options := []Option{}

	// since we apply options in order, make sure defaults are applied first
	options = append(options, defaultOptions...)

	if query.Has("starting_after") {
		options = append(options, StartingAfter(Cursor(query.Get("starting_after"))))
	}

	if query.Has("ending_before") {
		options = append(options, EndingBefore(Cursor(query.Get("ending_before"))))
	}

	if query.Has("limit") {
		limit, err := strconv.Atoi(query.Get("limit"))
		if err != nil {
			return nil, ucerr.Friendlyf(err, "error parsing 'limit' argument")
		}
		options = append(options, Limit(limit))
	}

	if query.Has("sort_key") {
		options = append(options, SortKey(Key(query.Get("sort_key"))))
	}

	if query.Has("sort_order") {
		options = append(options, SortOrder(Order(query.Get("sort_order"))))
	}

	if query.Has("version") {
		version, err := strconv.Atoi(query.Get("version"))
		if err != nil {
			return nil, ucerr.Friendlyf(err, "error parsing 'version' argument")
		}
		options = append(options, requestVersion(Version(version)))
	} else {
		options = append(options, requestVersion(Version1))
	}

	pager, err := ApplyOptions(options...)
	if err != nil {
		return nil, ucerr.Friendlyf(err, "paginator settings are invalid")
	}

	return pager, nil
}
