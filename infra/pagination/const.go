package pagination

import "github.com/gofrs/uuid"

// DefaultLimit is the default pagination limit (number of results to retrieve).
const DefaultLimit = 50

// MaxLimit is (at the moment) fairly arbitrarily chosen and limits results in a single call.
const MaxLimit = 100

// StartingID is a value that is used to indicate the lowest ID cursor value.
var StartingID = uuid.Nil
