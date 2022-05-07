package emailutil

import (
	"net/mail"
	"unicode"

	"userclouds.com/infra/ucerr"
)

// Validate returns an error if the provided email string is invalid, or nil if it's valid.
func Validate(email string) error {
	if _, err := mail.ParseAddress(email); err != nil {
		return ucerr.Errorf("invalid email: %w", err)
	}

	// Golang's email parser allows for things like
	// "My Name <foo@contoso.com>" which we don't allow, so also disallow whitespace.
	for _, v := range email {
		// Test each character to see if it is whitespace.
		if unicode.IsSpace(v) {
			return ucerr.New("invalid email: no whitespace allowed")
		}
	}

	return nil
}
