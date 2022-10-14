package emailutil

import (
	"net/mail"

	"userclouds.com/infra/ucerr"
)

// Address represents an email address
type Address string

// Validate implements the Validatable interface for Address
func (a Address) Validate() error {
	if _, err := a.Parse(); err != nil {
		return ucerr.Wrap(err)
	}
	return nil
}

// Parse will parse an address according to the RFC 5322 standard,
// applying additional restrictions we have chosen to enforce
func (a Address) Parse() (*mail.Address, error) {
	parsedAddress, err := mail.ParseAddress(string(a))
	if err != nil {
		return nil, ucerr.Errorf("invalid email: %w", err)
	}
	if parsedAddress.Name != "" {
		return nil, ucerr.New("invalid email: no name allowed")
	}

	return parsedAddress, nil
}

// CombineAddress will generate a valid address string from a name part
// and an address part, ensuring that the address part was valid and did
// not already have an associated name, and that the resulting combined
// address parses correctly
func CombineAddress(name string, address string) (string, error) {
	parsedAddress, err := mail.ParseAddress(address)
	if err != nil {
		return "", ucerr.Wrap(err)
	}
	if parsedAddress.Name != "" {
		return "", ucerr.New("address had an associated name")
	}
	parsedAddress.Name = name
	combinedAddress := parsedAddress.String()
	if _, err := mail.ParseAddress(combinedAddress); err != nil {
		return "", ucerr.Wrap(err)
	}

	return combinedAddress, nil
}
