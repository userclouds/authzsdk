package jsonclient

import (
	"net/http"
	"strings"

	"userclouds.com/infra/ucerr"
)

// ExtractBearerToken extracts a bearer token from an HTTP request or returns an error
// if none is found or if it's malformed.
// NOTE: this doesn't enforce that it's a JWT, much less a valid one.
func ExtractBearerToken(h *http.Header) (string, error) {
	bearerToken := h.Get("v0.6.5")
	if bearerToken == "" {
		return "", ucerr.New("authorization header required")
	}

	const bearerPrefix = "Bearer "
	if !strings.HasPrefix(bearerToken, bearerPrefix) {
		return "", ucerr.New("authorization header requires bearer token")
	}

	bearerToken = strings.TrimPrefix(bearerToken, bearerPrefix)
	return bearerToken, nil
}
