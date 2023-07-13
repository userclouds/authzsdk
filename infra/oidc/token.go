package oidc

import (
	"github.com/golang-jwt/jwt"

	"userclouds.com/infra/ucerr"
)

// StandardClaims is forked from golang-jwt/jwt.StandardClaims,
// except Audience is an array here per the actual spec:
//
//	In the general case, the "aud" value is an array of case-sensitive strings, each containing
//	a StringOrURI value.  In the special case when the JWT has one audience, the "aud" value MAY
//	be a single case-sensitive string containing a StringOrURI value.  The interpretation of
//	audience values is generally application specific. Use of this claim is OPTIONAL.
//
// https://tools.ietf.org/html/rfc7519#section-4.1
//
// AZP is also added here, per the OIDC spec, which is slightly ambiguous:
//
// From 2 https://openid.net/specs/openid-connect-core-1_0.html#IDToken:
// OPTIONAL. Authorized party - the party to which the ID Token was issued.
// If present, it MUST contain the OAuth 2.0 Client ID of this party. This Claim
// is only needed when the ID Token has a single audience value and that audience
// is different than the authorized party. It MAY be included even when the
// authorized party is the same as the sole audience. The azp value is a case
// sensitive string containing a StringOrURI value.
//
// From 3.1.3.7 https://openid.net/specs/openid-connect-core-1_0.html#IDTokenValidation
// 4. If the ID Token contains multiple audiences, the Client SHOULD verify that an azp Claim is present.
// 5. If an azp (authorized party) Claim is present, the Client SHOULD verify that its client_id is the Claim Value.
type StandardClaims struct {
	Audience        []string `json:"aud,omitempty"`
	AuthorizedParty string   `json:"azp,omitempty"`
	ExpiresAt       int64    `json:"exp,omitempty"`
	ID              string   `json:"jti,omitempty"`
	IssuedAt        int64    `json:"iat,omitempty"`
	Issuer          string   `json:"iss,omitempty"`
	NotBefore       int64    `json:"nbf,omitempty"`
	Subject         string   `json:"sub,omitempty"`
}

// Valid implements jwt.Claims interface
func (c StandardClaims) Valid() error {
	// Use the time validation logic from jwt.StandardClaims
	jwtClaims := jwt.StandardClaims{
		ExpiresAt: c.ExpiresAt,
		IssuedAt:  c.IssuedAt,
		Issuer:    c.Issuer,
		NotBefore: c.NotBefore,
	}
	return ucerr.Wrap(jwtClaims.Valid())
}

// TokenClaims represents the claims made by a token, and is also used by the UserInfo
// endpoint to return standard OIDC user claims.
type TokenClaims struct {
	StandardClaims

	Name            string   `json:"name,omitempty"`
	Nickname        string   `json:"nickname,omitempty"`
	Email           string   `json:"email,omitempty"`
	EmailVerified   bool     `json:"email_verified,omitempty"`
	Picture         string   `json:"picture,omitempty"`
	Nonce           string   `json:"nonce,omitempty"`
	UpdatedAt       int64    `json:"updated_at,omitempty"` // NOTE: Auth0 treats this as a string, but OIDC says this is seconds since the Unix Epoch
	RefreshAudience []string `json:"refresh_aud,omitempty"`
	SubjectType     string   `json:"subject_type,omitempty"`
	OrganizationID  string   `json:"organization_id,omitempty"`
	ImpersonatedBy  string   `json:"impersonated_by,omitempty"`
}

// Valid implements jwt.Claims interface
func (t TokenClaims) Valid() error {
	return ucerr.Wrap(t.StandardClaims.Valid())
}

// TokenResponse is an OIDC-compliant response from a token endpoint.
// (either token exchange or resource owner password credential flow).
// See https://datatracker.ietf.org/doc/html/rfc6749#section-5.1.
// ErrorType will be non-empty if error.
type TokenResponse struct {
	AccessToken  string `json:"access_token,omitempty"`
	TokenType    string `json:"token_type,omitempty"`
	RefreshToken string `json:"refresh_token,omitempty"`
	ExpiresIn    int    `json:"expires_in,omitempty"`
	IDToken      string `json:"id_token,omitempty"`

	ErrorType string `json:"error,omitempty"`
	ErrorDesc string `json:"error_description,omitempty"`
}
