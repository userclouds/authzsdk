package oidc

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/go-http-utils/headers"

	"userclouds.com/infra/ucerr"
)

// ClientCredentialsTokenSource encapsulates parameters required to issue a Client Credentials OIDC request and return a token
type ClientCredentialsTokenSource struct {
	TokenURL        string   `json:"token_url"`
	ClientID        string   `json:"client_id"`
	ClientSecret    string   `json:"client_secret"`
	CustomAudiences []string `json:"custom_audiences"`
	SubjectJWT      string   `json:"subject_jwt"` // optional, ID Token for a UC user if this access token is being created on their behalf
}

// GetToken issues a request to an OIDC-compliant token endpoint to perform
// the Client Credentials flow in exchange for an access token.
func (ccts ClientCredentialsTokenSource) GetToken() (string, error) {
	query := url.Values{}
	// TODO: move common OIDC values into constants
	query.Add("grant_type", "client_credentials")
	query.Add("client_id", ccts.ClientID)
	query.Add("client_secret", ccts.ClientSecret)
	for _, aud := range ccts.CustomAudiences {
		query.Add("audience", aud)
	}
	if ccts.SubjectJWT != "" {
		query.Add("subject_jwt", ccts.SubjectJWT)
	}
	req, err := http.NewRequest(http.MethodPost, ccts.TokenURL, strings.NewReader(query.Encode()))
	if err != nil {
		return "", ucerr.Wrap(err)
	}
	req.Header.Add(headers.ContentType, "application/x-www-form-urlencoded")
	// TODO: re-use client?
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", ucerr.Wrap(err)
	}
	if resp.StatusCode >= http.StatusBadRequest {
		var oauthe ucerr.OAuthError
		if resp.Header.Get(headers.ContentType) == "application/json" {
			if err := json.NewDecoder(resp.Body).Decode(&oauthe); err != nil {
				return "", ucerr.Wrap(err)
			}

			oauthe.Code = resp.StatusCode
			return "", ucerr.Wrap(oauthe)
		}
		// Handle non-json response
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return "", ucerr.Errorf("unexpected response from token endpoint %v: %v. Failed to read response body: %v", req.URL, resp.Status, err)
		}
		return "", ucerr.Errorf("unexpected response from token endpoint %v: %v: %v", req.URL, resp.Status, string(body))

	}
	var tresp TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tresp); err != nil {
		return "", ucerr.Wrap(err)
	}
	return tresp.AccessToken, nil
}
