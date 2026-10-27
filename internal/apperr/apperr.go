// Package apperr defines errors that are safe to show to API clients.
// Anything that is not an *Error is treated as internal and masked.
package apperr

import (
	"errors"
	"fmt"
)

type Code string

const (
	CodeUnauthenticated Code = "UNAUTHENTICATED"
	CodeForbidden       Code = "FORBIDDEN"
	CodeNotFound        Code = "NOT_FOUND"
	CodeInvalidInput    Code = "INVALID_INPUT"
	CodeConflict        Code = "CONFLICT"
	CodeRateLimited     Code = "RATE_LIMITED"
	CodeInternal        Code = "INTERNAL"
)

type Error struct {
	Code    Code
	Message string   // shown to the client; never include secrets or SQL
	Field   []string // input path for INVALID_INPUT, e.g. ["input", "stages", "2", "amount"]
	Err     error    // cause, logged but never shown
}

func (e *Error) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (e *Error) Unwrap() error { return e.Err }

func New(code Code, msg string) *Error { return &Error{Code: code, Message: msg} }

func Wrap(err error, code Code, msg string) *Error { return &Error{Code: code, Message: msg, Err: err} }

func Invalid(msg string, field ...string) *Error {
	return &Error{Code: CodeInvalidInput, Message: msg, Field: field}
}

func Unauthenticated() *Error { return New(CodeUnauthenticated, "sign in required") }
func Forbidden() *Error       { return New(CodeForbidden, "you don't have access to this") }
func NotFound(what string) *Error {
	return New(CodeNotFound, what+" not found")
}

// As returns the *Error in err's chain, if any.
func As(err error) (*Error, bool) {
	var e *Error
	ok := errors.As(err, &e)
	return e, ok
}
