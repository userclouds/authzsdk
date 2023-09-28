package set

import (
	"sort"
)

// NewIntSet returns a set of ints.
func NewIntSet(items ...int) Set[int] {
	return NewSet(func(i []int) { sort.Ints(i) }, items...)
}
