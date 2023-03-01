// NOTE: automatically generated file -- DO NOT EDIT

package socialprovider

import "userclouds.com/infra/ucerr"

// MarshalText implements encoding.TextMarshaler (for JSON)
func (t SocialProvider) MarshalText() ([]byte, error) {
	switch t {
	case SocialProviderFacebook:
		return []byte("facebook"), nil
	case SocialProviderGoogle:
		return []byte("google"), nil
	case SocialProviderLinkedIn:
		return []byte("linkedin"), nil
	case SocialProviderMsft:
		return []byte("msft"), nil
	case SocialProviderNone:
		return []byte("none"), nil
	case SocialProviderUnsupported:
		return []byte("unsupported"), nil
	default:
		return nil, ucerr.Errorf("unknown value %d", t)
	}
}

// UnmarshalText implements encoding.TextMarshaler (for JSON)
func (t *SocialProvider) UnmarshalText(b []byte) error {
	s := string(b)
	switch s {
	case "facebook":
		*t = SocialProviderFacebook
	case "google":
		*t = SocialProviderGoogle
	case "linkedin":
		*t = SocialProviderLinkedIn
	case "msft":
		*t = SocialProviderMsft
	case "none":
		*t = SocialProviderNone
	case "unsupported":
		*t = SocialProviderUnsupported
	default:
		return ucerr.Errorf("unknown value %s", s)
	}
	return nil
}

// Validate implements Validateable
func (t *SocialProvider) Validate() error {
	switch *t {
	case SocialProviderFacebook:
		return nil
	case SocialProviderGoogle:
		return nil
	case SocialProviderLinkedIn:
		return nil
	case SocialProviderMsft:
		return nil
	case SocialProviderNone:
		return nil
	case SocialProviderUnsupported:
		return nil
	default:
		return ucerr.Errorf("unknown SocialProvider value %d", *t)
	}
}

// AllSocialProviders is a slice of all SocialProvider values
var AllSocialProviders = []SocialProvider{
	SocialProviderFacebook,
	SocialProviderGoogle,
	SocialProviderLinkedIn,
	SocialProviderMsft,
	SocialProviderNone,
	SocialProviderUnsupported,
}

// just here for easier debugging
func (t SocialProvider) String() string {
	bs, err := t.MarshalText()
	if err != nil {
		return err.Error()
	}
	return string(bs)
}
