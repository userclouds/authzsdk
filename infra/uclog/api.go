package uclog

import (
	"context"
	"fmt"
	"os"

	"userclouds.com/infra/namespace/region"
)

// TODO (sgarrity 10/23): remove this when we're done with REGION
func init() {
	region.InitLogger(Warningf)
}

//  A set of wrappers that log messages at a pre-set level

// Fatalf is equivalent to Printf() followed by a call to os.Exit(1).
func Fatalf(ctx context.Context, f string, args ...interface{}) {
	logWithLevelf(ctx, LogLevelError, "F", f, args...)
	// Because os.Exit doesn't run deferred functions close the transports before calling it so the
	// last messages end up in the log
	Close()
	os.Exit(1)
}

// Errorf logs an error with optional format-string parsing
func Errorf(ctx context.Context, f string, args ...interface{}) {
	logWithLevelf(ctx, LogLevelError, "E", f, args...)
}

// Warningf logs a string at info level (default visible in user console)
func Warningf(ctx context.Context, f string, args ...interface{}) {
	logWithLevelf(ctx, LogLevelWarning, "W", f, args...)
}

// Infof logs a string at info level (default visible in user console)
func Infof(ctx context.Context, f string, args ...interface{}) {
	logWithLevelf(ctx, LogLevelInfo, "I", f, args...)
}

// Debugf logs a string with optional format-string parsing
// by default these are internal-to-Userclouds logs
func Debugf(ctx context.Context, f string, args ...interface{}) {
	logWithLevelf(ctx, LogLevelDebug, "D", f, args...)
}

// Verbosef is the loudest
// Originally introduced to log DB queries / timing without killing dev console
func Verbosef(ctx context.Context, f string, args ...interface{}) {
	logWithLevelf(ctx, LogLevelVerbose, "V", f, args...)
}

func logWithLevelf(ctx context.Context, level LogLevel, levelPrefix, f string, args ...interface{}) {
	s := fmt.Sprintf(f, args...)
	s = fmt.Sprintf("[%s] %s", levelPrefix, s)
	Log(ctx, LogEvent{LogLevel: level, Code: EventCodeNone, Message: s, Count: 1})
}

// A set of wrappers that log counter events

// IncrementEvent records a UserClouds event without message or payload
func IncrementEvent(ctx context.Context, eventName string) {
	e := LogEvent{LogLevel: LogLevelNonMessage, Name: eventName, Count: 1}
	Log(ctx, e)
}

// IncrementEventWithPayload logs event related to security that carry a payload
func IncrementEventWithPayload(ctx context.Context, eventName string, payload string) {
	Log(ctx, LogEvent{LogLevel: LogLevelNonMessage, Name: eventName, Payload: payload, Count: 1})
}
