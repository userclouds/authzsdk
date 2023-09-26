package pagination

import (
	"regexp"
	"strings"

	"userclouds.com/infra/ucerr"
)

type nodeType string

const (
	composite nodeType = "composite"
	leaf      nodeType = "leaf"
	nested    nodeType = "nested"
)

type operator string

const (
	and operator = "AND"
	eq  operator = "EQ"
	ge  operator = "GE"
	gt  operator = "GT"
	le  operator = "LE"
	il  operator = "IL"
	has operator = "HAS"
	lk  operator = "LK"
	lt  operator = "LT"
	ne  operator = "NE"
	nl  operator = "NL"
	or  operator = "OR"
)

func (o operator) isArrayOperator() bool {
	switch o {
	case has:
	default:
		return false
	}

	return true
}

func (o operator) isComparisonOperator() bool {
	switch o {
	case eq:
	case ge:
	case gt:
	case le:
	case lt:
	case ne:
	default:
		return false
	}

	return true
}

func (o operator) isPatternOperator() bool {
	switch o {
	case il:
	case lk:
	case nl:
	default:
		return false
	}

	return true
}

func (o operator) isLogicalOperator() bool {
	switch o {
	case and:
	case or:
	default:
		return false
	}

	return true
}

func (o operator) isLeafOperator() bool {
	return o.isComparisonOperator() || o.isPatternOperator() || o.isArrayOperator()
}

func (o operator) queryString() string {
	switch o {
	case eq:
		return "="
	case gt:
		return ">"
	case lt:
		return "<"
	case ge:
		return ">="
	case has:
		return "@>"
	case le:
		return "<="
	case ne:
		return "!="
	case il:
		return "ILIKE"
	case lk:
		return "LIKE"
	case nl:
		return "NOT LIKE"
	case and:
		return "AND"
	case or:
		return "OR"
	}

	return ""
}

// FilterQuery represents a parsed query tree for a filter query
type FilterQuery struct {
	nodeType      nodeType
	key           string
	value         string
	queryOperator operator
	leftQuery     *FilterQuery
	rightQuery    *FilterQuery
}

var leafFilter = regexp.MustCompile(`^[(]'[^']+',[A-Z]+,'.*?[^\\]'[)]`)

func parseLeafQuery(s string) (*FilterQuery, string, error) {
	// s must be of the form: ('key',op,'value')...
	if !leafFilter.MatchString(s) {
		return nil, "", ucerr.Errorf("query '%s' is not of the form '('key',operator,'value')", s)
	}

	leafPrefix := leafFilter.FindString(s)
	remainder := strings.TrimPrefix(s, leafPrefix)

	leafParts := strings.SplitN(strings.TrimPrefix(leafPrefix, "('"), ",", 3)
	if len(leafParts) != 3 {
		return nil, "", ucerr.Errorf("query '%s' has %d parts, expected 3", s, len(leafParts))
	}

	key := strings.TrimSuffix(leafParts[0], "'")
	op := operator(leafParts[1])
	value := strings.TrimSuffix(strings.TrimPrefix(leafParts[2], "'"), "')")

	if !op.isLeafOperator() {
		return nil, "", ucerr.Errorf("query '%s' operator '%v' is not a leaf operator", s, op)
	}

	return &FilterQuery{
		nodeType:      leaf,
		key:           key,
		queryOperator: op,
		value:         value,
	}, remainder, nil
}

func parseFilterQuery(s string) (*FilterQuery, string, error) {
	if strings.HasPrefix(s, "('") {
		// we expect s is a leaf query
		return parseLeafQuery(s)
	}

	// query must either be nested or composite
	if !strings.HasPrefix(s, "((") {
		return nil, "", ucerr.Errorf("query '%s' is neither a nested nor composite query", s)
	}

	leftQuery, remainder, err := parseFilterQuery(strings.TrimPrefix(s, "("))
	if err != nil {
		return nil, "", ucerr.Errorf("could not parse left-most query of left clause '%s': '%v'", s, err)
	}

	if strings.HasPrefix(remainder, ")") {
		// s was a nested query
		//
		// '((leftQuery))'
		return &FilterQuery{
			nodeType:  nested,
			leftQuery: leftQuery,
		}, strings.TrimPrefix(remainder, ")"), nil
	}

	if !strings.HasPrefix(remainder, ",") {
		return nil, "", ucerr.Errorf("query '%s' is not of the form (leftQuery,operator,rightQuery)", s)
	}

	// we expect s was a composite query
	//
	// '((leftQuery),operator,(rightQuery))'
	compositeParts := strings.SplitN(remainder, ",", 3)
	if len(compositeParts) != 3 {
		return nil, "", ucerr.Errorf("query '%s' had %d parts, expected 3", s, len(compositeParts))
	}

	op := operator(compositeParts[1])
	if !op.isLogicalOperator() {
		return nil, "", ucerr.Errorf("query '%s' operator '%v' is not a logical operator", s, op)
	}

	rightQuery, remainder, err := parseFilterQuery(compositeParts[2])
	if err != nil {
		return nil, "", ucerr.Errorf("could not parse right-most query of left clause '%s': '%v'", s, err)
	}

	if !strings.HasPrefix(remainder, ")") {
		return nil, "", ucerr.Errorf("composite query '%s' does not end in ')'", s)
	}

	return &FilterQuery{
		nodeType:      composite,
		leftQuery:     leftQuery,
		queryOperator: op,
		rightQuery:    rightQuery,
	}, strings.TrimPrefix(remainder, ")"), nil
}

// CreateFilterQuery creates a parsed filter query from a filter string
func CreateFilterQuery(s string) (*FilterQuery, error) {
	fq, remainder, err := parseFilterQuery(s)
	if err != nil {
		return nil, ucerr.Wrap(err)
	}

	if remainder != "" {
		return nil, ucerr.Errorf("filter query '%s' has unparsed remainder '%s'", s, remainder)
	}

	return fq, nil
}
