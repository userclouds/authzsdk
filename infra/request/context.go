package request

import (
	"context"
	"net/http"

	"github.com/gofrs/uuid"
)

type contextKey int

const (
	ctxRequestData contextKey = 1
)

// GetRequestID returns a per request id if one was set
func GetRequestID(ctx context.Context) uuid.UUID {
	return getRequestData(ctx).requestID
}

type requestData struct {
	requestID  uuid.UUID
	hostname   string
	authHeader string
	userAgent  string
	method     string
	path       string
}

func getRequestData(ctx context.Context) requestData {
	val := ctx.Value(ctxRequestData)
	rd, ok := val.(*requestData)
	if !ok {
		return requestData{requestID: uuid.Nil}
	}
	return *rd

}

// SetRequestData sets capture a bunch of data from the request and saves into a struct in the context
func SetRequestData(ctx context.Context, req *http.Request, requestID uuid.UUID) context.Context {
	var rd *requestData
	if req == nil {
		currRD := getRequestData(ctx)
		if currRD.requestID.IsNil() {
			currRD.requestID = requestID
			rd = &currRD
		} else {
			return ctx
		}
	} else {
		rd = &requestData{
			requestID:  requestID,
			hostname:   req.Host,
			userAgent:  req.UserAgent(),
			authHeader: req.Header.Get("Authorization"),
			method:     req.Method,
			path:       req.URL.Path,
		}
	}
	return context.WithValue(ctx, ctxRequestData, rd)
}

// GetHostname returns the hostname used for this particular request
func GetHostname(ctx context.Context) string {
	return getRequestData(ctx).hostname
}

// GetAuthHeader returns the Authorization Header for this particular request
func GetAuthHeader(ctx context.Context) string {
	return getRequestData(ctx).authHeader
}

// GetUserAgent returns the User-Agent header for this particular request
func GetUserAgent(ctx context.Context) string {
	return getRequestData(ctx).userAgent
}

// GetRequestDataMap returns the a map of request data for a particular request, this is useful when we want to pass unstructured data to to other systems (sentry, tracing, etc...) and we don't have a reference to the request object
func GetRequestDataMap(ctx context.Context) map[string]string {
	rd := getRequestData(ctx)
	if rd.hostname == "" {
		return nil
	}
	return map[string]string{"method": rd.method, "path": rd.path, "hostname": rd.hostname, "requestID": rd.requestID.String()}
}
