package logtransports

import (
	"github.com/gofrs/uuid"
	"userclouds.com/infra/namespace/service"
	"userclouds.com/infra/ucjwt"
	"userclouds.com/infra/uclog"
)

// LogServerTransportConfig defines the configuration for transport sending events to our servers
type LogServerTransportConfig struct {
	uclog.TransportConfig `yaml:"transportconfig" json:"transportconfig"`
	TenantID              uuid.UUID `yaml:"tenant_id" json:"tenant_id"`
	LogServiceURL         string    `yaml:"log_service_url" json:"log_service_url"`
	Service               string    `yaml:"service" json:"service"`
	SendRawData           bool      `yaml:"send_raw_data" json:"send_raw_data"`
}

// Config defines overall logging configuration
type Config struct {
	LogServerTransportC LogServerTransportConfig `yaml:"serverlogger,omitempty" json:"serverlogger"`
	NoRequestIDs        bool                     `yaml:"no_request_ids" json:"no_request_ids"`
}

// InitLoggerAndTransportsForSDK sets up logging transports for SDK
func InitLoggerAndTransportsForSDK(config *Config, auth *ucjwt.Config, name service.Service) {
	transports := initConfigInfoInTransports(name, config, auth)

	uclog.InitForService(name, transports, nil)
}

// initConfigInfoInTransports passes the config data to each transport
func initConfigInfoInTransports(name service.Service, config *Config, auth *ucjwt.Config) []uclog.Transport {

	var transports []uclog.Transport = make([]uclog.Transport, 0, 1)

	transports = append(transports, newTransportBackgroundIOWrapper(newLogServerTransport(&config.LogServerTransportC, auth, name)))

	return transports
}
