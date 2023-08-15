package ucjwt

import (
	"github.com/gofrs/uuid"
)

// Config represents config for Console Authentication & Authorization.
type Config struct {
	ClientID     string    `yaml:"client_id" validate:"notempty"`
	ClientSecret string    `yaml:"client_secret" validate:"notempty"` // TODO: convert to secret.String
	TenantURL    string    `yaml:"tenant_url" validate:"notempty"`
	TenantID     uuid.UUID `yaml:"tenant_id" validate:"notnil"`
	CompanyID    uuid.UUID `yaml:"company_id" validate:"notnil"`
}

//go:generate genvalidate Config
