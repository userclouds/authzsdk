package jsonclient

import (
	"context"
	"io"
	"net/http"

	"userclouds.com/infra/oidc"
)

// HeaderFunc is a callback that's invoked on every request to generate a header
// This is useful when the header should change per-request based on the context
// used for that request, eg. a long-lived client that makes requests on behalf
// of numerous customers with different X-Forwarded-For IPs, etc
// Note that returning a blank key indicates "no header to add this request"
type HeaderFunc func(context.Context) (key string, value string)

// DecodeFunc is a callback used with the CustomDecoder option to control deserializing
// the response from an HTTP request. Instead of automatically deserializing into the
// response object provided to the method (which must be nil instead), this method is invoked.
type DecodeFunc func(ctx context.Context, body io.ReadCloser) error

type options struct {
	headers          http.Header
	cookies          []http.Cookie
	unmarshalOnError bool
	parseOAuthError  bool
	stopLogging      bool

	// Required for automatic token refresh
	tokenSource oidc.TokenSource

	// allows runtime updating of headers eg. to pass along X-Forwarded-For on a per-request basis
	perRequestHeaders []HeaderFunc

	decodeFunc DecodeFunc

	// retryNetworkErrors causes the client to retry requests that fail due to network errors,
	// up to `maxRetries`, with a `backoff` pause each time
	retryNetworkErrors bool

	bypassRouting bool // bypass localhost routing for cross-service calls
}

func (o *options) clone() *options {
	cloned := *o
	cloned.headers = o.headers.Clone()
	copy(cloned.cookies, o.cookies)
	return &cloned
}

// Option makes jsonclient extensible
type Option interface {
	apply(*options)
}

type optFunc func(*options)

func (o optFunc) apply(opts *options) {
	o(opts)
}

// BypassRouting allows you to bypass our internal request rerouting system to test performance
func BypassRouting() Option {
	return optFunc(func(opts *options) {
		opts.bypassRouting = true
	})
}

// Header allows you to add arbitrary headers to jsonclient requests
func Header(k, v string) Option {
	return optFunc(func(opts *options) {
		opts.headers.Set(k, v)
	})
}

// PassthroughAuthorization allows you to pass the Authorization header through from
// an incoming request to an outgoing request. Note this will only work as an option
// in a request context (not a long-lived client)
// TODO: should we have a way to differentiate short- and long-lived options?
func PassthroughAuthorization(r *http.Request) Option {
	return Header("Authorization", r.Header.Get("Authorization"))
}

// PassthroughAuthorizationString does the same as PassthroughAuthorization but takes the Authorization string
func PassthroughAuthorizationString(authHeader string) Option {
	return Header("Authorization", authHeader)
}

// Cookie allows you to add cookies to jsonclient requests
func Cookie(cookie http.Cookie) Option {
	return optFunc(func(opts *options) {
		opts.cookies = append(opts.cookies, cookie)
	})
}

// UnmarshalOnError causes the response struct to be deserialized if a HTTP 400+ code is returned.
// The default behavior is to not deserialize and to return an error.
func UnmarshalOnError() Option {
	return optFunc(func(opts *options) {
		opts.unmarshalOnError = true
	})
}

// ParseOAuthError allows deserializing & capturing the last call's error
// into an OAuthError object for deeper inspection. This is richer than a jsonclient.Error
// but only makes sense on a call that is expected to be OAuth/OIDC compliant.
func ParseOAuthError() Option {
	return optFunc(func(opts *options) {
		opts.parseOAuthError = true
	})
}

// StopLogging causes the client not to log failures
func StopLogging() Option {
	return optFunc(func(opts *options) {
		opts.stopLogging = true
	})
}

// PerRequestHeader allows you to pass in a callback that takes a context and returns a header k,v
// that will be called on each request and a new header appended to the request
func PerRequestHeader(f HeaderFunc) Option {
	return optFunc(func(opts *options) {
		opts.perRequestHeaders = append(opts.perRequestHeaders, f)
	})
}

// ClientCredentialsTokenSource can be specified to enable support for RefreshBearerToken automatically
// refreshing the token if expired.
// TODO: deprecate this in favor of the more generic TokenSource option
func ClientCredentialsTokenSource(tokenURL, clientID, clientSecret string, customAudiences []string) Option {
	return TokenSource(oidc.ClientCredentialsTokenSource{
		TokenURL:        tokenURL,
		ClientID:        clientID,
		ClientSecret:    clientSecret,
		CustomAudiences: customAudiences,
	})
}

// TokenSource takes an arbitrary token source
func TokenSource(ts oidc.TokenSource) Option {
	return optFunc(func(opts *options) {
		opts.tokenSource = ts
	})
}

// CustomDecoder allows the caller to control deserializing the HTTP response.
// It is most useful when the exact structure of the response is not known ahead of time,
// and custom logic is required (e.g. for API compatibility).
func CustomDecoder(f DecodeFunc) Option {
	return optFunc(func(opts *options) {
		opts.decodeFunc = f
	})
}

// RetryNetworkErrors causes the client to retry on underlying network errors
// TODO: is this a good idea?
// TODO: should we have a max retry count, backoff, etc config?
func RetryNetworkErrors() Option {
	return optFunc(func(opts *options) {
		opts.retryNetworkErrors = true
	})
}
