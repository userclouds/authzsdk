package policy

import (
	"encoding/json"

	"github.com/gofrs/uuid"

	"userclouds.com/infra/ucerr"
)

// GenerationPolicy describes a token generation policy
type GenerationPolicy struct {
	ID         uuid.UUID `json:"id"`
	Name       string    `json:"name"`
	Function   string    `json:"function"`
	Parameters string    `json:"parameters"`
}

// Validate implements Validateable
func (g GenerationPolicy) Validate() error {
	// either ID or Function must be set, but not both
	if (g.ID == uuid.Nil) == (g.Function == "") {
		return ucerr.New("Exactly one of GenerationPolicy.ID and GenerationPolicy.Function must be set")
	}

	params := map[string]interface{}{}
	if err := json.Unmarshal([]byte(g.Parameters), &params); g.Parameters != "" && err != nil {
		paramsArr := []interface{}{}
		if err := json.Unmarshal([]byte(g.Parameters), &paramsArr); err != nil {
			return ucerr.New("GenerationPolicy.Parameters must be either empty, or a JSON dictionary or JSON array")
		}
	}

	return nil
}

// AccessPolicy describes a token generation policy
type AccessPolicy struct {
	ID         uuid.UUID `json:"id"`
	Name       string    `json:"name"`
	Function   string    `json:"function"`
	Parameters string    `json:"parameters"`
	Version    int       `json:"version"` // NB: this is currently emitted by the server, but not read by the server (for UI only)
}

// Validate implements Validateable
func (g AccessPolicy) Validate() error {
	// either ID or Function must be set, but not both
	if (g.ID == uuid.Nil) == (g.Function == "") {
		return ucerr.New("Exactly one of AccessPolicy.ID and AccessPolicy.Function must be set")
	}

	params := map[string]interface{}{}
	if err := json.Unmarshal([]byte(g.Parameters), &params); g.Parameters != "" && err != nil {
		return ucerr.New("AccessPolicy.Parameters must be either empty, or a JSON dictionary")
	}

	return nil
}

// ClientContext is passed by the client at resolution time
type ClientContext map[string]interface{}

// AccessPolicyContext gets passed to the access policy's function(context, params) at resolution time
type AccessPolicyContext struct {
	Server ServerContext `json:"server"`
	Client ClientContext `json:"client"`
}

// ServerContext is automatically injected by the server at resolution time
type ServerContext struct {
	// TODO: add token creation time
	IPAddress string          `json:"ip_address"`
	Resolver  ResolverContext `json:"resolver"`
	Action    Action          `json:"action"`
}

// ResolverContext contains automatic data about the authenticated user/system at resolution time
type ResolverContext struct {
	Username string `json:"username"`
}

// Action identifies the reason access policy is being invoked
type Action string

// Different reasons for running access policy
const (
	ActionResolve Action = "Resolve"
	ActionInspect Action = "Inspect"
	ActionLookup  Action = "Lookup"
	ActionDelete  Action = "Delete"
	ActionExecute Action = "Execute" // TODO: should this be a unique action?
)
