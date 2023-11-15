package testlogtransport

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"userclouds.com/infra/uclog"
)

// InitLoggerAndTransportsForTests configures logging to use golang test logging
// TODO: once we simplify log config & init, this can get unified through the main Init() path,
// but I don't want to introduce yet another config fork for this
// TODO: is there a way to do this more automatically than per-test? TestMain doesn't have testing.T or .B
// TODO: the fact that this returns a *bytes.Buffer representing the log is pretty icky
func InitLoggerAndTransportsForTests(t *testing.T) *TransportTest {
	ttc := uclog.TransportConfig{
		Required:    true,
		MaxLogLevel: uclog.LogLevelDebug,
	}
	tt := TransportTest{t: t, config: ttc}
	transports := []uclog.Transport{&tt}
	uclog.PreInit(transports)
	return &tt
}

type testLogRecord struct {
	timestamp time.Time
	event     uclog.LogEvent
}

// TransportTest is a test log transport
type TransportTest struct {
	t           *testing.T
	config      uclog.TransportConfig
	eventMutex  sync.Mutex
	logMutex    sync.Mutex
	Events      []testLogRecord
	LogMessages map[uclog.LogLevel][]string
}

// Init initializes the test transport
func (tt *TransportTest) Init() (*uclog.TransportConfig, error) {
	tt.Events = make([]testLogRecord, 0)
	tt.LogMessages = make(map[uclog.LogLevel][]string)
	return &tt.config, nil
}

// WriteMessage write a message
func (tt *TransportTest) WriteMessage(ctx context.Context, message string, level uclog.LogLevel) {
	tt.t.Helper()
	tt.t.Log(message)

	tt.logMutex.Lock()
	tt.LogMessages[level] = append(tt.LogMessages[level], message)
	tt.logMutex.Unlock()
}

// WriteCounter writes an event
func (tt *TransportTest) WriteCounter(ctx context.Context, event uclog.LogEvent) {
	lE := testLogRecord{event: event, timestamp: time.Now().UTC()}
	tt.eventMutex.Lock()
	tt.Events = append(tt.Events, lE)
	tt.eventMutex.Unlock()

	tt.logMutex.Lock()
	tt.LogMessages[event.LogLevel] = append(tt.LogMessages[event.LogLevel], event.Message)
	tt.logMutex.Unlock()
}

// GetEventsLoggedByName checks if a particular event has been logged
func (tt *TransportTest) GetEventsLoggedByName(name string) []uclog.LogEvent {
	tt.eventMutex.Lock()
	eA := make([]uclog.LogEvent, 0)
	for _, e := range tt.Events {
		if e.event.Name == name {
			eA = append(eA, e.event)
		}
	}
	tt.eventMutex.Unlock()
	return eA
}

// GetLogMessagesByLevel returns log messages by level
func (tt *TransportTest) GetLogMessagesByLevel(level uclog.LogLevel) []string {
	tt.logMutex.Lock()
	mA := tt.LogMessages[level]
	tt.logMutex.Unlock()
	return mA
}

// LogsContainString returns whether any of the logged messages contain the given string
func (tt *TransportTest) LogsContainString(s string) bool {
	tt.logMutex.Lock()
	defer tt.logMutex.Unlock()
	for level := range tt.LogMessages {
		for _, m := range tt.LogMessages[level] {
			if strings.Contains(m, s) {
				return true
			}
		}
	}
	return false
}

// ClearEvents clears all logged events
func (tt *TransportTest) ClearEvents() {
	tt.eventMutex.Lock()
	tt.Events = make([]testLogRecord, 0)
	tt.eventMutex.Unlock()
}

// GetStats  returns stats
func (tt *TransportTest) GetStats() uclog.LogTransportStats {
	return uclog.LogTransportStats{Name: tt.GetName(), QueueSize: 0, DroppedEventCount: 0, SentEventCount: 0, FailedAPICallsCount: 3146}
}

// GetName returns transport name
func (tt *TransportTest) GetName() string {
	return "TestTransport"
}

// Flush does nothing
func (tt *TransportTest) Flush() error {
	return nil
}

// Close does nothing
func (tt *TransportTest) Close() {}
