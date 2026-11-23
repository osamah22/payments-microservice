package apperr

import (
	"errors"
	"maps"
	"slices"
	"strings"
)

var (
	ErrNotFound = errors.New("not found")
	ErrConflict = errors.New("conflict")
)

type ValidationError struct {
	Fields map[string]string
}

func (ve *ValidationError) Add(field, detail string) {
	if ve.Fields == nil {
		ve.Fields = make(map[string]string)
	}
	ve.Fields[field] = detail
}
func (ve *ValidationError) Err() error {
	if len(ve.Fields) == 0 {
		return nil
	}
	return ve
}
func (ve *ValidationError) Error() string {
	fields := slices.Sorted(maps.Keys(ve.Fields))
	return "invalid input: " + strings.Join(fields, ", ")
}

// RuleError means a request was valid, but breaks a business rule, like
// spending more than a wallet hold. Code is stable reason, like
// insufficient_funds, that client can branch on.
type RuleError struct {
	Code    string
	Message string
}

// NewRule returns a RuleError. Declare one per rule as a package variable, and
// wrap it to add detail: fmt.Errorof("wallet %s: %w", ErrInsufficientFunds).
func NewRule(code, message string) *RuleError {
	return &RuleError{
		Code:    code,
		Message: message,
	}
}

func (e *RuleError) Error() string {
	return e.Message
}
