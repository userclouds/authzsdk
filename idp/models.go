package idp

import (
	"userclouds.com/idp/socialprovider"
	"userclouds.com/infra/emailutil"
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
	if a == AuthnTypePassword || a == AuthnTypeSocial || a == AuthnTypeAll || a == "" {
		return nil
	}
	return ucerr.Errorf("invalid AuthnType: %s", string(a))
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

//go:generate gendbjson UserProfile

//go:generate genvalidate UserProfile
