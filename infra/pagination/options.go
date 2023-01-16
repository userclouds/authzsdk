package pagination

// Option defines a method of passing optional args to paginated List APIs
type Option interface {
	apply(*Paginator)
}

type optFunc func(*Paginator)

func (of optFunc) apply(p *Paginator) {
	of(p)
	p.options = append(p.options, of)
}

// EndingBefore iterates the collection's values before (but not including) the value at the cursor.
// It is commonly used to implement "Prev" functionality.
func EndingBefore(cursor Cursor) Option {
	return optFunc(
		func(p *Paginator) {
			p.cursor = cursor
			p.direction = DirectionBackward
			p.backwardDirectionSet = true
		})
}

// Limit specifies how many results to fetch at once. If unspecified, the default limit will be used.
func Limit(limit int) Option {
	return optFunc(
		func(p *Paginator) {
			p.limit = limit
		})
}

func requestVersion(version Version) Option {
	return optFunc(
		func(p *Paginator) {
			p.version = version
		})
}

// SortKey optionally specifies which field of the collection should be used to sort results in the view.
func SortKey(key Key) Option {
	return optFunc(
		func(p *Paginator) {
			p.sortKey = key
		})
}

// SortOrder optionally specifies which way a view on a collection should be sorted.
func SortOrder(order Order) Option {
	return optFunc(
		func(p *Paginator) {
			p.sortOrder = order
		})
}

// StartingAfter iterates the collection starting after (but not including) the value at the cursor.
// It can be used to fetch the "current" page starting at a cursor, as well as to iterate the next page
// by passing in the cursor of the last item on the page.
func StartingAfter(cursor Cursor) Option {
	return optFunc(
		func(p *Paginator) {
			p.cursor = cursor
			p.direction = DirectionForward
			p.forwardDirectionSet = true
		})
}
