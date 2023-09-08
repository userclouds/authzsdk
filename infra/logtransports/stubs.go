package logtransports

import (
	"userclouds.com/infra/uclog"
)

// Close is only a wrapper around uclog.Close in the SDK
func Close() {
	uclog.Close()
}
