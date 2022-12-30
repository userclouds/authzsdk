package pagination

import "github.com/gofrs/uuid"

// DefaultLimit is the default pagination limit (number of results to retrieve). This is what we use when the client
// doesn't specify a limit TODO - we don't respect this across all handlers (missing in tokenizer, etc)
// TODO this should be 50 as soon as UI supports pagination
const DefaultLimit = 500

// DefaultDBReadBatch is the default pagination batch size (number of results to retrieve) for calls between UC service and DB.
// TODO optimize this to have the right best perf for roundtip time and
const DefaultDBReadBatch = 1000

// MaxLimit is (at the moment) fairly arbitrarily chosen and limits results in a single call. This protects the server/DB from trying to
// process too much data at once
// TODO this should be higher but I tested this to work fine on a number of browsers until we add UI pagination
const MaxLimit = 1500

// StartingID is a value that is used to indicate the lowest ID cursor value.
var StartingID = uuid.Nil
