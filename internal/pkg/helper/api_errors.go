package helpers

import (
	"errors"

	v1client "github.com/hashicorp/nomad-openapi/clients/go/v1"
)

// UnwrapAPIError will check if err is a GenericOpenAPIError. If so,
// it will return a new error that's string is its Body() value.
// Otherwise it returns the err it was passed.
func UnwrapAPIError(err error) error {
	var apiErr v1client.GenericOpenAPIError
	if errors.As(err, &apiErr) {
		return errors.New(string(apiErr.Body()))
	}
	return err
}
