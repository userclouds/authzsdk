package socialprovider

import (
	"userclouds.com/infra/ucerr"
)

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
	SocialProviderGoogle   SocialProvider = 1
	SocialProviderFacebook SocialProvider = 2
	SocialProviderLinkedIn SocialProvider = 3
	SocialProviderMsft     SocialProvider = 4
)

// ValidSocialProviders returns a slice containing all valid social auth providers
func ValidSocialProviders() []SocialProvider {
	// returning these in alphabetical order
	return []SocialProvider{SocialProviderFacebook, SocialProviderGoogle, SocialProviderLinkedIn, SocialProviderMsft}
}

//go:generate genconstant SocialProvider

// Validate implements Validateable
func (s SocialProvider) Validate() error {
	if _, err := s.MarshalText(); err != nil {
		return ucerr.Wrap(err)
	}

	return nil
}

// IsSupported returns true if SocialProvider is recognized and not SocialProviderUnsupported or SocialProviderNone
func (s SocialProvider) IsSupported() bool {
	if err := s.Validate(); err != nil {
		return false
	}

	return s != SocialProviderUnsupported && s != SocialProviderNone
}
