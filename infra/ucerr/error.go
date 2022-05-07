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
}

type ucError struct {
	text       string
	underlying error

	function string
	filename string
	line     int
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
	return fmt.Sprintf("%s%s (File %s:%d, in %s)", u, e.text, e.filename, e.line, e.function)
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
	return new(text, nil, 1)
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
	return new(fmt.Sprintf(temp, args...), wrapped, 1)
}

// Wrap wraps an existing error with an additional level of the callstack
func Wrap(err error) error {
	if err == nil {
		return nil
	}
	return new(wrappedText, err, 1)
}

// skips is the number of stack frames (besides new itself) to skip
func new(text string, wraps error, skips int) error {
	function, filename, line := whereAmI(skips + 1)
	err := &ucError{
		text:       text,
		underlying: wraps,
		function:   function,
		filename:   filename,
		line:       line,
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
