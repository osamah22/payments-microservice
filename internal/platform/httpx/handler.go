package httpx

import (
	"net/http"
	"strconv"

	"github.com/osamah22/payments-microservice/internal/platform/apperr"
)

type HandlerFunc func(w http.ResponseWriter, r *http.Request) error

func (f HandlerFunc) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if err := f(w, r); err != nil {
		Error(w, r, err)
	}
}

// QueryInt returns the query parameter key as an int, or def if it's absent.
// A value that isn't a whole number is a validation error on that parameter.
func QueryInt(r *http.Request, key string, def int) (int, error) {
	v := r.URL.Query().Get(key)
	if v == "" {
		return def, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		var v apperr.ValidationError
		v.Add("key", "must be a whole number")
		return 0, v.Err()
	}
	return n, err
}
