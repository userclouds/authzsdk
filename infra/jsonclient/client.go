package jsonclient

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"

	"userclouds.com/infra/oidc"
	"userclouds.com/infra/ucerr"
	"userclouds.com/infra/ucjwt"
)

// Error defines a jsonclient error for non-2XX/3XX status codes
// TODO: decide how to handle 3XX results
type Error struct {
	StatusCode int
}

// Error implements Error
func (e Error) Error() string {
	return fmt.Sprintf("HTTP error %d", e.StatusCode)
}

type options struct {
	headers          http.Header
	cookies          []http.Cookie
	unmarshalOnError bool
	parseOAuthError  bool

	// Required for automatic token refresh
	tokenSource oidc.ClientCredentialsTokenSource
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

// Header allows you to add arbitrary headers to jsonclient requests
func Header(k, v string) Option {
	return optFunc(func(opts *options) {
		opts.headers.Set(k, v)
	})
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

// ClientCredentialsTokenSource can be specified to enable support for RefreshBearerToken automatically
// refreshing the token if expired.
func ClientCredentialsTokenSource(tokenURL, clientID, clientSecret string, customAudiences []string) Option {
	return optFunc(func(opts *options) {
		opts.tokenSource.TokenURL = tokenURL
		opts.tokenSource.ClientID = clientID
		opts.tokenSource.ClientSecret = clientSecret
		opts.tokenSource.CustomAudiences = customAudiences
	})
}

// Client defines a JSON-focused HTTP client
// TODO: someday this should use a custom http.Client etc
type Client struct {
	baseURL string

	// Keep options contained so they can be cloned & augmented per request.
	options      options
	optionsMutex sync.RWMutex
}

// New returns a new Client
func New(url string, opts ...Option) *Client {
	c := &Client{
		baseURL: url,
		options: options{
			headers: make(http.Header),
		},
	}

	for _, opt := range opts {
		opt.apply(&c.options)
	}

	return c
}

// Apply applies options to an existing client (useful for updating a header/cookie/etc on an existing client).
func (c *Client) Apply(opts ...Option) {
	c.optionsMutex.Lock()
	defer c.optionsMutex.Unlock()
	for _, opt := range opts {
		opt.apply(&c.options)
	}
}

// GetBearerToken returns the bearer token associated with this client, if one exists,
// or an error otherwise.
func (c *Client) GetBearerToken() (string, error) {
	c.optionsMutex.RLock()
	defer c.optionsMutex.RUnlock()
	token, err := ucjwt.ExtractBearerToken(&c.options.headers)
	return token, ucerr.Wrap(err)
}

func (c *Client) tokenNeedsRefresh() bool {
	needsRefresh := true
	currentToken, err := c.GetBearerToken()
	if err == nil {
		needsRefresh, err = ucjwt.IsExpired(currentToken)
		if err != nil {
			needsRefresh = true
		}
	}
	return needsRefresh
}

func (c *Client) hasTokenSource() bool {
	c.optionsMutex.RLock()
	defer c.optionsMutex.RUnlock()
	return c.options.tokenSource.TokenURL != ""
}

// ValidateBearerTokenHeader ensures that there is a non-expired bearer token specified directly OR
// that there's a valid token source to refresh it if not specified or expired.
func (c *Client) ValidateBearerTokenHeader() error {
	if c.tokenNeedsRefresh() && !c.hasTokenSource() {
		return ucerr.New("cannot refresh unspecified or expired bearer token without specifying valid ClientCredentialsTokenSource option for jsonclient")
	}
	return nil
}

// RefreshBearerToken checks if the current token is invalid or expired, and refreshes it via
// the Client Credentials Flow if needed.
func (c *Client) RefreshBearerToken() error {
	if c.tokenNeedsRefresh() {
		if !c.hasTokenSource() {
			return ucerr.New("cannot refresh bearer token without specifying valid ClientCredentialsTokenSource option for jsonclient")
		}

		c.optionsMutex.RLock()
		accessToken, err := c.options.tokenSource.GetToken()
		c.optionsMutex.RUnlock()
		if err != nil {
			return ucerr.Wrap(err)
		}

		c.Apply(Header("Authorization", fmt.Sprintf("Bearer %s", accessToken)))
	}
	return nil
}

// Get makes an HTTP get using this client
// TODO: need to support query params soon :)
func (c *Client) Get(ctx context.Context, path string, response interface{}, opts ...Option) error {
	return ucerr.Wrap(c.makeRequest(ctx, http.MethodGet, path, nil, response, opts))
}

// Post makes an HTTP post using this client
// If response is nil, the response isn't decoded and merely the success or failure is returned
func (c *Client) Post(ctx context.Context, path string, body, response interface{}, opts ...Option) error {
	bs, err := json.Marshal(body)
	if err != nil {
		return ucerr.Wrap(err)
	}

	return ucerr.Wrap(c.makeRequest(ctx, http.MethodPost, path, bs, response, opts))
}

// Put makes an HTTP put using this client
// If response is nil, the response isn't decoded and merely the success or failure is returned
func (c *Client) Put(ctx context.Context, path string, body, response interface{}, opts ...Option) error {
	bs, err := json.Marshal(body)
	if err != nil {
		return ucerr.Wrap(err)
	}

	return ucerr.Wrap(c.makeRequest(ctx, http.MethodPut, path, bs, response, opts))
}

// Patch makes an HTTP patch using this client
// If response is nil, the response isn't decoded and merely the success or failure is returned
func (c *Client) Patch(ctx context.Context, path string, body, response interface{}, opts ...Option) error {
	bs, err := json.Marshal(body)
	if err != nil {
		return ucerr.Wrap(err)
	}

	return ucerr.Wrap(c.makeRequest(ctx, http.MethodPatch, path, bs, response, opts))
}

// Delete makes an HTTP delete using this client
func (c *Client) Delete(ctx context.Context, path string, opts ...Option) error {
	return ucerr.Wrap(c.makeRequest(ctx, http.MethodDelete, path, nil, nil, opts))
}

func (c *Client) makeRequest(ctx context.Context, method, path string, bs []byte, response interface{}, opts []Option) error {
	// Always clone to minimize contention
	c.optionsMutex.RLock()
	options := c.options.clone()
	c.optionsMutex.RUnlock()

	// Concat per-request options
	for _, opt := range opts {
		opt.apply(options)
	}
	client := http.DefaultClient

	reqURL := c.buildURL(path)
	req, err := http.NewRequest(method, reqURL, bytes.NewReader(bs))
	if err != nil {
		return ucerr.Wrap(err)
	}

	req.Header = options.headers.Clone()
	req.Header.Add("content-type", "application/json")

	for _, cookie := range options.cookies {
		req.AddCookie(&cookie)
	}

	// https://github.com/golang/go/issues/29865
	// Host header is ignored by http.Request.Write, but for test purposes
	// it is very useful to override the Host header.
	if req.Header.Get("Host") != "" {
		req.Host = req.Header.Get("Host")
	}

	res, err := client.Do(req)
	if err != nil {
		return ucerr.Wrap(err)
	}
	defer res.Body.Close()

	// If the response was not an error OR if the caller specified UnmarshalOnError, try to deserialize
	// the response into the provided struct.
	if res.StatusCode < http.StatusBadRequest || options.unmarshalOnError {
		if response != nil {
			if err := json.NewDecoder(res.Body).Decode(response); err != nil {
				return ucerr.Wrap(err)
			}
		}
	} else {
		// An error was returned and the caller is not intentionally capturing the body; log full error response for debugging purposes.
		// TODO: hide behind a flag for perf / PII / etc reasons?
		b, err := io.ReadAll(res.Body)
		var bs string
		if err != nil {
			bs = "<unable to decode response>"
		} else {
			bs = string(b)
		}
		log.Printf("http %s request to URL '%s' returned error response (code %d): %s", method, reqURL, res.StatusCode, bs) // ucwrapper-safe - avoid uclog dependency since this is used by SDK

		if options.parseOAuthError {
			var oauthe ucerr.OAuthError
			// OAuth standard requires us to return a body with error descriptions
			// in many cases, so try to decode response but ignore the error if it fails.
			err = json.NewDecoder(bytes.NewReader(b)).Decode(&oauthe)
			if err == nil {
				// Ensure we use the actual code from the http response.
				oauthe.Code = res.StatusCode
				return ucerr.Wrap(oauthe)
			}
		}
	}

	// TODO: validate that 2xx is received, not 3xx or something else?

	if res.StatusCode >= http.StatusBadRequest {
		return ucerr.Wrap(Error{res.StatusCode})
	}

	return nil
}

// normalize trailing and leading slashes
func (c *Client) buildURL(path string) string {
	if path != "" {
		return fmt.Sprintf("%s/%s", strings.TrimSuffix(c.baseURL, "/"), strings.TrimPrefix(path, "/"))
	}
	return c.baseURL
}

// GetHTTPStatusCode returns the underlying HTTP status code or -1 if no code could be extracted.
func GetHTTPStatusCode(err error) int {
	var jce Error
	var oauthe ucerr.OAuthError
	if errors.As(err, &jce) {
		return jce.StatusCode
	} else if errors.As(err, &oauthe) {
		return oauthe.Code
	}
	return -1
}
