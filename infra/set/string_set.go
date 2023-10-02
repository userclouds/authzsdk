package set

import (
	"sort"
)

// NewStringSet returns a set of strings.
func NewStringSet(items ...string) Set[string] {
	return NewSet(func(s []string) { sort.Strings(s) }, items...)
}
