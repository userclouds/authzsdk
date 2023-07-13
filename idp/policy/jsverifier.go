package policy

import (
	"userclouds.com/authz"
	"userclouds.com/infra/ucerr"
)

// JSVerifier specifies a minimal interface to allow verification of JS
type JSVerifier interface {
	RunScript(s string, o string, authzClient *authz.Client) (string, error)
}

var jsverifier JSVerifier

// RegisterJSVerifier registers a verifier for JS
func RegisterJSVerifier(v JSVerifier) {
	jsverifier = v
}

func validateJSHelper(s string, o string) error {
	if jsverifier != nil {
		if _, err := jsverifier.RunScript(s, o, nil); err != nil {
			return ucerr.Wrap(err)
		}
	}
	return nil
}
