package helpers

import (
	"errors"

	v1client "github.com/hashicorp/nomad-openapi/clients/go/v1"
	v1 "github.com/hashicorp/nomad-openapi/v1"
)

// UnwrapAPIError will check if err is a GenericOpenAPIError. If so,
// it will return a new error that's string is its Body() value.
// Otherwise it returns the err it was passed.
func UnwrapAPIError(err error) error {
	switch e := err.(type) {
	case v1client.GenericOpenAPIError:
		return errors.New(string(e.Body()))
	case *v1.APIError:
		return errors.New(string(e.Body()))
	default:
		return e
	}
}
