package oidc

import (
	"github.com/golang-jwt/jwt"

	"userclouds.com/infra/ucerr"
)

// StandardClaims is forked from golang-jwt/jwt.StandardClaims,
// except Audience is an array here per the actual spec:
//   In the general case, the "aud" value is an array of case-sensitive strings, each containing
//   a StringOrURI value.  In the special case when the JWT has one audience, the "aud" value MAY
//   be a single case-sensitive string containing a StringOrURI value.  The interpretation of
//   audience values is generally application specific. Use of this claim is OPTIONAL.
// https://tools.ietf.org/html/rfc7519#section-4.1
type StandardClaims struct {
	Audience  []string `json:"aud,omitempty"`
	ExpiresAt int64    `json:"exp,omitempty"`
	ID        string   `json:"jti,omitempty"`
	IssuedAt  int64    `json:"iat,omitempty"`
	Issuer    string   `json:"iss,omitempty"`
	NotBefore int64    `json:"nbf,omitempty"`
	Subject   string   `json:"sub,omitempty"`
}

// Valid implements jwt.Claims interface
func (c StandardClaims) Valid() error {
	// Use the time validation logic from jwt.StandardClaims
	jwtClaims := jwt.StandardClaims{
		ExpiresAt: c.ExpiresAt,
		IssuedAt:  c.IssuedAt,
		NotBefore: c.NotBefore,
	}
	return ucerr.Wrap(jwtClaims.Valid())
}

// TokenClaims represents the claims made by a token, and is also used by the UserInfo
// endpoint to return standard OIDC user claims.
type TokenClaims struct {
	Name          string `json:"name,omitempty"`
	Nickname      string `json:"nickname,omitempty"`
	Email         string `json:"email,omitempty"`
	EmailVerified bool   `json:"email_verified"`
	Picture       string `json:"picture,omitempty"`
	Nonce         string `json:"nonce,omitempty"`
	UpdatedAt     int64  `json:"updated_at,omitempty"` // NOTE: Auth0 treats this as a string, but OIDC says this is seconds since the Unix Epoch
	StandardClaims

	// TODO: not sure if this is the right place for this, but didn't come up with a clever interface
	// to use with GeneratePlexUserToken etc yet. With omitempty, it shouldn't affect anything else when unused
	ImpersonatedBy string `json:"impersonated_by,omitempty"`
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
