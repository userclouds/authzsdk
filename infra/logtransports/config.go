package logtransports

import (
	"gopkg.in/yaml.v3"

	"userclouds.com/infra/namespace/service"
	"userclouds.com/infra/ucerr"
	"userclouds.com/infra/ucjwt"
	"userclouds.com/infra/uclog"
)

// Config defines overall logging configuration
type Config struct {
	Transports   TransportConfigs `yaml:"transports" json:"transports"`
	NoRequestIDs bool             `yaml:"no_request_ids" json:"no_request_ids"`
}

//go:generate genvalidate Config

// TransportConfigs is an alias for an array of TransportConfig so we can handle polymorphic config unmarshalling
type TransportConfigs []TransportConfig

// UnmarshalYAML implements yaml.Unmarshaler
func (t TransportConfigs) UnmarshalYAML(value *yaml.Node) error {
	var c []intermediateConfig
	if err := value.Decode(&c); err != nil {
		return ucerr.Wrap(err)
	}
	t = make([]TransportConfig, len(c))
	for i, v := range c {
		t[i] = v.c
	}
	return nil
}

// intermediateConfig is a place to unmarshal to before we know the type of transport
type intermediateConfig struct {
	c TransportConfig
}

// UnmarshalYAML implements yaml.Unmarshaler
func (i *intermediateConfig) UnmarshalYAML(value *yaml.Node) error {
	for _, d := range decoders {
		if c, err := d(value); err == nil {
			i.c = c
			return nil
		}
	}
	return ucerr.New("unknown TransportConfig implementation")
}

// decoders allows different files to register themselves as available decoders/types
// so that we can ship some transports externally and leave others internal without causing
// build issues
var decoders = make(map[TransportType]func(*yaml.Node) (TransportConfig, error))

// registerDecoder centralizes manipulation of `decoders“
func registerDecoder(name TransportType, f func(*yaml.Node) (TransportConfig, error)) {
	decoders[name] = f
}

// TransportConfig defines the interface for a transport config
type TransportConfig interface {
	GetTransport(service.Service, *ucjwt.Config) uclog.Transport
	GetType() TransportType
	Validate() error
}

// TransportType defines the type of transport
type TransportType string
