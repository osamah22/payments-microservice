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
