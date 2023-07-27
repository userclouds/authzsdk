package request

import (
	"context"

	"github.com/gofrs/uuid"
)

type contextKey int

const (
	ctxRequestID  contextKey = 1 // key for RequestID uuid
	ctxHost       contextKey = 2 // key to save the request's hostname
	ctxAuthHeader contextKey = 3 // key for the request's header
)

// GetRequestID returns a per request id if one was set
func GetRequestID(ctx context.Context) uuid.UUID {
	val := ctx.Value(ctxRequestID)
	id, ok := val.(uuid.UUID)
	if !ok {
		return uuid.Nil
	}
	return id
}

// SetRequestID set the Request ID for this request
func SetRequestID(ctx context.Context, requestID uuid.UUID) context.Context {
	return context.WithValue(ctx, ctxRequestID, requestID)
}

// GetHostname returns the hostname used for this particular request
func GetHostname(ctx context.Context) string {
	val := ctx.Value(ctxHost)
	host, _ := val.(string)
	return host
}

// GetAuthHeader returns the Authorization Header for this particular request
func GetAuthHeader(ctx context.Context) string {
	val := ctx.Value(ctxAuthHeader)
	header, _ := val.(string)
	return header
}
