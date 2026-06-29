// Copyright IBM Corp. 2023, 2026
// SPDX-License-Identifier: MPL-2.0

package source

import (
	"fmt"

	"github.com/zclconf/go-cty/cty"
	ctyjson "github.com/zclconf/go-cty/cty/json"
)

// decodeValue converts a raw external-source value into a cty.Value of the
// expected type. String values are returned as-is; every other type is
// JSON-decoded into expectedType. source names the backend (for example
// "Consul") so callers share one decode path while keeping backend-specific
// error messages.
func decodeValue(source string, data []byte, expectedType cty.Type) (cty.Value, error) {
	if expectedType == cty.String {
		return cty.StringVal(string(data)), nil
	}

	val, err := ctyjson.Unmarshal(data, expectedType)
	if err != nil {
		return cty.NilVal, fmt.Errorf("decoding %s value as %s: %w", source, expectedType.FriendlyName(), err)
	}

	return val, nil
}
