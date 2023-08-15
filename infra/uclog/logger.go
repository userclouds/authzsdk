package uclog

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"

	"github.com/gofrs/uuid"

	"userclouds.com/infra/namespace/service"
	"userclouds.com/infra/request"
	"userclouds.com/infra/ucerr"
)

// loggerStatus represents current state of the logger
type loggerStatus int

// Possible states of the logger interface
const (
	loggerNotInitialized   loggerStatus = iota // no initialization has been performed
	loggerPreInitialized                       // no initialization has been performed but logging messages have been recieved
	loggerToolMode                             // initialized for short lifetime tool
	loggerServiceMode                          // initialized for long time running service
	loggerShuttingDownMode                     // transports are in the process of being closed
)

// Data for the logging layer
type loggerData struct {
	loggerConfigMutex    sync.RWMutex
	eventMetadataMutex   sync.RWMutex
	transports           []Transport
	transportConfigs     []TransportConfig
	loggerState          loggerStatus
	eventMetadata        map[uuid.UUID]EventMetadataMap
	eventMetadataFetcher EventMetadataFetcher
	registeredHandlers   []string
	noRequestIDs         bool
	serviceName          string
}

// Global instance of the logger shared for the process
var loggerInst = loggerData{loggerState: loggerNotInitialized}

// PreInit sets up logging to the screen before config file was read
func PreInit(transports []Transport) {
	initialize("", loggerPreInitialized, true, transports, nil)
}

// InitForService sets up logging transports for long running serving
func InitForService(name service.Service, transports []Transport, fetcher EventMetadataFetcher) {
	initialize(string(name), loggerServiceMode, false, transports, fetcher)
}

// InitForTools configures logging to the screen and file if desired for a tool
func InitForTools(ctx context.Context, toolName string, fileLogName string, transports []Transport) {
	initialize(toolName, loggerToolMode, true, transports, nil)
	// Log basic debugging information that is useful across all tools
	Debugf(ctx, "------------------------------------------------------") // Log a visual separator to make break out multi run logs
	Debugf(ctx, "Command Line: \"%v\" Logfile - %s", os.Args, fileLogName)
}

// called with logging config to "really" init the logger
func initialize(name string, l loggerStatus, noRequestIDs bool, transports []Transport, fetcher EventMetadataFetcher) {
	loggerInst.loggerConfigMutex.Lock()
	defer loggerInst.loggerConfigMutex.Unlock()

	// Check for unexpected state transitions - we may allow this in the future but for now fatal
	if (loggerInst.loggerState == loggerServiceMode || loggerInst.loggerState == loggerToolMode) && l != loggerInst.loggerState {
		log.Fatalf("Failed to initialize logger - unexpected state change from %v to %v", loggerInst.loggerState, l)
	}
	loggerInst.loggerState = l
	loggerInst.serviceName = name
	status.initStatus()
	loggerInst.eventMetadataFetcher = fetcher
	loggerInst.eventMetadata = make(map[uuid.UUID]EventMetadataMap)

	if fetcher != nil {
		if err := fetcher.Init(updateEventMetadata); err != nil {
			log.Fatal("Failed to initialize metadata fetcher")
		}
	}
	loggerInst.transports = []Transport{}
	loggerInst.transportConfigs = []TransportConfig{}

	// Initialize the transports and store their post initialization state
	for _, t := range transports {
		c, err := t.Init()
		if err != nil && c.Required {
			log.Fatal("Failed to initialize required logger", err, t)
		}
		// Only keep transports that were able to properly initialize
		if err == nil {
			loggerInst.transports = append(loggerInst.transports, t)
			loggerInst.transportConfigs = append(loggerInst.transportConfigs, *c)
		}
	}
	loggerInst.noRequestIDs = noRequestIDs
}

// This callback allows transport to tell logger which events they support and how to handle them
func updateEventMetadata(updatedMap *EventMetadataMap, tenantID uuid.UUID) error {
	loggerInst.loggerConfigMutex.Lock()
	loggerInst.eventMetadata[tenantID] = *updatedMap
	loggerInst.loggerConfigMutex.Unlock()

	// Take a lock to ensure that there is no conflict with other handlers being added during validation
	// In  practice this doesn't happen due to registered handlers being added during initialization
	// and the event map being fetched at least a second later
	loggerInst.loggerConfigMutex.RLock()
	defer loggerInst.loggerConfigMutex.RUnlock()

	ctx := context.Background()

	if getEventInfoByName(EventNameNone, 0, tenantID).Code != EventCodeNone ||
		getEventInfoByName(EventNameUnknown, 0, tenantID).Code != EventCodeUnknown {
		Errorf(ctx, "Recieved invalid map (either None or Unknown missing, map length %d)", len(loggerInst.eventMetadata))
	}

	validateHandlerMap(ctx)

	return nil
}

// GetStats returns the stats for each of the transports
func GetStats() []LogTransportStats {
	// Take a reader lock to prevent potential execution against bad configuration if GetStats is
	// called during initialize(...)
	loggerInst.loggerConfigMutex.RLock()
	defer loggerInst.loggerConfigMutex.RUnlock()

	logStats := []LogTransportStats{}

	// Only read stats if the logger is fully initilized and is not in process of shutting down
	if loggerInst.loggerState == loggerServiceMode || loggerInst.loggerState == loggerToolMode {
		for i := range loggerInst.transports {
			logStats = append(logStats, loggerInst.transports[i].GetStats())
		}
	}
	return logStats
}

// AddTransport adds another transport to the logger
func AddTransport(t Transport) error {
	loggerInst.loggerConfigMutex.Lock()
	defer loggerInst.loggerConfigMutex.Unlock()

	// Check if we are in a state that allows for addition of a logger
	if loggerInst.loggerState == loggerNotInitialized || loggerInst.loggerState == loggerShuttingDownMode {
		return ucerr.New("Logger is not in a valid state for addition of a transport")
	}

	// Initialize the transports and add it
	c, err := t.Init()
	if err != nil {
		return ucerr.New("Transport failed to initialize")
	}
	loggerInst.transports = append(loggerInst.transports, t)
	loggerInst.transportConfigs = append(loggerInst.transportConfigs, *c)

	return nil
}

// RemoveTransport removes named transport if it is active
func RemoveTransport(name string) error {
	loggerInst.loggerConfigMutex.Lock()
	defer loggerInst.loggerConfigMutex.Unlock()

	// Check if we are in a state that allows for addition of a logger
	if loggerInst.loggerState == loggerNotInitialized || loggerInst.loggerState == loggerShuttingDownMode {
		return ucerr.New("Logger is not in a valid state for removal of a transport")
	}

	// Try to find transport by name
	var t Transport
	for i := range loggerInst.transports {
		if loggerInst.transports[i].GetName() == name {
			t = loggerInst.transports[i]
			loggerInst.transports = append(loggerInst.transports[:i], loggerInst.transports[i+1:len(loggerInst.transports)]...)
			loggerInst.transportConfigs = append(loggerInst.transportConfigs[:i], loggerInst.transportConfigs[i+1:len(loggerInst.transportConfigs)]...)
			break
		}
	}

	if t == nil {
		return ucerr.New("Transport not found")
	}

	// Close the transport being removed
	t.Close()

	return nil
}

// Close shuts down logging transports
func Close() {
	// Take a writer lock to prevent block Log(..) calls while we are shutting down transports
	loggerInst.loggerConfigMutex.Lock()
	loggerInst.loggerState = loggerShuttingDownMode
	loggerInst.loggerConfigMutex.Unlock()

	for i := range loggerInst.transports {
		loggerInst.transports[i].Close()
	}
}

// Log logs a specific event
func Log(ctx context.Context, event LogEvent) {
	// Take a reader lock to prevent potential execution against bad configuration if Log is
	// called during initialize(...)
	loggerInst.loggerConfigMutex.RLock()
	defer loggerInst.loggerConfigMutex.RUnlock()

	// Check if the logger is in a valid state to process events, otherwise return
	if loggerInst.loggerState != loggerPreInitialized && loggerInst.loggerState != loggerToolMode && loggerInst.loggerState != loggerServiceMode {
		return
	}

	// Check if passed in event is valid, otherwise drop it
	if err := event.Validate(); err != nil {
		Warningf(ctx, "Got invalid logging event %v. It was dropped: %v", event, err)
		return
	}

	// Get the tenant ID from the context if not passed in
	if event.TenantID == uuid.Nil {
		event.TenantID = GetTenantID(ctx)
	}

	// Fetch the event metadata from the map
	eventInfo := getEventInfoByName(event.Name, event.Code, event.TenantID)

	// Don't log this event if it is configured to be ignored
	if eventInfo.Ignore {
		return
	}

	// If the event metadata is not in the map - reset the type as it will be sent as unknown event
	if event.Code != EventCodeNone && eventInfo.Code == EventCodeUnknown {
		event.Code = EventCodeUnknown
	}

	if event.Message != "" && !loggerInst.noRequestIDs {
		id := request.GetRequestID(ctx)
		event.Message = fmt.Sprintf("%v: %s", id, event.Message)
	}

	// if this is a multiline message, tab-indent the following lines to make them slightly easier to read
	// TODO: there might be a better / more clever way to do this?
	event.Message = strings.ReplaceAll(event.Message, "\n", "\n\t")

	// Send the raw event on all transports that are signed up for that event type and
	// messages to transports that log at that log level
	for i := range loggerInst.transports {
		if event.Code == EventCodeNone {
			if event.LogLevel <= loggerInst.transportConfigs[i].MaxLogLevel {
				loggerInst.transports[i].WriteMessage(ctx, event.Message, event.LogLevel)
			}
		} else {
			loggerInst.transports[i].WriteCounter(ctx, event)
		}
	}

	if event.Code != EventCodeNone {
		status.updateStatus(event, eventInfo)
	}
}
