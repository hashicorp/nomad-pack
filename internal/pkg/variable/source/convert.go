// Copyright IBM Corp. 2023, 2026
// SPDX-License-Identifier: MPL-2.0

package source

import (
	"fmt"

	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/gocty"
)

// convertJSONToCty converts a Go any (from JSON) to a cty.Value.
// This is shared by multiple source implementations that need to convert
// JSON data to HCL types.
func convertJSONToCty(v any) (cty.Value, error) {
	switch val := v.(type) {
	case nil:
		return cty.NullVal(cty.DynamicPseudoType), nil

	case string:
		return cty.StringVal(val), nil

	case float64:
		return cty.NumberFloatVal(val), nil

	case bool:
		return cty.BoolVal(val), nil

	case []any:
		if len(val) == 0 {
			return cty.ListValEmpty(cty.DynamicPseudoType), nil
		}

		// Convert each element
		elements := make([]cty.Value, len(val))
		for i, elem := range val {
			elemVal, err := convertJSONToCty(elem)
			if err != nil {
				return cty.NilVal, fmt.Errorf("failed to convert list element %d: %w", i, err)
			}
			elements[i] = elemVal
		}

		// Create a list with a unified type
		return cty.ListVal(elements), nil

	case map[string]any:
		if len(val) == 0 {
			return cty.EmptyObjectVal, nil
		}

		// Convert each value
		attrs := make(map[string]cty.Value)
		for k, v := range val {
			attrVal, err := convertJSONToCty(v)
			if err != nil {
				return cty.NilVal, fmt.Errorf("failed to convert object attribute %s: %w", k, err)
			}
			attrs[k] = attrVal
		}

		return cty.ObjectVal(attrs), nil

	default:
		ty, err := gocty.ImpliedType(v)
		if err != nil {
			return cty.NilVal, fmt.Errorf("unsupported type %T: %w", v, err)
		}

		ctyVal, err := gocty.ToCtyValue(v, ty)
		if err != nil {
			return cty.NilVal, fmt.Errorf("failed to convert %T to cty.Value: %w", v, err)
		}

		return ctyVal, nil
	}
}
