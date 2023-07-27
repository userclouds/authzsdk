package pagination

import (
	"net/http"
	"strconv"

	"userclouds.com/infra/ucerr"
)

// NewPaginatorFromRequest calls NewPaginatorFromQuery, creating a PaginationQuery from the
// request and any specified default pagination options
func NewPaginatorFromRequest(r *http.Request, defaultOptions ...Option) (*Paginator, error) {
	urlValues := r.URL.Query()

	req := QueryParams{}
	if urlValues.Has("ending_before") {
		v := urlValues.Get("ending_before")
		req.EndingBefore = &v
	}
	if urlValues.Has("filter") {
		v := urlValues.Get("filter")
		req.Filter = &v
	}
	if urlValues.Has("limit") {
		v := urlValues.Get("limit")
		req.Limit = &v
	}
	if urlValues.Has("sort_key") {
		v := urlValues.Get("sort_key")
		req.SortKey = &v
	}
	if urlValues.Has("sort_order") {
		v := urlValues.Get("sort_order")
		req.SortOrder = &v
	}
	if urlValues.Has("starting_after") {
		v := urlValues.Get("starting_after")
		req.StartingAfter = &v
	}
	if urlValues.Has("version") {
		v := urlValues.Get("version")
		req.Version = &v
	}

	p, err := NewPaginatorFromQuery(req, defaultOptions...)
	return p, ucerr.Wrap(err)
}

// Query is the interface needed by NewPaginatorFromQuery to create a Paginator
type Query interface {
	getStartingAfter() *string
	getEndingBefore() *string
	getLimit() *string
	getFilter() *string
	getSortKey() *string
	getSortOrder() *string
	getVersion() *string
}

// QueryParams is a struct that implements PaginationQuery, which can be incorporated in other request structs
// for handlers that need to take pagination query parameters
type QueryParams struct {
	StartingAfter *string `query:"starting_after"`
	EndingBefore  *string `query:"ending_before"`
	Limit         *string `query:"limit"`
	Filter        *string `query:"filter"`
	SortKey       *string `query:"sort_key"`
	SortOrder     *string `query:"sort_order"`
	Version       *string `query:"version"`
}

func (p QueryParams) getStartingAfter() *string {
	return p.StartingAfter
}

func (p QueryParams) getEndingBefore() *string {
	return p.EndingBefore
}

func (p QueryParams) getLimit() *string {
	return p.Limit
}

func (p QueryParams) getFilter() *string {
	return p.Filter
}

func (p QueryParams) getSortKey() *string {
	return p.SortKey
}

func (p QueryParams) getSortOrder() *string {
	return p.SortOrder
}

func (p QueryParams) getVersion() *string {
	return p.Version
}

// NewPaginatorFromQuery applies any default options and any additional options from parsing
// the query to produce a Paginator instance, validates that instance, and returns it if valid
func NewPaginatorFromQuery(query Query, defaultOptions ...Option) (*Paginator, error) {
	options := []Option{}

	// since we apply options in order, make sure defaults are applied first
	options = append(options, defaultOptions...)

	if query.getStartingAfter() != nil {
		options = append(options, StartingAfter(Cursor(*query.getStartingAfter())))
	}

	if query.getEndingBefore() != nil {
		options = append(options, EndingBefore(Cursor(*query.getEndingBefore())))
	}

	if query.getLimit() != nil {
		limit, err := strconv.Atoi(*query.getLimit())
		if err != nil {
			return nil, ucerr.Friendlyf(err, "error parsing 'limit' argument")
		}
		options = append(options, Limit(limit))
	}

	if query.getFilter() != nil {
		options = append(options, Filter(*query.getFilter()))
	}

	if query.getSortKey() != nil {
		options = append(options, SortKey(Key(*query.getSortKey())))
	}

	if query.getSortOrder() != nil {
		options = append(options, SortOrder(Order(*query.getSortOrder())))
	}

	if query.getVersion() != nil {
		version, err := strconv.Atoi(*query.getVersion())
		if err != nil {
			return nil, ucerr.Friendlyf(err, "error parsing 'version' argument")
		}
		options = append(options, requestVersion(Version(version)))
	}

	pager, err := ApplyOptions(options...)
	if err != nil {
		return nil, ucerr.Friendlyf(err, "paginator settings are invalid")
	}

	return pager, nil
}
