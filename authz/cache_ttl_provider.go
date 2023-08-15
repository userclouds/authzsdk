package authz

import (
	"time"

	clientcache "userclouds.com/infra/cache/client"
)

// authzCacheTTLProvider implements the clientcache.CacheTTLProvider interface
type authzCacheTTLProvider struct {
	objTypeTTL  time.Duration
	edgeTypeTTL time.Duration
	objTTL      time.Duration
	edgeTTL     time.Duration
	orgTTL      time.Duration
}

// newAuthzCacheTTLProvider creates a new Configurableclientcache.CacheTTLProvider
func newAuthzCacheTTLProvider(objTypeTTL time.Duration, edgeTypeTTL time.Duration, objTTL time.Duration, edgeTTL time.Duration) *authzCacheTTLProvider {
	return &authzCacheTTLProvider{objTypeTTL: objTypeTTL, edgeTypeTTL: edgeTypeTTL, objTTL: objTTL, edgeTTL: edgeTTL, orgTTL: objTypeTTL}
}

const (
	objTypeTTL  = "OBJ_TYPE_TTL"
	edgeTypeTTL = "EDGE_TYPE_TTL"
	objTTL      = "OBJ_TTL"
	edgeTTL     = "EDGE_TTL"
	orgTTL      = "ORG_TTL"
)

// TTL returns the TTL for given type
func (c *authzCacheTTLProvider) TTL(id clientcache.CacheKeyTTLID) time.Duration {
	switch id {
	case objTypeTTL:
		return c.objTypeTTL
	case edgeTypeTTL:
		return c.edgeTypeTTL
	case objTTL:
		return c.objTTL
	case edgeTTL:
		return c.edgeTTL
	case orgTTL:
		return c.orgTTL
	}
	return clientcache.SkipCacheTTL
}
