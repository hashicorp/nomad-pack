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
	var openApiErr v1client.GenericOpenAPIError
	if errors.As(err, &openApiErr) {
		return errors.New(string(openApiErr.Body()))
	}
	var apiErr v1.APIError
	if errors.As(err, &apiErr) {
		return errors.New(apiErr.Error())
	}
	return err
}
