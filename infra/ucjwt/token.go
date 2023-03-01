package ucjwt

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/gofrs/uuid"
	"github.com/golang-jwt/jwt"

	"userclouds.com/infra/oidc"
	"userclouds.com/infra/ucerr"
)

const defaultTokenExpiry int64 = 86400

// CreateToken creates a new JWT
func CreateToken(ctx context.Context, privateKey *rsa.PrivateKey, keyID string, tokenID uuid.UUID, claims oidc.TokenClaims, jwtIssuerURL string) (string, error) {
	// Augment claims with special fields.
	claims.IssuedAt = time.Now().Unix()
	claims.Issuer = jwtIssuerURL

	if claims.ExpiresAt == 0 {
		claims.ExpiresAt = claims.IssuedAt + defaultTokenExpiry
	}

	// Put unique token ID in claims so we can track tokens back to any context around their issuance.
	// As a side effect, we also get unique tokens which is currently required since we want to be able to look
	// up each token issuance uniquely by the token.
	claims.ID = tokenID.String()

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = keyID
	signedToken, err := token.SignedString(privateKey)
	if err != nil {
		return "", ucerr.Wrap(err)
	}
	return signedToken, nil
}

// ParseClaimsUnverified extracts the claims from a token without validating
// its signature or anything else.
func ParseClaimsUnverified(token string) (*oidc.TokenClaims, error) {
	parts := strings.Split(token, ".")
	if len(parts) < 2 {
		return nil, ucerr.Errorf("malformed jwt, expected 3 parts got %d", len(parts))
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, ucerr.Errorf("malformed jwt payload: %v", err)
	}
	var claims oidc.TokenClaims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, ucerr.Errorf("failed to unmarshal claims: %v", err)
	}

	return &claims, nil
}

// ParseClaimsVerified extracts the claims from a token and verifies the signature, expiration, etc.
func ParseClaimsVerified(token string, key *rsa.PublicKey) (*oidc.TokenClaims, error) {
	var claims oidc.TokenClaims
	_, err := jwt.ParseWithClaims(token, &claims, func(t *jwt.Token) (interface{}, error) {
		return key, nil
	})
	return &claims, ucerr.Wrap(err)
}

// IsExpired returns `true, nil` if the supplied JWT has valid claims and is expired,
// `false, nil` if it has valid claims and is unexpired, and `true, err` if the claims
// aren't parseable.
// NOTE: It does NOT validate the token's signature!
func IsExpired(jwt string) (bool, error) {
	claims, err := ParseClaimsUnverified(jwt)
	if err != nil {
		return true, ucerr.Wrap(err)
	}
	if time.Unix(claims.ExpiresAt, 0).After(time.Now().UTC()) {
		return false, nil
	}
	return true, nil
}

// ExtractBearerToken extracts a bearer token from an HTTP request or returns an error
// if none is found or if it's malformed.
// NOTE: this doesn't enforce that it's a JWT, much less a valid one.
func ExtractBearerToken(h *http.Header) (string, error) {
	bearerToken := h.Get("Authorization")
	if bearerToken == "" {
		return "", ucerr.New("authorization header required")
	}

	const bearerPrefix = "Bearer "
	if !strings.HasPrefix(bearerToken, bearerPrefix) {
		return "", ucerr.New("authorization header requires bearer token")
	}

	bearerToken = strings.TrimPrefix(bearerToken, bearerPrefix)
	return bearerToken, nil
}
