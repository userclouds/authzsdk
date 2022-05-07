package jsonclient

import "context"

func logError(ctx context.Context, method, url, errorMsg string, code int) {
	// don't log by default in SDK
}
