package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
)

// validationError is returned by the Validate method on typed request
// objects when a field is missing or invalid.
type validationError struct {
	msg string
}

func (e validationError) Error() string { return e.msg }

func errMissingField(name string) error {
	return validationError{msg: fmt.Sprintf("%s is required", name)}
}

func errInvalidField(name, reason string) error {
	return validationError{msg: fmt.Sprintf("%s %s", name, reason)}
}

func decodeJSON(r *http.Request, v interface{}) error {
	defer r.Body.Close()
	return json.NewDecoder(r.Body).Decode(v)
}

func queryInt(r *http.Request, key string, defaultVal int) int {
	v := r.URL.Query().Get(key)
	if v == "" {
		return defaultVal
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return defaultVal
	}
	return n
}

func queryString(r *http.Request, key string) string {
	return r.URL.Query().Get(key)
}
