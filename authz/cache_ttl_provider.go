package authz

import (
	"time"

	clientcache "userclouds.com/infra/cache/client"
)

// CacheTTLProvider implements the clientcache.CacheTTLProvider interface
type CacheTTLProvider struct {
	objTypeTTL  time.Duration
	edgeTypeTTL time.Duration
	objTTL      time.Duration
	edgeTTL     time.Duration
	orgTTL      time.Duration
}

// NewCacheTTLProvider creates a new Configurableclientcache.CacheTTLProvider
func NewCacheTTLProvider(objTypeTTL time.Duration, edgeTypeTTL time.Duration, objTTL time.Duration, edgeTTL time.Duration) *CacheTTLProvider {
	return &CacheTTLProvider{objTypeTTL: objTypeTTL, edgeTypeTTL: edgeTypeTTL, objTTL: objTTL, edgeTTL: edgeTTL, orgTTL: objTypeTTL}
}

const (
	// ObjectTypeTTL is the TTL for object types
	ObjectTypeTTL = "OBJ_TYPE_TTL"
	// EdgeTypeTTL is the TTL for edge types
	EdgeTypeTTL = "EDGE_TYPE_TTL"
	// ObjectTTL is the TTL for objects
	ObjectTTL = "OBJ_TTL"
	// EdgeTTL is the TTL for edges
	EdgeTTL = "EDGE_TTL"
	// OrganizationTTL is the TTL for organizations
	OrganizationTTL = "ORG_TTL"
)

// TTL returns the TTL for given type
func (c *CacheTTLProvider) TTL(id clientcache.CacheKeyTTLID) time.Duration {
	switch id {
	case ObjectTypeTTL:
		return c.objTypeTTL
	case EdgeTypeTTL:
		return c.edgeTypeTTL
	case ObjectTTL:
		return c.objTTL
	case EdgeTTL:
		return c.edgeTTL
	case OrganizationTTL:
		return c.orgTTL
	}
	return clientcache.SkipCacheTTL
}
