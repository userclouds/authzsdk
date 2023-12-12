package logtransports

import (
	"context"
	"os"

	"userclouds.com/infra/namespace/service"
	"userclouds.com/infra/ucjwt"
	"userclouds.com/infra/uclog"
)

// InitLoggerAndTransportsForService sets up logging transports for long running serving
func InitLoggerAndTransportsForService(config *Config, auth *ucjwt.Config, name service.Service) {
	transports := initConfigInfoInTransports(name, config, auth)
	fetcher := initConfigInfoInFetcher(config, auth)
	uclog.InitForService(name, transports, fetcher)
}

// InitLoggerAndTransportsForTools configures logging to the screen and file if desired for a tool
func InitLoggerAndTransportsForTools(ctx context.Context, lScreen uclog.LogLevel, lFile uclog.LogLevel, toolName string, opts ...ToolLogOption) {
	// Get the optional parameters if any
	to := &ToolLogConfig{
		filename: "",
		prefix:   DefaultPrefixVal,
	}
	for _, v := range opts {
		v.apply(to)
	}

	loggerConfig := Config{}
	loggerConfig.Transports = []TransportConfig{&GoTransportConfig{
		Type: TransportTypeGo,
		TransportConfig: uclog.TransportConfig{
			Required:    true,
			MaxLogLevel: lScreen,
		},
		PrefixFlag:    NoPrefixVal,
		SupportsColor: to.supportsColor,
		NoRequestIDs:  true,
	},
	}

	if lFile != uclog.LogLevelNonMessage {
		if to.filename == "" {
			// Generate default name using name of the tool as a prefix
			f, err := os.CreateTemp("/tmp/", toolName+".*")
			if err != nil {
				// TODO: not sure what else to do here yet?
				return
			}
			to.filename = f.Name()
			f.Close()
		}
		loggerConfig.Transports = append(loggerConfig.Transports,
			&FileTransportConfig{
				TransportConfig: uclog.TransportConfig{
					Required:    true,
					MaxLogLevel: lFile,
				},
				Filename:     to.filename,
				Append:       true,
				PrefixFlag:   to.prefix,
				NoRequestIDs: true,
			})
	}

	loggerConfig.NoRequestIDs = true
	// TODO: this is a bit of a hack to make service typing work (in fact we never configure service-required
	// transports for tools) but when I get to refactoring all of this it'll get fixed
	transports := initConfigInfoInTransports(service.Service(toolName), &loggerConfig, nil)
	uclog.InitForTools(ctx, string(toolName), to.filename, transports)
}

// InitTransportsForTests returns an array of setup transports to use in testing
func InitTransportsForTests(config *Config, auth *ucjwt.Config, name service.Service) []uclog.Transport {
	return initConfigInfoInTransports(name, config, auth)
}

// initConfigInfoInFetcher passes the config data to each fetcher (just one for now)
// TODO (sgarrity 8/23): this currently lives at the wrong level of abstraction (it's part
// of generic logtransports, but it's really an implementation detail of a single specific transport,
// namely the logserver transport), but can be refactored later
func initConfigInfoInFetcher(config *Config, auth *ucjwt.Config) uclog.EventMetadataFetcher {
	for _, tr := range config.Transports {
		if tr.GetType() != TransportTypeServer {
			continue
		}
		lstc := tr.(*LogServerTransportConfig)
		return newLogServerMapFetcher(lstc.LogServiceURL, lstc.Service)
	}
	// If we didn't find a logserver transport, return a no-op fetcher (see TODO above)
	return newLogServerMapFetcher("", "")
}
