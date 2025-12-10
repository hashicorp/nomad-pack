// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package errors

import (
	stdErrors "errors"
)

var As = stdErrors.As
var Is = stdErrors.Is
var New = stdErrors.New
var Unwrap = stdErrors.Unwrap

// newError is an alias to the standard errors.New func for use inside
// the errors package to make the code's intention more obvious than
// undecorated calls to New might.
var newError = stdErrors.New
