package logtransports

import (
	"userclouds.com/infra/namespace/service"
	"userclouds.com/infra/ucjwt"
	"userclouds.com/infra/uclog"
)

const serverURL = "https://logserver.userclouds.com"

// InitLoggingSDK sets up logging transport for SDK
func InitLoggingSDK(auth *ucjwt.Config, rawLogs bool) {
	var transports []uclog.Transport = make([]uclog.Transport, 0, 1)

	config := &Config{NoRequestIDs: true,
		LogServerTransportC: LogServerTransportConfig{
			TransportConfig: uclog.TransportConfig{
				Required:    false,
				MaxLogLevel: 5,
			},
			Service:       string(service.SDK),
			TenantID:      auth.TenantID,
			LogServiceURL: serverURL,
			SendRawData:   rawLogs,
		},
	}
	transports = append(transports, newTransportBackgroundIOWrapper(newLogServerTransport(&config.LogServerTransportC, auth, service.SDK)))

	uclog.InitForService(service.SDK, transports, nil)
}
