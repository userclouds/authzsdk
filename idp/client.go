package idp

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/gofrs/uuid"

	"userclouds.com/idp/paths"
	"userclouds.com/idp/socialprovider"
	"userclouds.com/idp/userstore"
	"userclouds.com/infra/emailutil"
	"userclouds.com/infra/jsonclient"
	"userclouds.com/infra/ucerr"
	"userclouds.com/policy"
)

// AuthnType defines the kinds of authentication factors
type AuthnType string

// AuthnType constants
const (
	AuthnTypePassword AuthnType = "password"
	AuthnTypeSocial   AuthnType = "social"

	// Used for filter queries; not a valid type
	AuthnTypeAll AuthnType = "all"
)

// Validate implements Validateable
func (a AuthnType) Validate() error {
	if a == AuthnTypePassword || a == AuthnTypeSocial || a == AuthnTypeAll {
		return nil
	}
	return ucerr.Errorf("invalid AuthnType: %s", string(a))
}

// Client represents a client to talk to the Userclouds IDP
type Client struct {
	client *jsonclient.Client
}

// NewClient constructs a new IDP client
func NewClient(url string, opts ...jsonclient.Option) (*Client, error) {
	c := &Client{
		client: jsonclient.New(strings.TrimSuffix(url, "/"), opts...),
	}
	if err := c.client.ValidateBearerTokenHeader(); err != nil {
		return nil, ucerr.Wrap(err)
	}
	return c, nil
}

// UserAuthn represents an authentication factor for a user.
// NOTE: some fields are not used in some circumstances, e.g. Password is only
// used when creating an account but never used when getting an account.
// TODO: use this for UpdateUser too.
type UserAuthn struct {
	AuthnType AuthnType `json:"authn_type"`

	// Fields specified if AuthnType == 'password'
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`

	// Fields specified if AuthnType == 'social'
	SocialProvider socialprovider.SocialProvider `json:"social_provider,omitempty"`
	OIDCSubject    string                        `json:"oidc_subject,omitempty"`
}

// UserProfile is a collection of per-user properties stored in the DB as JSON since
// they are likely to be sparse and change more frequently.
// Follow conventions of https://openid.net/specs/openid-connect-core-1_0.html#StandardClaims for
// all standard fields.
type UserProfile struct {
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	Name          string `json:"name,omitempty"`     // Full name in displayable form (incl titles, suffixes, etc) localized to end-user.
	Nickname      string `json:"nickname,omitempty"` // Casual name of the user, may or may not be same as Given Name.
	Picture       string `json:"picture,omitempty"`  // URL of the user's profile picture.

	// TODO: email is tricky; it's used for authn, 2fa, and (arguably) user profile.
	// If a user merges authns (e.g. I had 2 accounts, oops), then there can be > 1.
	// It may make sense to keep the primary user email (used for 2FA) in `User`, separately
	// from the profile, but allow 0+ profile emails (e.g. alternate contacts, merged accounts, etc).
}

//go:generate gendbjson UserProfile

//go:generate genvalidate UserProfile

func (up UserProfile) extraValidate() error {
	if up.Email == "" {
		return nil
	}
	a := emailutil.Address(up.Email)
	if err := a.Validate(); err != nil {
		return ucerr.Wrap(err)
	}
	return nil
}

// CreateUserAndAuthnRequest creates a user on the IDP
type CreateUserAndAuthnRequest struct {
	UserProfile `json:"profile"`

	// TODO: these fields really belong in a better client-facing User type
	ExternalAlias *string `json:"external_alias,omitempty"`
	RequireMFA    bool    `json:"require_mfa"`

	UserExtendedProfile userstore.Record `json:"profile_ext"`

	UserAuthn
}

// UserAndAuthnResponse is the response body for methods which return user data.
type UserAndAuthnResponse struct {
	ID        uuid.UUID `json:"id"`
	UpdatedAt int64     `json:"updated_at"` // seconds since the Unix Epoch (UTC)

	UserProfile `json:"profile"`

	ExternalAlias *string `json:"external_alias,omitempty"`
	RequireMFA    bool    `json:"require_mfa"`

	UserExtendedProfile userstore.Record `json:"profile_ext"`

	Authns []UserAuthn `json:"authns"`
}

// CreateUser creates a user without authn. extendedProfile & externalAlias are optional (nil is ok)
func (c *Client) CreateUser(ctx context.Context,
	profile UserProfile,
	extendedProfile userstore.Record,
	externalAlias string) (uuid.UUID, error) {
	// TODO: we don't validate the profile here, since we don't require email in this path
	// this probably should be refactored to be more consistent in this client

	req := CreateUserAndAuthnRequest{
		UserProfile:         profile,
		UserExtendedProfile: extendedProfile,
	}

	if externalAlias != "" {
		req.ExternalAlias = &externalAlias
	}

	var res UserAndAuthnResponse
	if err := c.client.Post(ctx, paths.CreateUser, req, &res); err != nil {
		return uuid.Nil, ucerr.Wrap(err)
	}

	return res.ID, nil
}

// GetUser gets a user by ID
func (c *Client) GetUser(ctx context.Context, id uuid.UUID) (*UserAndAuthnResponse, error) {
	var res UserAndAuthnResponse

	requestURL := url.URL{
		Path: fmt.Sprintf("/authn/users/%s", id),
	}

	if err := c.client.Get(ctx, requestURL.String(), &res); err != nil {
		return nil, ucerr.Wrap(err)
	}

	return &res, nil
}

// GetUserByExternalAlias gets a user by external alias
func (c *Client) GetUserByExternalAlias(ctx context.Context, alias string) (*UserAndAuthnResponse, error) {
	u := url.URL{
		Path: paths.GetUserByExternalAlias,
		RawQuery: url.Values{
			"external_alias": []string{alias},
		}.Encode(),
	}

	var res UserAndAuthnResponse
	if err := c.client.Get(ctx, u.String(), &res); err != nil {
		return nil, ucerr.Wrap(err)
	}

	return &res, nil
}

// MutableUserProfile is used by UpdateUserRequest to update parts of the core user profile.
// Only non-nil fields in the underlying struct will be updated.
type MutableUserProfile struct {
	EmailVerified *bool   `json:"email_verified,omitempty"`
	Name          *string `json:"name,omitempty"`
	Nickname      *string `json:"nickname,omitempty"`
	Picture       *string `json:"picture,omitempty"`
}

// UpdateUserRequest optionally updates some or all mutable fields of a user struct.
// Pointers are used to distinguish between unset vs. set to default value (false, "", etc).
// TODO: should we allow changing Email? That's a more complex one as there are more implications to
// changing email that may affect AuthNs and security (e.g. account hijacking, unverified emails, etc).
type UpdateUserRequest struct {
	UserProfile MutableUserProfile `json:"profile"`

	// TODO: add MFA factors
	RequireMFA *bool `json:"require_mfa,omitempty"`

	// Only fields set in the underlying map will be updated
	UserExtendedProfile userstore.Record `json:"profile_ext"`
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

// CreateColumnResponse is the response body for creating a new column
type CreateColumnResponse struct {
	Column userstore.Column `json:"column"`
}

// CreateColumn creates a new column for the associated tenant
func (c *Client) CreateColumn(ctx context.Context, column userstore.Column) (*userstore.Column, error) {
	req := CreateColumnRequest{
		Column: column,
	}
	var res CreateColumnResponse
	if err := c.client.Post(ctx, paths.CreateColumnPath, req, &res); err != nil {
		return nil, ucerr.Wrap(err)
	}

	return &res.Column, nil
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
	Accessor userstore.ColumnAccessor `json:"accessor"`
}

// CreateAccessorResponse is the response body for creating a new accessor
type CreateAccessorResponse struct {
	Accessor userstore.ColumnAccessor `json:"accessor"`
}

// CreateAccessor creates a new accessor for the associated tenant
func (c *Client) CreateAccessor(ctx context.Context, fa userstore.ColumnAccessor) (*userstore.ColumnAccessor, error) {
	req := CreateAccessorRequest{
		Accessor: fa,
	}
	var res CreateAccessorResponse
	if err := c.client.Post(ctx, paths.CreateAccessorPath, req, &res); err != nil {
		return nil, ucerr.Wrap(err)
	}

	return &res.Accessor, nil
}

// DeleteAccessor deletes the accessor specified by the accessor ID for the associated tenant
func (c *Client) DeleteAccessor(ctx context.Context, accessorID uuid.UUID) error {
	return ucerr.Wrap(c.client.Delete(ctx, paths.DeleteAccessorPath(accessorID), nil))
}

// GetAccessor returns the accessor specified by the accessor ID for the associated tenant
func (c *Client) GetAccessor(ctx context.Context, accessorID uuid.UUID) (*userstore.ColumnAccessor, error) {
	var resp userstore.ColumnAccessor
	if err := c.client.Get(ctx, paths.GetAccessorPath(accessorID), &resp); err != nil {
		return nil, ucerr.Wrap(err)
	}

	return &resp, nil
}

// ListAccessorsResponse is the response body for listing accessors
type ListAccessorsResponse struct {
	Accessors []userstore.ColumnAccessor `json:"accessors"`
}

// ListAccessors lists all the available accessors for the associated tenant
func (c *Client) ListAccessors(ctx context.Context) ([]userstore.ColumnAccessor, error) {
	var res ListAccessorsResponse
	if err := c.client.Get(ctx, paths.ListAccessorsPath, &res); err != nil {
		return nil, ucerr.Wrap(err)
	}

	return res.Accessors, nil
}

// UpdateAccessorRequest is the request body for updating an accessor
type UpdateAccessorRequest struct {
	Accessor userstore.ColumnAccessor `json:"accessor"`
}

// UpdateAccessorResponse is the response body for updating an accessor
type UpdateAccessorResponse struct {
	Accessor userstore.ColumnAccessor `json:"accessor"`
}

// UpdateAccessor updates the accessor specified by the accessor ID with the specified data for the associated tenant
func (c *Client) UpdateAccessor(ctx context.Context, accessorID uuid.UUID, updatedAccessor userstore.ColumnAccessor) (*userstore.ColumnAccessor, error) {
	req := UpdateAccessorRequest{
		Accessor: updatedAccessor,
	}

	var resp UpdateAccessorResponse
	if err := c.client.Put(ctx, paths.UpdateAccessorPath(accessorID), req, &resp); err != nil {
		return nil, ucerr.Wrap(err)
	}

	return &resp.Accessor, nil
}

// UserSelector lets you request the user to run an accessor on
// Currently we only support UserClouds ID or your own ID (ExternalAlias)
// but plan to enhance this soon.
type UserSelector struct {
	ID            uuid.UUID `json:"id"`
	ExternalAlias string    `json:"external_alias"` // TODO: using this here makes me think we should rename it
}

// ExecuteAccessorRequest is the request body for accessing a column
type ExecuteAccessorRequest struct {
	User       UserSelector         `json:"user"`        // the user who's data you are accessing
	AccessorID uuid.UUID            `json:"accessor_id"` // the accessor that specifies what you're accessing
	Context    policy.ClientContext `json:"context"`     // context that is provided to the accessor Access Policy
}

// ExecuteAccessorResponse is the response body for accessing a column
type ExecuteAccessorResponse struct {
	Value string `json:"value"`
}

// ExecuteAccessor accesses a column via an accessor for the associated tenant
func (c *Client) ExecuteAccessor(ctx context.Context, user UserSelector, accessorID uuid.UUID, clientContext policy.ClientContext) (string, error) {
	req := ExecuteAccessorRequest{
		User:       user,
		AccessorID: accessorID,
		Context:    clientContext,
	}

	var res ExecuteAccessorResponse
	if err := c.client.Post(ctx, paths.ExecuteAccessorPath, req, &res); err != nil {
		return "", ucerr.Wrap(err)
	}

	return res.Value, nil
}
