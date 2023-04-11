package idp

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/gofrs/uuid"

	"userclouds.com/idp/paths"
	"userclouds.com/idp/userstore"
	"userclouds.com/infra/jsonclient"
	"userclouds.com/infra/pagination"
	"userclouds.com/infra/ucerr"
	"userclouds.com/policy"
)

type options struct {
	ifNotExists       bool
	includeAuthN      bool
	organizationID    uuid.UUID
	paginationOptions []pagination.Option
	jsonclientOptions []jsonclient.Option
}

// Option makes idp.Client extensible
type Option interface {
	apply(*options)
}

type optFunc func(*options)

func (o optFunc) apply(opts *options) {
	o(opts)
}

// IfNotExists returns an Option that will cause the client not to return an error if an identical object to the one being created already exists
func IfNotExists() Option {
	return optFunc(func(opts *options) {
		opts.ifNotExists = true
	})
}

// IncludeAuthN returns a ManagementOption that will have the called method include AuthN fields
func IncludeAuthN() Option {
	return optFunc(func(opts *options) {
		opts.includeAuthN = true
	})
}

// OrganizationID returns an Option that will cause the client to use the specified organization ID for the request
func OrganizationID(organizationID uuid.UUID) Option {
	return optFunc(func(opts *options) {
		opts.organizationID = organizationID
	})
}

// Pagination is a wrapper around pagination.Option
func Pagination(opt ...pagination.Option) Option {
	return optFunc(func(opts *options) {
		opts.paginationOptions = append(opts.paginationOptions, opt...)
	})
}

// JSONClient is a wrapper around jsonclient.Option
func JSONClient(opt ...jsonclient.Option) Option {
	return optFunc(func(opts *options) {
		opts.jsonclientOptions = append(opts.jsonclientOptions, opt...)
	})
}

// Client represents a client to talk to the Userclouds IDP
type Client struct {
	client  *jsonclient.Client
	options options
}

// NewClient constructs a new IDP client
func NewClient(url string, opts ...Option) (*Client, error) {

	var options options
	for _, opt := range opts {
		opt.apply(&options)
	}

	c := &Client{
		client:  jsonclient.New(strings.TrimSuffix(url, "/"), options.jsonclientOptions...),
		options: options,
	}
	if err := c.client.ValidateBearerTokenHeader(); err != nil {
		return nil, ucerr.Wrap(err)
	}
	return c, nil
}

// CreateUserAndAuthnRequest creates a user on the IDP
type CreateUserAndAuthnRequest struct {
	Profile userstore.Record `json:"profile"`

	RequireMFA bool `json:"require_mfa"`

	OrganizationID uuid.UUID `json:"organization_id"`

	UserAuthn
}

// UserAndAuthnResponse is the response body for methods which return user data.
type UserAndAuthnResponse struct {
	ID        uuid.UUID `json:"id"`
	UpdatedAt int64     `json:"updated_at"` // seconds since the Unix Epoch (UTC)

	RequireMFA bool `json:"require_mfa"`

	Profile userstore.Record `json:"profile"`

	OrganizationID uuid.UUID `json:"organization_id"`

	Authns []UserAuthn `json:"authns"`
}

// CreateUser creates a user without authn. Profile is optional (okay to pass nil)
func (c *Client) CreateUser(ctx context.Context, profile userstore.Record, opts ...Option) (uuid.UUID, error) {
	// TODO: we don't validate the profile here, since we don't require email in this path
	// this probably should be refactored to be more consistent in this client

	options := c.options
	for _, opt := range opts {
		opt.apply(&options)
	}

	req := CreateUserAndAuthnRequest{
		Profile: profile,
	}
	if options.organizationID != uuid.Nil {
		req.OrganizationID = options.organizationID
	}

	var res UserAndAuthnResponse
	if err := c.client.Post(ctx, paths.CreateUser, req, &res); err != nil {
		return uuid.Nil, ucerr.Wrap(err)
	}

	return res.ID, nil
}

// GetUser gets a user by ID
func (c *Client) GetUser(ctx context.Context, id uuid.UUID, opts ...Option) (*UserAndAuthnResponse, error) {

	requestURL := url.URL{
		Path: fmt.Sprintf("/authn/users/%s", id),
	}

	options := c.options
	for _, opt := range opts {
		opt.apply(&options)
	}

	if options.includeAuthN {
		requestURL.RawQuery = url.Values{
			"include_authn": []string{"true"},
		}.Encode()
	}

	var res UserAndAuthnResponse
	if err := c.client.Get(ctx, requestURL.String(), &res); err != nil {
		return nil, ucerr.Wrap(err)
	}

	return &res, nil
}

// UpdateUserRequest optionally updates some or all mutable fields of a user struct.
// Pointers are used to distinguish between unset vs. set to default value (false, "", etc).
// TODO: should we allow changing Email? That's a more complex one as there are more implications to
// changing email that may affect AuthNs and security (e.g. account hijacking, unverified emails, etc).
type UpdateUserRequest struct {
	// TODO: add MFA factors
	RequireMFA *bool `json:"require_mfa,omitempty"`

	// Only fields set in the underlying map will be updated
	Profile userstore.Record `json:"profile"`
}

// UpdateUser updates user profile data for a given user ID
func (c *Client) UpdateUser(ctx context.Context, id uuid.UUID, req UpdateUserRequest) (*UserAndAuthnResponse, error) {
	requestURL := url.URL{
		Path: fmt.Sprintf("/authn/users/%s", id),
	}

	var resp UserAndAuthnResponse

	if err := c.client.Put(ctx, requestURL.String(), &req, &resp); err != nil {
		return nil, ucerr.Wrap(err)
	}

	return &resp, nil
}

// DeleteUser deletes a user by ID
func (c *Client) DeleteUser(ctx context.Context, id uuid.UUID) error {
	requestURL := url.URL{
		Path: fmt.Sprintf("/authn/users/%s", id),
	}

	return ucerr.Wrap(c.client.Delete(ctx, requestURL.String(), nil))
}

// CreateColumnRequest is the request body for creating a new column
// TODO: should this support multiple at once before we ship this API?
type CreateColumnRequest struct {
	Column userstore.Column `json:"column"`
}

//go:generate genvalidate CreateColumnRequest

// CreateColumnResponse is the response body for creating a new column
type CreateColumnResponse struct {
	Column userstore.Column `json:"column"`
}

// CreateColumn creates a new column for the associated tenant
func (c *Client) CreateColumn(ctx context.Context, column userstore.Column, opts ...Option) (*userstore.Column, error) {

	options := c.options
	for _, opt := range opts {
		opt.apply(&options)
	}

	req := CreateColumnRequest{
		Column: column,
	}

	var resp CreateColumnResponse
	if options.ifNotExists {
		exists, existingID, err := c.client.CreateIfNotExists(ctx, paths.CreateColumnPath, req, &resp)
		if err != nil {
			return nil, ucerr.Wrap(err)
		}
		if exists {
			resp.Column = req.Column
			resp.Column.ID = existingID
		}
	} else {
		if err := c.client.Post(ctx, paths.CreateColumnPath, req, &resp); err != nil {
			return nil, ucerr.Wrap(err)
		}
	}

	return &resp.Column, nil
}

// DeleteColumn deletes the column specified by the column ID for the associated tenant
func (c *Client) DeleteColumn(ctx context.Context, columnID uuid.UUID) error {
	return ucerr.Wrap(c.client.Delete(ctx, paths.DeleteColumnPath(columnID), nil))
}

// GetColumn returns the column specified by the column ID for the associated tenant
func (c *Client) GetColumn(ctx context.Context, columnID uuid.UUID) (*userstore.Column, error) {
	var resp userstore.Column
	if err := c.client.Get(ctx, paths.GetColumnPath(columnID), &resp); err != nil {
		return nil, ucerr.Wrap(err)
	}

	return &resp, nil
}

// ListColumnsResponse is the response body for listing columns
type ListColumnsResponse struct {
	Columns []userstore.Column `json:"columns"`
}

// ListColumns lists all columns for the associated tenant
func (c *Client) ListColumns(ctx context.Context) ([]userstore.Column, error) {
	var res ListColumnsResponse
	if err := c.client.Get(ctx, paths.ListColumnsPath, &res); err != nil {
		return nil, ucerr.Wrap(err)
	}

	return res.Columns, nil
}

// UpdateColumnRequest is the request body for updating a column
type UpdateColumnRequest struct {
	Column userstore.Column `json:"column"`
}

//go:generate genvalidate UpdateColumnRequest

// UpdateColumnResponse is the response body for updating a column
type UpdateColumnResponse struct {
	Column userstore.Column `json:"column"`
}

// UpdateColumn updates the column specified by the column ID with the specified data for the associated tenant
func (c *Client) UpdateColumn(ctx context.Context, columnID uuid.UUID, updatedColumn userstore.Column) (*userstore.Column, error) {
	req := UpdateColumnRequest{
		Column: updatedColumn,
	}

	var resp UpdateColumnResponse
	if err := c.client.Put(ctx, paths.UpdateColumnPath(columnID), req, &resp); err != nil {
		return nil, ucerr.Wrap(err)
	}

	return &resp.Column, nil
}

// CreateAccessorRequest is the request body for creating a new accessor
type CreateAccessorRequest struct {
	Accessor userstore.Accessor `json:"accessor"`
}

//go:generate genvalidate CreateAccessorRequest

// CreateAccessorResponse is the response body for creating a new accessor
type CreateAccessorResponse struct {
	Accessor userstore.Accessor `json:"accessor"`
}

// CreateAccessor creates a new accessor for the associated tenant
func (c *Client) CreateAccessor(ctx context.Context, fa userstore.Accessor, opts ...Option) (*userstore.Accessor, error) {

	options := c.options
	for _, opt := range opts {
		opt.apply(&options)
	}

	req := CreateAccessorRequest{
		Accessor: fa,
	}

	var resp CreateAccessorResponse
	if options.ifNotExists {
		exists, existingID, err := c.client.CreateIfNotExists(ctx, paths.CreateAccessorPath, req, &resp)
		if err != nil {
			return nil, ucerr.Wrap(err)
		}
		if exists {
			resp.Accessor = req.Accessor
			resp.Accessor.ID = existingID
		}
	} else {
		if err := c.client.Post(ctx, paths.CreateAccessorPath, req, &resp); err != nil {
			return nil, ucerr.Wrap(err)
		}
	}

	return &resp.Accessor, nil
}

// DeleteAccessor deletes the accessor specified by the accessor ID for the associated tenant
func (c *Client) DeleteAccessor(ctx context.Context, accessorID uuid.UUID) error {
	return ucerr.Wrap(c.client.Delete(ctx, paths.DeleteAccessorPath(accessorID), nil))
}

// GetAccessor returns the accessor specified by the accessor ID for the associated tenant
func (c *Client) GetAccessor(ctx context.Context, accessorID uuid.UUID) (*userstore.Accessor, error) {
	var resp userstore.Accessor
	if err := c.client.Get(ctx, paths.GetAccessorPath(accessorID), &resp); err != nil {
		return nil, ucerr.Wrap(err)
	}

	return &resp, nil
}

// GetAccessorByVersion returns the version of an accessor specified by the accessor ID and version for the associated tenant
func (c *Client) GetAccessorByVersion(ctx context.Context, accessorID uuid.UUID, version int) (*userstore.Accessor, error) {
	var resp userstore.Accessor
	if err := c.client.Get(ctx, paths.GetAccessorByVersionPath(accessorID, version), &resp); err != nil {
		return nil, ucerr.Wrap(err)
	}

	return &resp, nil
}

// ListAccessorsResponse is the response body for listing accessors
type ListAccessorsResponse struct {
	Accessors []userstore.Accessor `json:"accessors"`
}

// ListAccessors lists all the available accessors for the associated tenant
func (c *Client) ListAccessors(ctx context.Context) ([]userstore.Accessor, error) {
	var res ListAccessorsResponse
	if err := c.client.Get(ctx, paths.ListAccessorsPath, &res); err != nil {
		return nil, ucerr.Wrap(err)
	}

	return res.Accessors, nil
}

// UpdateAccessorRequest is the request body for updating an accessor
type UpdateAccessorRequest struct {
	Accessor userstore.Accessor `json:"accessor"`
}

//go:generate genvalidate UpdateAccessorRequest

// UpdateAccessorResponse is the response body for updating an accessor
type UpdateAccessorResponse struct {
	Accessor userstore.Accessor `json:"accessor"`
}

// UpdateAccessor updates the accessor specified by the accessor ID with the specified data for the associated tenant
func (c *Client) UpdateAccessor(ctx context.Context, accessorID uuid.UUID, updatedAccessor userstore.Accessor) (*userstore.Accessor, error) {
	req := UpdateAccessorRequest{
		Accessor: updatedAccessor,
	}

	var resp UpdateAccessorResponse
	if err := c.client.Put(ctx, paths.UpdateAccessorPath(accessorID), req, &resp); err != nil {
		return nil, ucerr.Wrap(err)
	}

	return &resp.Accessor, nil
}

// CreateMutatorRequest is the request body for creating a new mutator
type CreateMutatorRequest struct {
	Mutator userstore.Mutator `json:"mutator"`
}

//go:generate genvalidate CreateMutatorRequest

// CreateMutatorResponse is the response body for creating a new mutator
type CreateMutatorResponse struct {
	Mutator userstore.Mutator `json:"mutator"`
}

// CreateMutator creates a new mutator for the associated tenant
func (c *Client) CreateMutator(ctx context.Context, fa userstore.Mutator, opts ...Option) (*userstore.Mutator, error) {

	options := c.options
	for _, opt := range opts {
		opt.apply(&options)
	}

	req := CreateMutatorRequest{
		Mutator: fa,
	}

	var resp CreateMutatorResponse
	if options.ifNotExists {
		exists, existingID, err := c.client.CreateIfNotExists(ctx, paths.CreateMutatorPath, req, &resp)
		if err != nil {
			return nil, ucerr.Wrap(err)
		}
		if exists {
			resp.Mutator = req.Mutator
			resp.Mutator.ID = existingID
		}
	} else {
		if err := c.client.Post(ctx, paths.CreateMutatorPath, req, &resp); err != nil {
			return nil, ucerr.Wrap(err)
		}
	}

	return &resp.Mutator, nil
}

// DeleteMutator deletes the mutator specified by the mutator ID for the associated tenant
func (c *Client) DeleteMutator(ctx context.Context, mutatorID uuid.UUID) error {
	return ucerr.Wrap(c.client.Delete(ctx, paths.DeleteMutatorPath(mutatorID), nil))
}

// GetMutator returns the mutator specified by the mutator ID for the associated tenant
func (c *Client) GetMutator(ctx context.Context, mutatorID uuid.UUID) (*userstore.Mutator, error) {
	var resp userstore.Mutator
	if err := c.client.Get(ctx, paths.GetMutatorPath(mutatorID), &resp); err != nil {
		return nil, ucerr.Wrap(err)
	}

	return &resp, nil
}

// GetMutatorByVersion returns the version of an mutator specified by the mutator ID and version for the associated tenant
func (c *Client) GetMutatorByVersion(ctx context.Context, mutatorID uuid.UUID, version int) (*userstore.Mutator, error) {
	var resp userstore.Mutator
	if err := c.client.Get(ctx, paths.GetMutatorByVersionPath(mutatorID, version), &resp); err != nil {
		return nil, ucerr.Wrap(err)
	}

	return &resp, nil
}

// ListMutatorsResponse is the response body for listing mutators
type ListMutatorsResponse struct {
	Mutators []userstore.Mutator `json:"mutators"`
}

// ListMutators lists all the available mutators for the associated tenant
func (c *Client) ListMutators(ctx context.Context) ([]userstore.Mutator, error) {
	var res ListMutatorsResponse
	if err := c.client.Get(ctx, paths.ListMutatorsPath, &res); err != nil {
		return nil, ucerr.Wrap(err)
	}

	return res.Mutators, nil
}

// UpdateMutatorRequest is the request body for updating a mutator
type UpdateMutatorRequest struct {
	Mutator userstore.Mutator `json:"mutator"`
}

//go:generate genvalidate UpdateMutatorRequest

// UpdateMutatorResponse is the response body for updating a mutator
type UpdateMutatorResponse struct {
	Mutator userstore.Mutator `json:"mutator"`
}

// UpdateMutator updates the mutator specified by the mutator ID with the specified data for the associated tenant
func (c *Client) UpdateMutator(ctx context.Context, mutatorID uuid.UUID, updatedMutator userstore.Mutator) (*userstore.Mutator, error) {
	req := UpdateMutatorRequest{
		Mutator: updatedMutator,
	}

	var resp UpdateMutatorResponse
	if err := c.client.Put(ctx, paths.UpdateMutatorPath(mutatorID), req, &resp); err != nil {
		return nil, ucerr.Wrap(err)
	}

	return &resp.Mutator, nil
}

// ExecuteAccessorRequest is the request body for accessing a column
type ExecuteAccessorRequest struct {
	AccessorID     uuid.UUID                    `json:"accessor_id"`     // the accessor that specifies what data to access
	Context        policy.ClientContext         `json:"context"`         // context that is provided to the accessor Access Policy
	SelectorValues userstore.UserSelectorValues `json:"selector_values"` // the values to use for the selector
}

// ExecuteAccessorResponse is the response body for accessing a column
type ExecuteAccessorResponse struct {
	Value []string `json:"value"`
}

// ExecuteAccessor accesses a column via an accessor for the associated tenant
func (c *Client) ExecuteAccessor(ctx context.Context, accessorID uuid.UUID, clientContext policy.ClientContext, selectorValues userstore.UserSelectorValues) ([]string, error) {
	req := ExecuteAccessorRequest{
		AccessorID:     accessorID,
		Context:        clientContext,
		SelectorValues: selectorValues,
	}

	var res ExecuteAccessorResponse
	if err := c.client.Post(ctx, paths.ExecuteAccessorPath, req, &res); err != nil {
		return nil, ucerr.Wrap(err)
	}

	return res.Value, nil
}

type mutatorSystemValue struct {
	SystemValue string `json:"special_value"`
}

// MutatorColumnDefaultValue is a special value that can be used to set a column to its default value
var MutatorColumnDefaultValue = mutatorSystemValue{SystemValue: "default"}

// MutatorColumnCurrentValue is a special value that can be used to set a column to its current value
var MutatorColumnCurrentValue = mutatorSystemValue{SystemValue: "current"}

// ExecuteMutatorRequest is the request body for modifying data in the userstore
type ExecuteMutatorRequest struct {
	MutatorID      uuid.UUID                    `json:"mutator_id"`      // the mutator that specifies what columns to edit
	Context        policy.ClientContext         `json:"context"`         // context that is provided to the mutator's Access Policy
	SelectorValues userstore.UserSelectorValues `json:"selector_values"` // the values to use for the selector
	RowValues      map[string]interface{}       `json:"row_values"`      // the values to use for the users table row
}

// ExecuteMutatorResponse is the response body for modifying data in the userstore
type ExecuteMutatorResponse struct {
	UserIDs []uuid.UUID `json:"user_ids"`
}

// ExecuteMutator modifies columns in userstore via a mutator for the associated tenant
func (c *Client) ExecuteMutator(ctx context.Context, mutatorID uuid.UUID, clientContext policy.ClientContext, selectorValues userstore.UserSelectorValues, rowValues map[string]interface{}) ([]uuid.UUID, error) {
	req := ExecuteMutatorRequest{
		MutatorID:      mutatorID,
		Context:        clientContext,
		SelectorValues: selectorValues,
		RowValues:      rowValues,
	}

	var resp ExecuteMutatorResponse
	if err := c.client.Post(ctx, paths.ExecuteMutatorPath, req, &resp); err != nil {
		return nil, ucerr.Wrap(err)
	}

	return resp.UserIDs, nil
}
