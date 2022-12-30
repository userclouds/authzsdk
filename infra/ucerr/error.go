package ucerr

import (
	"errors"
	"fmt"
	"runtime"
	"strings"
)

// UCError lets us figure out if this is a wrapped error
type UCError interface {
	BaseError() string
	Error() string // include this so UCError implements Error for erroras linter
	Friendly() string
	FriendlyStructure() interface{}
}

type ucError struct {
	text      string      // this is intended for internal use
	friendly  string      // (optional) this will get propagated to the user (or developer-user)
	structure interface{} // if non-nil, then FriendlyStructure() will a marshalable struct as its value

	underlying error

	function string
	filename string
	line     int
}

// Option defines a way to modify ucerr behavior
type Option interface {
	apply(*options)
}

type options struct {
	skipFrames int
}

type optFunc func(*options)

func (o optFunc) apply(os *options) {
	o(os)
}

var errorWrappingSuffix = ": %w"
var wrappedText = "(wrapped)"

const repoRoot = "userclouds/"

// Return a path relative to the repo root, assuming that:
// (1) there is no 'userclouds' directory created within the source tree of our repo,
// (2) the repo is cloned into the default directory.
// If the path is not within the repo, return the path unmodified.
func repoRelativePath(s string) string {
	if idx := strings.LastIndex(s, repoRoot); idx >= 0 {
		return s[idx+len(repoRoot):]
	}
	return s
}

// Given a fully qualified go function name "pkgname.[type].func",
// return "func" (or return string unchanged if no period found).
func funcName(s string) string {
	if idx := strings.LastIndex(s, "."); idx >= 0 {
		return s[idx+1:]
	}
	return s
}

// BaseError implements UCError
// Just return the error message(s), no stack trace
func (e ucError) BaseError() string {
	var msg string
	// how do we wrap what's underlying us?
	// 1) keep unwrapping until we're at the bottom of the wrapped stack
	// 2) start with the error message from the original error
	var uce UCError
	if errors.As(e.underlying, &uce) {
		msg = uce.BaseError()
	} else if e.underlying != nil {
		msg = e.underlying.Error()
	}

	// how do we display ourselves rationally?
	// 3) if the bottom of the stack is just wrapping a non-UCError, don't show (wrapped)
	t := e.text
	if t == wrappedText {
		t = ""
	}

	// is there enough to add a :
	// 4) if the bottom of the stack was a ucerr.New(), just show the text
	// 5) if the bottom of the stack was a ucerr.Wrap(), just show the base error
	if msg == "" {
		return t
	} else if t == "" {
		return msg
	}

	// 6) if the bottom of the stack was ucerr.Errorf(), show the original annotation + base error
	return fmt.Sprintf("%s: %s", t, msg)
}

// Error implements error
func (e ucError) Error() string {
	var u string
	if e.underlying != nil {
		u = fmt.Sprintf("%s\n", e.underlying.Error())
	}

	// fall back to friendly message if no internal message is defined
	t := e.text
	if e.text == "" {
		t = fmt.Sprintf("[friendly] %s", e.friendly)
	}

	return fmt.Sprintf("%s%s (File %s:%d, in %s)", u, t, e.filename, e.line, e.function)
}

// Unwrap implements errors.Unwrap for errors.Is
func (e *ucError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.underlying // ok if this returns nil
}

// New creates a new ucerr
func New(text string) error {
	return new(text, "", nil, 1, nil)
}

// Errorf is our local version of fmt.Errorf including callsite info
func Errorf(temp string, args ...interface{}) error {
	var wrapped error
	// if using %w to wrap another error, use our wrapping mechanism
	if strings.HasSuffix(temp, errorWrappingSuffix) {
		temp = strings.TrimSuffix(temp, errorWrappingSuffix)
		// use the safe cast in case this fails
		var ok bool
		wrapped, ok = args[len(args)-1].(error)
		if !ok {
			wrapped = New("seems as if ucerr.Errorf() was called with a non-error %w")
		}
		args = args[0 : len(args)-1]
	}
	return new(fmt.Sprintf(temp, args...), "", wrapped, 1, nil)
}

// Friendlyf wraps an error with a user-friendly message
func Friendlyf(err error, format string, args ...interface{}) error {
	s := fmt.Sprintf(format, args...)
	return new("", s, err, 1, nil)
}

// WrapWithFriendlyStructure wraps an error with a structured error
func WrapWithFriendlyStructure(err error, structure interface{}) error {
	return new("", "", err, 1, structure)
}

// Wrap wraps an existing error with an additional level of the callstack
func Wrap(err error, opts ...Option) error {
	if err == nil {
		return nil
	}
	var options options
	for _, opt := range opts {
		opt.apply(&options)
	}
	return new(wrappedText, "", err, options.skipFrames+1, nil)
}

// ExtraSkip tells Wrap to skip an extra frame in the stack when wrapping an error
// This allows calls like uchttp.Error() and jsonapi.MarshalError() to call Wrap()
// and capture the stack frame that actually logged the error (since we rarely call
// eg, jsonapi.MarshalError(ucerr.Wrap(err)), we lose useful debugging data)
func ExtraSkip() Option {
	return optFunc(func(o *options) { o.skipFrames++ })
}

// skips is the number of stack frames (besides new itself) to skip
func new(text, friendly string, wraps error, skips int, structure interface{}) error {
	function, filename, line := whereAmI(skips + 1)
	err := &ucError{
		text:       text,
		friendly:   friendly,
		underlying: wraps,
		function:   function,
		filename:   filename,
		line:       line,
		structure:  structure,
	}
	return err
}

// s == stack frames to skip not including myself
func whereAmI(s int) (string, string, int) {
	pc, filename, line, ok := runtime.Caller(s + 1)
	if !ok {
		return "", "", 0
	}
	f := runtime.FuncForPC(pc)
	return funcName(f.Name()), repoRelativePath(filename), line
}

// Friendly returns the friendly message, if any, or default string
// Currently takes the first one in the stack, although we could
// eventually extend this to allow composing etc
func (e ucError) Friendly() string {
	if e.friendly != "" {
		return e.friendly
	}

	var uce UCError
	if errors.As(e.underlying, &uce) {
		return uce.Friendly()
	}

	return "an unspecified error occurred"
}

// FriendlyStructure returns something that can be marshaled to JSON for the client to
// access programatically
func (e ucError) FriendlyStructure() interface{} {
	if e.structure != nil {
		return e.structure
	}

	var uce UCError
	if errors.As(e.underlying, &uce) {
		return uce.FriendlyStructure()
	}

	return nil
}

// UserFriendlyMessage is just a simple wrapper to handle casting error -> ucError
func UserFriendlyMessage(err error) string {
	var uce UCError
	if errors.As(err, &uce) {
		return uce.Friendly()
	}

	// note subtle difference in language from Friendly() identifies an
	// (unlikely) place where we didn't wrap an error with a ucError ever
	return "an unknown error occurred"
}

// UserFriendlyStructure exposes the structured error data if error is a ucError
func UserFriendlyStructure(err error) interface{} {
	var uce UCError
	if errors.As(err, &uce) {
		return uce.FriendlyStructure()
	}

	return nil
}
