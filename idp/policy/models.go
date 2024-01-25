package policy

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/gofrs/uuid"

	"userclouds.com/idp/userstore"
	"userclouds.com/infra/pagination"
	"userclouds.com/infra/ucdb"
	"userclouds.com/infra/ucerr"
	"userclouds.com/infra/uctypes/uuidarray"
)

var validIdentifier = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_-]*$`)

// TransformType describes the type of transform to be performed
type TransformType string

const (
	// TransformTypePassThrough is a no-op transformation
	TransformTypePassThrough TransformType = "passthrough"

	// TransformTypeTransform is a transformation that doesn't tokenize
	TransformTypeTransform TransformType = "transform"

	// TransformTypeTokenizeByValue is a transformation that tokenizes the value passed in
	TransformTypeTokenizeByValue TransformType = "tokenizebyvalue"

	// TransformTypeTokenizeByReference is a transformation that tokenizes the userstore reference to the value passed in
	TransformTypeTokenizeByReference TransformType = "tokenizebyreference"
)

//go:generate genconstant TransformType

// Transformer describes a token transformer
type Transformer struct {
	ID                 uuid.UUID           `json:"id"`
	Name               string              `json:"name" validate:"length:1,128" required:"true"`
	Description        string              `json:"description"`
	InputType          userstore.DataType  `json:"input_type" required:"true"`
	OutputType         userstore.DataType  `json:"output_type" validate:"skip"`
	ReuseExistingToken bool                `json:"reuse_existing_token" validate:"skip" description:"Specifies if the tokenizing transfomer should return existing token instead of creating a new one."`
	TransformType      TransformType       `json:"transform_type" required:"true"`
	TagIDs             uuidarray.UUIDArray `json:"tag_ids" validate:"skip"`
	Function           string              `json:"function" required:"true"`
	Parameters         string              `json:"parameters"`
	IsSystem           bool                `json:"is_system" description:"Whether this transformer is a system transformer. System transformers cannot be deleted or modified. This property cannot be changed."`
}

// GetPaginationKeys is part of the pagination.PageableType interface
func (Transformer) GetPaginationKeys() pagination.KeyTypes {
	return pagination.KeyTypes{
		"id":             pagination.UUIDKeyType,
		"name":           pagination.StringKeyType,
		"description":    pagination.StringKeyType,
		"input_type":     pagination.StringKeyType,
		"output_type":    pagination.StringKeyType,
		"transform_type": pagination.StringKeyType,
		"created":        pagination.TimestampKeyType,
		"updated":        pagination.TimestampKeyType,
	}
}

//go:generate genvalidate Transformer

// IsPolicyRequiredForExecution checks the transformation type and returns if an access policy is required to execute the transformer
func (g Transformer) IsPolicyRequiredForExecution() bool {
	return g.TransformType == TransformTypeTokenizeByValue || g.TransformType == TransformTypeTokenizeByReference
}

func (g Transformer) extraValidate() error {

	if !validIdentifier.MatchString(string(g.Name)) {
		return ucerr.Friendlyf(nil, `Transformer name "%s" has invalid characters`, g.Name)
	}

	params := map[string]interface{}{}
	if err := json.Unmarshal([]byte(g.Parameters), &params); g.Parameters != "" && err != nil {
		paramsArr := []interface{}{}
		if err := json.Unmarshal([]byte(g.Parameters), &paramsArr); err != nil {
			return ucerr.New("Transformer.Parameters must be either empty, or a JSON dictionary or JSON array")
		}
	}

	if err := validateJSHelper(g.Function, fmt.Sprintf("%v.js", g.ID)); err != nil {
		return ucerr.Friendlyf(err, "Transformer validation - Javascript error")
	}

	if g.OutputType != userstore.DataTypeInvalid {
		if err := g.OutputType.Validate(); err != nil {
			return ucerr.Wrap(err)
		}
	}

	if g.ReuseExistingToken && g.TransformType != TransformTypeTokenizeByValue && g.TransformType != TransformTypeTokenizeByReference {
		return ucerr.Friendlyf(nil, "ReuseExistingToken can only be true for tokenization transformers")
	}

	return nil
}

// Equals returns true if the two policies are equal, ignoring the ID and description fields
func (g *Transformer) Equals(other *Transformer) bool {
	return (g.ID == other.ID || g.ID.IsNil() || other.ID.IsNil()) &&
		strings.EqualFold(g.Name, other.Name) &&
		g.InputType == other.InputType &&
		g.OutputType == other.OutputType &&
		g.TransformType == other.TransformType &&
		g.ReuseExistingToken == other.ReuseExistingToken &&
		g.Function == other.Function &&
		g.Parameters == other.Parameters &&
		g.IsSystem == other.IsSystem
}

// UserstoreDataProvenance is used by TransformTypeTokenizeByReference to describe the provenance of the data
type UserstoreDataProvenance struct {
	UserID   uuid.UUID `json:"user_id" validate:"notnil"`
	ColumnID uuid.UUID `json:"column_id" validate:"notnil"`
}

//go:generate genvalidate UserstoreDataProvenance

// PolicyType describes the type of an access policy
type PolicyType string //revive:disable-line:exported

const (
	// PolicyTypeInvalid is an invalid policy type
	PolicyTypeInvalid PolicyType = "invalid"

	// PolicyTypeCompositeIntersection is the type for composite policies in which all components must be satisfied to grant access
	PolicyTypeCompositeIntersection = "compositeintersection"

	// PolicyTypeCompositeUnion is the type for composite policies in which any component must be satisfied to grant access
	PolicyTypeCompositeUnion = "compositeunion"
)

//go:generate genconstant PolicyType

// AccessPolicyTemplate describes a template for an access policy
type AccessPolicyTemplate struct {
	ucdb.SystemAttributeBaseModel `validate:"skip"`
	Name                          string `db:"name" json:"name" validate:"length:1,128" required:"true"`
	Description                   string `db:"description" json:"description"`
	Function                      string `db:"function" json:"function" required:"true"`
	Version                       int    `db:"version" json:"version"`
}

// GetPaginationKeys is part of the pagination.PageableType interface
func (AccessPolicyTemplate) GetPaginationKeys() pagination.KeyTypes {
	return pagination.KeyTypes{
		"id":          pagination.UUIDKeyType,
		"name":        pagination.StringKeyType,
		"description": pagination.StringKeyType,
		"created":     pagination.TimestampKeyType,
		"updated":     pagination.TimestampKeyType,
	}
}

//go:generate genvalidate AccessPolicyTemplate

func (a AccessPolicyTemplate) extraValidate() error {
	if !validIdentifier.MatchString(string(a.Name)) {
		return ucerr.Friendlyf(nil, `Access policy template name "%s" has invalid characters`, a.Name)
	}
	if err := validateJSHelper(a.Function, fmt.Sprintf("%v.js", a.ID)); err != nil {
		return ucerr.Friendlyf(err, "Access policy template validation - Javascript error")
	}

	return nil
}

// Equals returns true if the two templates are equal, ignoring the ID, description, and version fields
func (a *AccessPolicyTemplate) Equals(other *AccessPolicyTemplate) bool {
	return (a.ID == other.ID || a.ID.IsNil() || other.ID.IsNil()) &&
		strings.EqualFold(a.Name, other.Name) &&
		a.Function == other.Function &&
		a.IsSystem == other.IsSystem
}

// AccessPolicyComponent is either an access policy a template paired with parameters to fill it with
type AccessPolicyComponent struct {
	Policy             *userstore.ResourceID `json:"policy,omitempty"`
	Template           *userstore.ResourceID `json:"template,omitempty"`
	TemplateParameters string                `json:"template_parameters,omitempty"`
}

// Validate implements Validateable
func (a AccessPolicyComponent) Validate() error {
	policyValidErr := a.Policy.Validate()
	templateValidErr := a.Template.Validate()
	if (policyValidErr != nil && templateValidErr != nil) || (policyValidErr == nil && templateValidErr == nil) {
		return ucerr.New("AccessPolicyComponent must have either a Policy or a Template specified, but not both")
	}

	if templateValidErr == nil {
		params := map[string]interface{}{}
		if err := json.Unmarshal([]byte(a.TemplateParameters), &params); a.TemplateParameters != "" && err != nil {
			return ucerr.New("AccessPolicyComponent.Parameters must be either empty, or a JSON dictionary")
		}
	} else if a.TemplateParameters != "" {
		return ucerr.New("AccessPolicyComponent.Parameters must be empty when a Policy is specified")
	}

	return nil
}

// AccessPolicy describes an access policy
type AccessPolicy struct {
	ID          uuid.UUID           `json:"id" validate:"skip"`
	Name        string              `json:"name" validate:"length:1,128" required:"true"`
	Description string              `json:"description"`
	PolicyType  PolicyType          `json:"policy_type" required:"true"`
	TagIDs      uuidarray.UUIDArray `json:"tag_ids" validate:"skip"`
	Version     int                 `json:"version"`
	IsSystem    bool                `json:"is_system" description:"Whether this policy is a system policy. System policies cannot be deleted or modified. This property cannot be changed."`

	Components []AccessPolicyComponent `json:"components" validate:"skip"`
}

// GetPaginationKeys is part of the pagination.PageableType interface
func (AccessPolicy) GetPaginationKeys() pagination.KeyTypes {
	return pagination.KeyTypes{
		"id":          pagination.UUIDKeyType,
		"name":        pagination.StringKeyType,
		"description": pagination.StringKeyType,
		"policy_type": pagination.StringKeyType,
		"created":     pagination.TimestampKeyType,
		"updated":     pagination.TimestampKeyType,
	}
}

//go:generate genvalidate AccessPolicy

func (a AccessPolicy) extraValidate() error {

	if !validIdentifier.MatchString(string(a.Name)) {
		return ucerr.Friendlyf(nil, `Access policy name "%s" has invalid characters`, a.Name)
	}

	return nil
}

// ClientContext is passed by the client at resolution time
type ClientContext map[string]interface{}

// AccessPolicyContext gets passed to the access policy's function(context, params) at resolution time
type AccessPolicyContext struct {
	Server ServerContext    `json:"server"`
	Client ClientContext    `json:"client"`
	User   userstore.Record `json:"user"`
}

//go:generate genvalidate AccessPolicyContext

// ServerContext is automatically injected by the server at resolution time
type ServerContext struct {
	// TODO: add token creation time
	IPAddress string          `json:"ip_address"`
	Resolver  ResolverContext `json:"resolver"`
	Action    Action          `json:"action"`
}

//go:generate genvalidate ServerContext

// ResolverContext contains automatic data about the authenticated user/system at resolution time
type ResolverContext struct {
	Username string `json:"username"`
}

//go:generate genvalidate ResolverContext

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
