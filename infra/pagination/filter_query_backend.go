package pagination

import (
	"fmt"

	"userclouds.com/infra/ucerr"
)

func (fq *FilterQuery) queryFields(supportedKeys KeyTypes, queryFields []interface{}) ([]interface{}, error) {
	switch fq.nodeType {
	case nested:
		return fq.leftQuery.queryFields(supportedKeys, queryFields)
	case leaf:
		if fq.queryOperator.isComparisonOperator() {
			value, err := supportedKeys.getValidExactValue(fq.key, fq.value)
			if err != nil {
				return nil, ucerr.Wrap(err)
			}
			queryFields = append(queryFields, value)
			return queryFields, nil
		} else if fq.queryOperator.isPatternOperator() {
			value, err := supportedKeys.getValidNonExactValue(fq.key, fq.value)
			if err != nil {
				return nil, ucerr.Wrap(err)
			}
			queryFields = append(queryFields, value)
			return queryFields, nil
		} else {
			return nil, ucerr.Errorf("unsupported filter query leaf operator '%v'", fq.queryOperator)
		}
	case composite:
		queryFields, err := fq.leftQuery.queryFields(supportedKeys, queryFields)
		if err != nil {
			return nil, ucerr.Wrap(err)
		}
		return fq.rightQuery.queryFields(supportedKeys, queryFields)
	default:
		return nil, ucerr.Errorf("unsupported filter query node type '%v'", fq.nodeType)
	}
}

func (fq *FilterQuery) queryString(paramIndex int) (string, int) {
	switch fq.nodeType {
	case nested:
		s, paramIndex := fq.leftQuery.queryString(paramIndex)
		return fmt.Sprintf("(%s)", s), paramIndex
	case leaf:
		s := fmt.Sprintf("(%s %s $%d)", fq.key, fq.queryOperator.queryString(), paramIndex)
		return s, paramIndex + 1
	case composite:
		left, paramIndex := fq.leftQuery.queryString(paramIndex)
		right, paramIndex := fq.rightQuery.queryString(paramIndex)
		return fmt.Sprintf("(%s %s %s)", left, fq.queryOperator.queryString(), right), paramIndex
	}

	return "", paramIndex
}

// IsValid validates the parsed filter query using the specified KeyTypes
func (fq *FilterQuery) IsValid(supportedKeys KeyTypes) error {
	switch fq.nodeType {
	case nested:
		if fq.leftQuery == nil {
			return ucerr.New("nested filter query is missing leftQuery")
		}

		if err := fq.leftQuery.IsValid(supportedKeys); err != nil {
			return ucerr.Wrap(err)
		}
	case leaf:
		if fq.queryOperator.isComparisonOperator() {
			if err := supportedKeys.isValidExactValue(fq.key, fq.value); err != nil {
				return ucerr.Errorf("leaf query key '%s' and value '%s' are invalid: '%v'", fq.key, fq.value, err)
			}
		} else if fq.queryOperator.isPatternOperator() {
			if err := supportedKeys.isValidNonExactValue(fq.key, fq.value); err != nil {
				return ucerr.Errorf("leaf query key '%s' and value '%s' are invalid: '%v'", fq.key, fq.value, err)
			}
		} else {
			return ucerr.Errorf("leaf query has unsupported operator '%v'", fq.queryOperator)
		}
	case composite:
		if !fq.queryOperator.isLogicalOperator() {
			return ucerr.Errorf("composite query does not have a logical operator '%v'", fq.queryOperator)
		}

		if fq.leftQuery == nil {
			return ucerr.New("composite query does not have a leftQuery")
		}

		if fq.rightQuery == nil {
			return ucerr.New("composite query does not have a rightQuery")
		}

		if err := fq.leftQuery.IsValid(supportedKeys); err != nil {
			return ucerr.Wrap(err)
		}

		if err := fq.rightQuery.IsValid(supportedKeys); err != nil {
			return ucerr.Wrap(err)
		}
	}

	return nil
}
