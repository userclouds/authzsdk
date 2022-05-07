package idp

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/gofrs/uuid"

	"userclouds.com/idp/userstore"
	"userclouds.com/infra/emailutil"
	"userclouds.com/infra/jsonclient"
	"userclouds.com/infra/ucerr"
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

// SocialProvider defines the known External/Social Identity Providers
type SocialProvider int

// SocialProvider constants
const (
	// When sync'ing data from other IDPs, it's possible to encounter social auth providers not yet supported,
	// in which case we store SocialProviderUnsupported in the DB.
	SocialProviderUnsupported SocialProvider = -1

	// Not having a social provider is the "default", hence why SocialProviderNone is 0.
	SocialProviderNone SocialProvider = 0

	// Valid social auth providers are numbered starting with 1
	SocialProviderGoogle SocialProvider = 1
)

//go:generate genconstant SocialProvider

// Validate implements Validateable
func (s SocialProvider) Validate() error {
	// None and Unsupported are both "valid" for different scenarios (see documentation on constants)
	if s == SocialProviderGoogle || s == SocialProviderNone || s == SocialProviderUnsupported {
		return nil
	}
	return ucerr.Errorf("invalid SocialProvider: %s", s.String())
}

// UsernamePasswordLoginRequest specifies the IDP request to login with username & password.
type UsernamePasswordLoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// LoginStatus indicates whether a login attempt succeeded, failed, or requires additional validation (e.g. MFA)
type LoginStatus string

// LoginStatus constants
const (
	LoginStatusSuccess     LoginStatus = "success"
	LoginStatusMFARequired LoginStatus = "mfa_required"
)

// LoginResponse is the full response returned from an IDP login API
type LoginResponse struct {
	Status LoginStatus `json:"status"`

	// UserID is set iff Status == LoginStatusSuccess (TODO: maybe also LoginStatusMFARequired?)
	UserID uuid.UUID `json:"user_id"`

	// MFAToken is set iff Status == LoginStatusMFARequired
	MFAToken string `json:"mfa_token,omitempty"`
}

// UpdateUsernamePasswordRequest is used to keep the follower IDP(s) in sync
type UpdateUsernamePasswordRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// UpdateUsernamePasswordResponse confirms an update succeeded (or not)
type UpdateUsernamePasswordResponse struct {
	Success bool `json:"success"`
}

// MFALoginRequest allows the client to submit an MFA code
type MFALoginRequest struct {
	MFARequestID uuid.UUID
	MFACode      string
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

// Login supports username/password login to the UC IDP
func (c *Client) Login(ctx context.Context, username, password string) (*LoginResponse, error) {
	if err := c.client.RefreshBearerToken(); err != nil {
		return nil, ucerr.Wrap(err)
	}

	lr := UsernamePasswordLoginRequest{
		Username: username,
		Password: password,
	}
	var response LoginResponse

	if err := c.client.Post(ctx, "/authn/uplogin", lr, &response, jsonclient.ParseOAuthError()); err != nil {
		return nil, ucerr.Wrap(err)
	}

	return &response, nil
}

// LoginWithMFA sends the MFA code response
func (c *Client) LoginWithMFA(ctx context.Context, sessionID, code string) (*LoginResponse, error) {
	if err := c.client.RefreshBearerToken(); err != nil {
		return nil, ucerr.Wrap(err)
	}

	id, err := uuid.FromString(sessionID)
	if err != nil {
		return nil, ucerr.Wrap(err)
	}

	body := MFALoginRequest{id, code}
	var response LoginResponse
	if err := c.client.Post(ctx, "/authn/mfaresponse", body, &response, jsonclient.ParseOAuthError()); err != nil {
		return nil, ucerr.Wrap(err)
	}

	return &response, nil
}

// UpdateUsernamePassword updates the stored password for a user for follower-sync purposes
func (c *Client) UpdateUsernamePassword(ctx context.Context, username, password string) error {
	if err := c.client.RefreshBearerToken(); err != nil {
		return ucerr.Wrap(err)
	}

	lr := UpdateUsernamePasswordRequest{
		Username: username,
		Password: password,
	}

	var response UpdateUsernamePasswordResponse
	if err := c.client.Post(ctx, "/authn/upupdate", lr, &response); err != nil {
		return ucerr.Wrap(err)
	}

	if !response.Success {
		return ucerr.New("update username/password failure")
	}

	return nil
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
	SocialProvider SocialProvider `json:"social_provider,omitempty"`
	OIDCSubject    string         `json:"oidc_subject,omitempty"`
}

// NewPasswordAuthn creates a new UserAuthn for username + password.
func NewPasswordAuthn(username, password string) UserAuthn {
	return UserAuthn{
		AuthnType: AuthnTypePassword,
		Username:  username,
		Password:  password,
	}
}

// NewSocialAuthn creates a new UserAuthn for social / OIDC login.
func NewSocialAuthn(provider SocialProvider, oidcSubject string) UserAuthn {
	return UserAuthn{
		AuthnType:      AuthnTypeSocial,
		SocialProvider: provider,
		OIDCSubject:    oidcSubject,
	}
}

// UserProfile is a collection of per-user properties stored in the DB as JSON since
// they are likely to be sparse and change more frequently.
// Follow conventions of https://openid.net/specs/openid-connect-core-1_0.html#StandardClaims for
// all standard fields.
type UserProfile struct {
	Email         string `json:"email" validate:"notempty"`
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

func (u UserProfile) extraValidate() error {
	if err := emailutil.Validate(u.Email); err != nil {
		return ucerr.Wrap(err)
	}
	return nil
}

// CreateUserRequest creates a user on the IDP
type CreateUserRequest struct {
	UserProfile `json:"profile"`

	RequireMFA bool `json:"require_mfa"`

	UserExtendedProfile userstore.Record `json:"profile_ext"`

	UserAuthn
}

// UserResponse is the response body for methods which return user data.
type UserResponse struct {
	ID        uuid.UUID `json:"id"`
	UpdatedAt int64     `json:"updated_at"` // seconds since the Unix Epoch (UTC)

	UserProfile `json:"profile"`

	RequireMFA bool `json:"require_mfa"`

	UserExtendedProfile userstore.Record `json:"profile_ext"`

	Authns []UserAuthn `json:"authns"`
}

// CreateUserWithPassword creates a user on the IDP
func (c *Client) CreateUserWithPassword(ctx context.Context, username, password string, profile UserProfile) (uuid.UUID, error) {
	if err := c.client.RefreshBearerToken(); err != nil {
		return uuid.Nil, ucerr.Wrap(err)
	}

	if err := profile.Validate(); err != nil {
		return uuid.Nil, ucerr.Wrap(err)
	}

	req := CreateUserRequest{
		UserProfile: profile,
		RequireMFA:  false,
		UserAuthn:   NewPasswordAuthn(username, password),
	}

	var res UserResponse

	if err := c.client.Post(ctx, "/authn/users", req, &res); err != nil {
		return uuid.Nil, ucerr.Wrap(err)
	}

	return res.ID, nil
}

// CreateUserWithSocial creates a user on the IDP
func (c *Client) CreateUserWithSocial(ctx context.Context, provider SocialProvider, subject string, profile UserProfile) (uuid.UUID, error) {
	if err := c.client.RefreshBearerToken(); err != nil {
		return uuid.Nil, ucerr.Wrap(err)
	}

	req := CreateUserRequest{
		UserProfile: profile,
		RequireMFA:  false,
		UserAuthn:   NewSocialAuthn(provider, subject),
	}

	var res UserResponse

	if err := c.client.Post(ctx, "/authn/users", req, &res); err != nil {
		return uuid.Nil, ucerr.Wrap(err)
	}

	return res.ID, nil
}

// ListUsers lists all users
// TODO: pagination desperately needed
func (c *Client) ListUsers(ctx context.Context) ([]UserResponse, error) {
	if err := c.client.RefreshBearerToken(); err != nil {
		return nil, ucerr.Wrap(err)
	}

	var res []UserResponse

	requestURL := url.URL{Path: "/authn/users"}

	if err := c.client.Get(ctx, requestURL.String(), &res); err != nil {
		return nil, ucerr.Wrap(err)
	}

	return res, nil
}

// GetUser gets a user by ID
func (c *Client) GetUser(ctx context.Context, id uuid.UUID) (*UserResponse, error) {
	if err := c.client.RefreshBearerToken(); err != nil {
		return nil, ucerr.Wrap(err)
	}

	var res UserResponse

	requestURL := url.URL{
		Path: fmt.Sprintf("/authn/users/%s", id),
	}

	if err := c.client.Get(ctx, requestURL.String(), &res); err != nil {
		return nil, ucerr.Wrap(err)
	}

	return &res, nil
}

// GetUserForSocial gets a user by social provider / ID
func (c *Client) GetUserForSocial(ctx context.Context, provider SocialProvider, oidcSubject string) (*UserResponse, error) {
	if err := c.client.RefreshBearerToken(); err != nil {
		return nil, ucerr.Wrap(err)
	}

	var res []UserResponse

	prov, err := provider.MarshalText()
	if err != nil {
		return nil, ucerr.Wrap(err)
	}
	reqURL := url.URL{
		Path: "/authn/users",
		RawQuery: url.Values{
			"authn_type": []string{string(AuthnTypeSocial)},
			"provider":   []string{string(prov)},
			"subject":    []string{oidcSubject},
		}.Encode(),
	}
	if err := c.client.Get(ctx, reqURL.String(), &res); err != nil {
		return nil, ucerr.Wrap(err)
	}

	if len(res) != 1 {
		return nil, ucerr.Errorf("unexpected number of results (%d)", len(res))
	}

	return &res[0], nil
}

// ListUsersForEmail gets all user accounts associated with an email and authn type
func (c *Client) ListUsersForEmail(ctx context.Context, email string, authnType AuthnType) ([]UserResponse, error) {
	if err := c.client.RefreshBearerToken(); err != nil {
		return nil, ucerr.Wrap(err)
	}

	var res []UserResponse

	requestURL := url.URL{
		Path: "/authn/users",
		RawQuery: url.Values{
			"email":      []string{email},
			"authn_type": []string{string(authnType)},
		}.Encode(),
	}

	if err := c.client.Get(ctx, requestURL.String(), &res); err != nil {
		return nil, ucerr.Wrap(err)
	}

	return res, nil
}

// UpdateUserRequest optionally updates some or all mutable fields of a user struct.
// Pointers are used to distinguish between unset vs. set to default value (false, "", etc).
// TODO: should we allow changing Email? That's a more complex one as there are more implications to
// changing email that may affect AuthNs and security (e.g. account hijacking, unverified emails, etc).
type UpdateUserRequest struct {
	EmailVerified *bool   `json:"email_verified,omitempty"`
	Name          *string `json:"name,omitempty"`
	Nickname      *string `json:"nickname,omitempty"`
	Picture       *string `json:"picture,omitempty"`

	// TODO: add MFA factors
	RequireMFA *bool `json:"require_mfa,omitempty"`

	// Only fields set in the underlying map will be updated
	UserExtendedProfile userstore.Record `json:"profile_ext"`
}

// UpdateUser updates user profile data for a given user ID
func (c *Client) UpdateUser(ctx context.Context, id uuid.UUID, req UpdateUserRequest) (*UserResponse, error) {
	if err := c.client.RefreshBearerToken(); err != nil {
		return nil, ucerr.Wrap(err)
	}

	requestURL := url.URL{
		Path: fmt.Sprintf("/authn/users/%s", id),
	}

	var resp UserResponse

	if err := c.client.Put(ctx, requestURL.String(), &req, &resp); err != nil {
		return nil, ucerr.Wrap(err)
	}

	return &resp, nil
}

// DeleteUser deletes a user by ID
func (c *Client) DeleteUser(ctx context.Context, id uuid.UUID) error {
	if err := c.client.RefreshBearerToken(); err != nil {
		return ucerr.Wrap(err)
	}

	requestURL := url.URL{
		Path: fmt.Sprintf("/authn/users/%s", id),
	}

	return ucerr.Wrap(c.client.Delete(ctx, requestURL.String()))
}
