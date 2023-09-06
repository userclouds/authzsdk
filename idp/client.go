package idp

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/gofrs/uuid"

	"userclouds.com/idp/paths"
	"userclouds.com/idp/policy"
	"userclouds.com/idp/userstore"
	"userclouds.com/infra/jsonclient"
	"userclouds.com/infra/pagination"
	"userclouds.com/infra/sdkclient"
	"userclouds.com/infra/ucerr"
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
	client          *sdkclient.Client
	options         options
	TokenizerClient *TokenizerClient
}

// NewClient constructs a new IDP client
func NewClient(url string, opts ...Option) (*Client, error) {

	var options options
	for _, opt := range opts {
		opt.apply(&options)
	}

	c := &Client{
		client:  sdkclient.New(strings.TrimSuffix(url, "/"), options.jsonclientOptions...),
		options: options,
	}
	c.TokenizerClient = &TokenizerClient{client: c.client, options: options}

	if err := c.client.ValidateBearerTokenHeader(); err != nil {
		return nil, ucerr.Wrap(err)
	}
	return c, nil
}

// CreateUserAndAuthnRequest creates a user on the IDP
type CreateUserAndAuthnRequest struct {
	Profile userstore.Record `json:"profile"`

	OrganizationID uuid.UUID `json:"organization_id"`

	UserAuthn
}

// UserAndAuthnResponse is the response body for methods which return user data.
type UserAndAuthnResponse struct {
	ID        uuid.UUID `json:"id"`
	UpdatedAt int64     `json:"updated_at"` // seconds since the Unix Epoch (UTC)

	Profile userstore.Record `json:"profile"`

	OrganizationID uuid.UUID `json:"organization_id"`

	Authns []UserAuthn `json:"authns"`

	MFAChannels []UserMFAChannel `json:"mfa_channels"`
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

// CreateColumn creates a new column for the associated tenant
func (c *Client) CreateColumn(ctx context.Context, column userstore.Column, opts ...Option) (*userstore.Column, error) {

	options := c.options
	for _, opt := range opts {
		opt.apply(&options)
	}

	req := CreateColumnRequest{
		Column: column,
	}

	var resp userstore.Column
	if options.ifNotExists {
		exists, existingID, err := c.client.CreateIfNotExists(ctx, paths.CreateColumnPath, req, &resp)
		if err != nil {
			return nil, ucerr.Wrap(err)
		}
		if exists {
			resp = req.Column
			resp.ID = existingID
		}
	} else {
		if err := c.client.Post(ctx, paths.CreateColumnPath, req, &resp); err != nil {
			return nil, ucerr.Wrap(err)
		}
	}

	return &resp, nil
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

// ListColumnsResponse is the paginated response struct for listing columns
type ListColumnsResponse struct {
	Data []userstore.Column `json:"data"`
	pagination.ResponseFields
}

// ListColumns lists all columns for the associated tenant
func (c *Client) ListColumns(ctx context.Context, opts ...Option) (*ListColumnsResponse, error) {
	options := c.options
	for _, opt := range opts {
		opt.apply(&options)
	}
	pager, err := pagination.ApplyOptions(options.paginationOptions...)
	if err != nil {
		return nil, ucerr.Wrap(err)
	}

	query := pager.Query()

	var res ListColumnsResponse
	if err := c.client.Get(ctx, fmt.Sprintf("%s?%s", paths.ListColumnsPath, query.Encode()), &res); err != nil {
		return nil, ucerr.Wrap(err)
	}

	return &res, nil
}

// UpdateColumnRequest is the request body for updating a column
type UpdateColumnRequest struct {
	Column userstore.Column `json:"column"`
}

//go:generate genvalidate UpdateColumnRequest

// UpdateColumn updates the column specified by the column ID with the specified data for the associated tenant
func (c *Client) UpdateColumn(ctx context.Context, columnID uuid.UUID, updatedColumn userstore.Column) (*userstore.Column, error) {
	req := UpdateColumnRequest{
		Column: updatedColumn,
	}

	var resp userstore.Column
	if err := c.client.Put(ctx, paths.UpdateColumnPath(columnID), req, &resp); err != nil {
		return nil, ucerr.Wrap(err)
	}

	return &resp, nil
}

// DurationType identifies whether a duration is for a pre-deleted
// (i.e., "live") value or a post-deleted value
type DurationType int

// Supported duration types
const (
	DurationTypePreDelete  DurationType = 1
	DurationTypePostDelete DurationType = 2
)

//go:generate genconstant DurationType

// DurationUnit identifies the unit of measurement for a duration
type DurationUnit int

// Supported duration units
const (
	DurationUnitIndefinite DurationUnit = 1
	DurationUnitYear       DurationUnit = 2
	DurationUnitMonth      DurationUnit = 3
	DurationUnitWeek       DurationUnit = 4
	DurationUnitDay        DurationUnit = 5
	DurationUnitHour       DurationUnit = 6
)

//go:generate genconstant DurationUnit

// RetentionDuration represents a duration with a specific duration unit
type RetentionDuration struct {
	Unit     DurationUnit `json:"unit"`
	Duration int          `json:"duration"`
}

func (d *RetentionDuration) extraValidate() error {
	if d.Duration < 0 {
		return ucerr.New("Duration must be non-negative")
	}

	if d.Unit == DurationUnitIndefinite && d.Duration != 0 {
		return ucerr.New("Duration must be 0 if Unit is DurationUnitIndefinite")
	}

	return nil
}

//go:generate genvalidate RetentionDuration

// AddToTime will add the retention duration to a passed in time
func (d RetentionDuration) AddToTime(t time.Time) time.Time {
	switch d.Unit {
	case DurationUnitIndefinite:
		return time.Time{}
	case DurationUnitYear:
		return t.AddDate(d.Duration, 0, 0)
	case DurationUnitMonth:
		return t.AddDate(0, d.Duration, 0)
	case DurationUnitWeek:
		return t.AddDate(0, 0, 7*d.Duration)
	case DurationUnitDay:
		return t.AddDate(0, 0, d.Duration)
	case DurationUnitHour:
		return t.Add(time.Duration(d.Duration) * time.Hour)
	}

	return t
}

// LessThan returns true if the duration is strictly smaller than other
func (d RetentionDuration) LessThan(other RetentionDuration) bool {
	var t time.Time
	return d.AddToTime(t).Before(other.AddToTime(t))
}

// ColumnRetentionDurationType identifies the type of the retention duration
type ColumnRetentionDurationType int

// Supported column retention duration types
const (
	ColumnRetentionDurationTypeTenant   ColumnRetentionDurationType = 1
	ColumnRetentionDurationTypeColumn   ColumnRetentionDurationType = 2
	ColumnRetentionDurationTypePurpose  ColumnRetentionDurationType = 3
	ColumnRetentionDurationTypeSpecific ColumnRetentionDurationType = 4
)

//go:generate genconstant ColumnRetentionDurationType

// ColumnRetentionDuration represents an identified retention duration. If ID is nil, it
// represents an inherited or new value. UseDefault set to true means that the duration is
// inherited from a less specific default value. DefaultDuration represents the duration
// that would be inherited if a specific value is not set for the retention duration identifier.
type ColumnRetentionDuration struct {
	Type            ColumnRetentionDurationType `json:"type"`
	ID              uuid.UUID                   `json:"id" validate:"skip"`
	Version         int                         `json:"version"`
	ColumnID        uuid.UUID                   `json:"column_id" validate:"skip"`
	PurposeID       uuid.UUID                   `json:"purpose_id" validate:"skip"`
	DurationType    DurationType                `json:"duration_type"`
	PurposeName     string                      `json:"purpose_name"`
	Duration        RetentionDuration           `json:"duration"`
	UseDefault      bool                        `json:"use_default"`
	DefaultDuration RetentionDuration           `json:"default_duration"`
}

func (d *ColumnRetentionDuration) extraValidate() error {
	switch d.Type {
	case ColumnRetentionDurationTypeTenant:
		if d.ColumnID != uuid.Nil || d.PurposeID != uuid.Nil {
			return ucerr.New("ColumnID and PurposeID must be nil for tenant type")
		}
	case ColumnRetentionDurationTypeColumn:
		if d.ColumnID == uuid.Nil || d.PurposeID == uuid.Nil {
			return ucerr.New("ColumnID and PurposeID must be non-nil for column type")
		}
	case ColumnRetentionDurationTypePurpose:
		if d.ColumnID != uuid.Nil || d.PurposeID == uuid.Nil {
			return ucerr.New("PurposeID must be non-nil and ColumnID must be nil for purpose type")
		}
	case ColumnRetentionDurationTypeSpecific:
		if d.ID == uuid.Nil {
			return ucerr.New("ID must be non-nil for specific type")
		}
	}

	if d.PurposeID != uuid.Nil && d.PurposeName == "" {
		return ucerr.New("PurposeName must be specified if PurposeID is non-nil")
	}

	if d.PurposeID == uuid.Nil && d.PurposeName != "" {
		return ucerr.New("PurposeName must be empty if PurposeID is nil")
	}

	if d.UseDefault && d.Duration != d.DefaultDuration {
		return ucerr.New("Duration must equal DefaultDuration if UseDefault is true")
	}

	if d.DurationType == DurationTypePreDelete {
		if d.Duration.Unit != DurationUnitIndefinite && d.Duration.Duration == 0 {
			return ucerr.New("DurationTypePreDelete cannot have a duration of 0")
		}
	} else if d.Duration.Unit == DurationUnitIndefinite {
		return ucerr.New("DurationTypePostDelete cannot have an indefinite duration")
	}

	return nil
}

//go:generate genvalidate ColumnRetentionDuration

// ColumnRetentionDurationsResponse is the response to a get or update request for retention
// durations. The set of retention durations that apply for the request will be returned,
// along with a max allowed retention duration appropriate for the request parameters. Each
// of the retention durations will have a non-nil ID if they are saved values, or a nil ID
// if they represent an inherited value.
type ColumnRetentionDurationsResponse struct {
	MaxDuration        RetentionDuration         `json:"max_duration"`
	RetentionDurations []ColumnRetentionDuration `json:"retention_durations"`
}

//go:generate genvalidate ColumnRetentionDurationsResponse

// GetColumnRetentionDurations returns the retention durations for the specified column
// and duration type
func (c *Client) GetColumnRetentionDurations(
	ctx context.Context,
	columnID uuid.UUID,
	dt DurationType,
) (*ColumnRetentionDurationsResponse, error) {
	path := paths.GetColumnRetentionDurationURL(columnID, dt == DurationTypePreDelete)

	var resp ColumnRetentionDurationsResponse
	if err := c.client.Get(ctx, path, &resp); err != nil {
		return nil, ucerr.Wrap(err)
	}

	return &resp, nil
}

// UpdateColumnRetentionDurationsRequest is used to update the specified set of retention durations
// for a column. If ID for a retention duration is non-nil, that retention duration will be updated
// if UseDefault is set to false, or deleted if UseDefault is set to true. If ID is nil, the
// associated retention duration will be inserted.
type UpdateColumnRetentionDurationsRequest struct {
	RetentionDurations []ColumnRetentionDuration `json:"retention_durations"`
}

func (r UpdateColumnRetentionDurationsRequest) extraValidate() error {
	if len(r.RetentionDurations) == 0 {
		return ucerr.New("no retentions to update")
	}

	return nil
}

//go:generate genvalidate UpdateColumnRetentionDurationsRequest

// UpdateColumnRetentionDurations updates the column retention durations
// for the specified column and duration type, returning the updated set
// of retention durations for the column and duration type.
func (c *Client) UpdateColumnRetentionDurations(
	ctx context.Context,
	columnID uuid.UUID,
	dt DurationType,
	req UpdateColumnRetentionDurationsRequest,
) (*ColumnRetentionDurationsResponse, error) {
	path := paths.GetColumnRetentionDurationURL(columnID, dt == DurationTypePreDelete)

	var resp ColumnRetentionDurationsResponse
	if err := c.client.Post(ctx, path, req, &resp); err != nil {
		return nil, ucerr.Wrap(err)
	}

	return &resp, nil
}

// CreateAccessorRequest is the request body for creating a new accessor
type CreateAccessorRequest struct {
	Accessor userstore.Accessor `json:"accessor"`
}

//go:generate genvalidate CreateAccessorRequest

// CreateAccessor creates a new accessor for the associated tenant
func (c *Client) CreateAccessor(ctx context.Context, fa userstore.Accessor, opts ...Option) (*userstore.Accessor, error) {

	options := c.options
	for _, opt := range opts {
		opt.apply(&options)
	}

	req := CreateAccessorRequest{
		Accessor: fa,
	}

	var resp userstore.Accessor
	if options.ifNotExists {
		exists, existingID, err := c.client.CreateIfNotExists(ctx, paths.CreateAccessorPath, req, &resp)
		if err != nil {
			return nil, ucerr.Wrap(err)
		}
		if exists {
			resp = req.Accessor
			resp.ID = existingID
		}
	} else {
		if err := c.client.Post(ctx, paths.CreateAccessorPath, req, &resp); err != nil {
			return nil, ucerr.Wrap(err)
		}
	}

	return &resp, nil
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

// ListAccessorsResponse is the paginated response from listing accessors.
type ListAccessorsResponse struct {
	Data []userstore.Accessor `json:"data"`
	pagination.ResponseFields
}

// ListAccessors lists all the available accessors for the associated tenant
func (c *Client) ListAccessors(ctx context.Context, opts ...Option) (*ListAccessorsResponse, error) {
	options := c.options
	for _, opt := range opts {
		opt.apply(&options)
	}
	pager, err := pagination.ApplyOptions(options.paginationOptions...)
	if err != nil {
		return nil, ucerr.Wrap(err)
	}

	query := pager.Query()

	var res ListAccessorsResponse
	if err := c.client.Get(ctx, fmt.Sprintf("%s?%s", paths.ListAccessorsPath, query.Encode()), &res); err != nil {
		return nil, ucerr.Wrap(err)
	}

	return &res, nil
}

// UpdateAccessorRequest is the request body for updating an accessor
type UpdateAccessorRequest struct {
	Accessor userstore.Accessor `json:"accessor"`
}

//go:generate genvalidate UpdateAccessorRequest

// UpdateAccessor updates the accessor specified by the accessor ID with the specified data for the associated tenant
func (c *Client) UpdateAccessor(ctx context.Context, accessorID uuid.UUID, updatedAccessor userstore.Accessor) (*userstore.Accessor, error) {
	req := UpdateAccessorRequest{
		Accessor: updatedAccessor,
	}

	var resp userstore.Accessor
	if err := c.client.Put(ctx, paths.UpdateAccessorPath(accessorID), req, &resp); err != nil {
		return nil, ucerr.Wrap(err)
	}

	return &resp, nil
}

// CreateMutatorRequest is the request body for creating a new mutator
type CreateMutatorRequest struct {
	Mutator userstore.Mutator `json:"mutator"`
}

//go:generate genvalidate CreateMutatorRequest

// CreateMutator creates a new mutator for the associated tenant
func (c *Client) CreateMutator(ctx context.Context, fa userstore.Mutator, opts ...Option) (*userstore.Mutator, error) {

	options := c.options
	for _, opt := range opts {
		opt.apply(&options)
	}

	req := CreateMutatorRequest{
		Mutator: fa,
	}

	var resp userstore.Mutator
	if options.ifNotExists {
		exists, existingID, err := c.client.CreateIfNotExists(ctx, paths.CreateMutatorPath, req, &resp)
		if err != nil {
			return nil, ucerr.Wrap(err)
		}
		if exists {
			resp = req.Mutator
			resp.ID = existingID
		}
	} else {
		if err := c.client.Post(ctx, paths.CreateMutatorPath, req, &resp); err != nil {
			return nil, ucerr.Wrap(err)
		}
	}

	return &resp, nil
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

// ListMutatorsResponse is the paginated response from listing mutators.
type ListMutatorsResponse struct {
	Data []userstore.Mutator `json:"data"`
	pagination.ResponseFields
}

// ListMutators lists all the available mutators for the associated tenant
func (c *Client) ListMutators(ctx context.Context, opts ...Option) (*ListMutatorsResponse, error) {
	options := c.options
	for _, opt := range opts {
		opt.apply(&options)
	}
	pager, err := pagination.ApplyOptions(options.paginationOptions...)
	if err != nil {
		return nil, ucerr.Wrap(err)
	}

	query := pager.Query()

	var res ListMutatorsResponse
	if err := c.client.Get(ctx, fmt.Sprintf("%s?%s", paths.ListMutatorsPath, query.Encode()), &res); err != nil {
		return nil, ucerr.Wrap(err)
	}

	return &res, nil
}

// UpdateMutatorRequest is the request body for updating a mutator
type UpdateMutatorRequest struct {
	Mutator userstore.Mutator `json:"mutator"`
}

//go:generate genvalidate UpdateMutatorRequest

// UpdateMutator updates the mutator specified by the mutator ID with the specified data for the associated tenant
func (c *Client) UpdateMutator(ctx context.Context, mutatorID uuid.UUID, updatedMutator userstore.Mutator) (*userstore.Mutator, error) {
	req := UpdateMutatorRequest{
		Mutator: updatedMutator,
	}

	var resp userstore.Mutator
	if err := c.client.Put(ctx, paths.UpdateMutatorPath(mutatorID), req, &resp); err != nil {
		return nil, ucerr.Wrap(err)
	}

	return &resp, nil
}

// ExecuteAccessorRequest is the request body for accessing user data
type ExecuteAccessorRequest struct {
	AccessorID     uuid.UUID                    `json:"accessor_id"`     // the accessor that specifies what data to access
	Context        policy.ClientContext         `json:"context"`         // context that is provided to the accessor Access Policy
	SelectorValues userstore.UserSelectorValues `json:"selector_values"` // the values to use for the selector
}

// ExecuteAccessorResponse is the response body for accessing user data
type ExecuteAccessorResponse struct {
	Data []string `json:"data"`
}

// ExecuteAccessor accesses a column via an accessor for the associated tenant
func (c *Client) ExecuteAccessor(ctx context.Context, accessorID uuid.UUID, clientContext policy.ClientContext, selectorValues userstore.UserSelectorValues) (*ExecuteAccessorResponse, error) {
	req := ExecuteAccessorRequest{
		AccessorID:     accessorID,
		Context:        clientContext,
		SelectorValues: selectorValues,
	}

	var res ExecuteAccessorResponse
	if err := c.client.Post(ctx, paths.ExecuteAccessorPath, req, &res); err != nil {
		return nil, ucerr.Wrap(err)
	}

	return &res, nil
}

type mutatorSystemValue struct {
	SystemValue string `json:"special_value"`
}

// MutatorColumnDefaultValue is a special value that can be used to set a column to its default value
var MutatorColumnDefaultValue = mutatorSystemValue{SystemValue: "default"}

// MutatorColumnCurrentValue is a special value that can be used to set a column to its current value
var MutatorColumnCurrentValue = mutatorSystemValue{SystemValue: "current"}

// ValueAndPurposes is a tuple for specifying the value and the purpose to store for a user column
type ValueAndPurposes struct {
	Value            any                    `json:"value"`
	PurposeAdditions []userstore.ResourceID `json:"purpose_additions"`
	PurposeDeletions []userstore.ResourceID `json:"purpose_deletions"`
}

// ExecuteMutatorRequest is the request body for modifying data in the userstore
type ExecuteMutatorRequest struct {
	MutatorID      uuid.UUID                    `json:"mutator_id"`      // the mutator that specifies what columns to edit
	Context        policy.ClientContext         `json:"context"`         // context that is provided to the mutator's Access Policy
	SelectorValues userstore.UserSelectorValues `json:"selector_values"` // the values to use for the selector
	RowData        map[string]ValueAndPurposes  `json:"row_data"`        // the values to use for the users table row
}

// ExecuteMutatorResponse is the response body for modifying data in the userstore
type ExecuteMutatorResponse struct {
	UserIDs []uuid.UUID `json:"user_ids"`
}

// ExecuteMutator modifies columns in userstore via a mutator for the associated tenant
func (c *Client) ExecuteMutator(ctx context.Context, mutatorID uuid.UUID, clientContext policy.ClientContext, selectorValues userstore.UserSelectorValues, rowData map[string]ValueAndPurposes) (*ExecuteMutatorResponse, error) {
	req := ExecuteMutatorRequest{
		MutatorID:      mutatorID,
		Context:        clientContext,
		SelectorValues: selectorValues,
		RowData:        rowData,
	}

	var resp ExecuteMutatorResponse
	if err := c.client.Post(ctx, paths.ExecuteMutatorPath, req, &resp); err != nil {
		return nil, ucerr.Wrap(err)
	}

	return &resp, nil
}

// CreatePurposeRequest is the request body for creating a new purpose
type CreatePurposeRequest struct {
	Purpose userstore.Purpose `json:"purpose"`
}

//go:generate genvalidate CreatePurposeRequest

// CreatePurpose creates a new purpose for the associated tenant
func (c *Client) CreatePurpose(ctx context.Context, purpose userstore.Purpose, opts ...Option) (*userstore.Purpose, error) {
	options := c.options
	for _, opt := range opts {
		opt.apply(&options)
	}

	req := CreatePurposeRequest{
		Purpose: purpose,
	}

	var resp userstore.Purpose
	if options.ifNotExists {
		exists, existingID, err := c.client.CreateIfNotExists(ctx, paths.CreatePurposePath, req, &resp)
		if err != nil {
			return nil, ucerr.Wrap(err)
		}
		if exists {
			resp = req.Purpose
			resp.ID = existingID
		}
	} else {
		if err := c.client.Post(ctx, paths.CreatePurposePath, req, &resp); err != nil {
			return nil, ucerr.Wrap(err)
		}
	}

	return &resp, nil
}

// GetPurpose gets a purpose by ID
func (c *Client) GetPurpose(ctx context.Context, purposeID uuid.UUID) (*userstore.Purpose, error) {
	var resp userstore.Purpose
	if err := c.client.Get(ctx, paths.GetPurposePath(purposeID), &resp); err != nil {
		return nil, ucerr.Wrap(err)
	}

	return &resp, nil
}

// ListPurposesResponse is the paginated response struct for listing purposes
type ListPurposesResponse struct {
	Data []userstore.Purpose `json:"data"`
	pagination.ResponseFields
}

// ListPurposes lists all purposes for the associated tenant
func (c *Client) ListPurposes(ctx context.Context, opts ...Option) (*ListPurposesResponse, error) {

	options := c.options
	for _, opt := range opts {
		opt.apply(&options)
	}

	pager, err := pagination.ApplyOptions(options.paginationOptions...)
	if err != nil {
		return nil, ucerr.Wrap(err)
	}
	query := pager.Query()

	var res ListPurposesResponse
	if err := c.client.Get(ctx, fmt.Sprintf("%s?%s", paths.ListPurposesPath, query.Encode()), &res); err != nil {
		return nil, ucerr.Wrap(err)
	}

	return &res, nil
}

// UpdatePurposeRequest is the request body for updating a purpose
type UpdatePurposeRequest struct {
	Purpose userstore.Purpose `json:"purpose"`
}

//go:generate genvalidate UpdatePurposeRequest

// UpdatePurpose updates a purpose for the associated tenant
func (c *Client) UpdatePurpose(ctx context.Context, purpose userstore.Purpose) (*userstore.Purpose, error) {
	req := UpdatePurposeRequest{
		Purpose: purpose,
	}

	var resp userstore.Purpose
	if err := c.client.Put(ctx, paths.UpdatePurposePath(purpose.ID), req, &resp); err != nil {
		return nil, ucerr.Wrap(err)
	}

	return &resp, nil
}

// DeletePurpose deletes a purpose by ID
func (c *Client) DeletePurpose(ctx context.Context, purposeID uuid.UUID) error {
	if err := c.client.Delete(ctx, paths.DeletePurposePath(purposeID), nil); err != nil {
		return ucerr.Wrap(err)
	}

	return nil
}

// GetConsentedPurposesForUserRequest is the request body for getting the purposes that are consented for a user
type GetConsentedPurposesForUserRequest struct {
	UserID  uuid.UUID              `json:"user_id"`
	Columns []userstore.ResourceID `json:"columns"`
}

// ColumnConsentedPurposes is a tuple for specifying the column and the purposes that are consented for that column
type ColumnConsentedPurposes struct {
	Column            userstore.ResourceID   `json:"column"`
	ConsentedPurposes []userstore.ResourceID `json:"consented_purposes"`
}

// GetConsentedPurposesForUserResponse is the response body for getting the purposes that are consented for a user
type GetConsentedPurposesForUserResponse struct {
	Data []ColumnConsentedPurposes `json:"data"`
}

// GetConsentedPurposesForUser gets the purposes that are consented for a user
func (c *Client) GetConsentedPurposesForUser(ctx context.Context, userID uuid.UUID, columns []userstore.ResourceID) (GetConsentedPurposesForUserResponse, error) {
	req := GetConsentedPurposesForUserRequest{
		UserID:  userID,
		Columns: columns,
	}

	var resp GetConsentedPurposesForUserResponse
	if err := c.client.Post(ctx, paths.GetConsentedPurposesForUserPath, req, &resp); err != nil {
		return resp, ucerr.Wrap(err)
	}

	return resp, nil
}
