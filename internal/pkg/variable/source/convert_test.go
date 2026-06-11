// Copyright IBM Corp. 2023, 2026
// SPDX-License-Identifier: MPL-2.0

package source

import (
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/shoenig/test/must"
	"github.com/zclconf/go-cty/cty"
)

func TestConvertJSONToCty(t *testing.T) {
	ci.Parallel(t)

	// Test basic types
	testCases := []struct {
		name  string
		input any
		want  cty.Type
	}{
		{"string", "hello", cty.String},
		{"number", float64(42), cty.Number},
		{"bool true", true, cty.Bool},
		{"bool false", false, cty.Bool},
		{"null", nil, cty.DynamicPseudoType},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ci.Parallel(t)
			result, err := convertJSONToCty(tc.input)
			must.NoError(t, err)
			must.True(t, result.Type().Equals(tc.want))
		})
	}

	// Test string value
	t.Run("string value", func(t *testing.T) {
		ci.Parallel(t)
		result, err := convertJSONToCty("test")
		must.NoError(t, err)
		must.Eq(t, "test", result.AsString())
	})

	// Test number value
	t.Run("number value", func(t *testing.T) {
		ci.Parallel(t)
		result, err := convertJSONToCty(float64(42))
		must.NoError(t, err)
		f, _ := result.AsBigFloat().Float64()
		must.Eq(t, 42.0, f)
	})

	// Test empty list
	t.Run("empty list", func(t *testing.T) {
		ci.Parallel(t)
		result, err := convertJSONToCty([]any{})
		must.NoError(t, err)
		must.True(t, result.Type().Equals(cty.List(cty.DynamicPseudoType)))
	})

	// Test list of numbers
	t.Run("list of numbers", func(t *testing.T) {
		ci.Parallel(t)
		input := []any{float64(1), float64(2), float64(3)}
		result, err := convertJSONToCty(input)
		must.NoError(t, err)
		must.True(t, result.Type().Equals(cty.List(cty.Number)))
		must.Eq(t, 3, result.LengthInt())
	})

	// Test list of strings
	t.Run("list of strings", func(t *testing.T) {
		ci.Parallel(t)
		input := []any{"a", "b", "c"}
		result, err := convertJSONToCty(input)
		must.NoError(t, err)
		must.True(t, result.Type().Equals(cty.List(cty.String)))
	})

	// Test empty object
	t.Run("empty object", func(t *testing.T) {
		ci.Parallel(t)
		result, err := convertJSONToCty(map[string]any{})
		must.NoError(t, err)
		must.True(t, result.Type().Equals(cty.EmptyObject))
	})

	// Test object with mixed types
	t.Run("object with mixed types", func(t *testing.T) {
		ci.Parallel(t)
		input := map[string]any{
			"name":    "test",
			"count":   float64(5),
			"enabled": true,
		}
		result, err := convertJSONToCty(input)
		must.NoError(t, err)

		objMap := result.AsValueMap()
		must.Eq(t, "test", objMap["name"].AsString())

		count, _ := objMap["count"].AsBigFloat().Float64()
		must.Eq(t, 5.0, count)

		must.True(t, objMap["enabled"].True())
	})

	// Test nested object
	t.Run("nested object", func(t *testing.T) {
		ci.Parallel(t)
		input := map[string]any{
			"config": map[string]any{
				"timeout": float64(30),
				"debug":   false,
			},
		}
		result, err := convertJSONToCty(input)
		must.NoError(t, err)

		objMap := result.AsValueMap()
		configMap := objMap["config"].AsValueMap()

		timeout, _ := configMap["timeout"].AsBigFloat().Float64()
		must.Eq(t, 30.0, timeout)
		must.True(t, configMap["debug"].False())
	})

	// Test real-world pack variable examples
	t.Run("pack replicas", func(t *testing.T) {
		ci.Parallel(t)
		result, err := convertJSONToCty(float64(3))
		must.NoError(t, err)
		must.True(t, result.Type().Equals(cty.Number))
	})

	t.Run("pack ports", func(t *testing.T) {
		ci.Parallel(t)
		input := []any{float64(8080), float64(8443)}
		result, err := convertJSONToCty(input)
		must.NoError(t, err)
		must.True(t, result.Type().Equals(cty.List(cty.Number)))
	})

	t.Run("pack config", func(t *testing.T) {
		ci.Parallel(t)
		input := map[string]any{
			"timeout": float64(30),
			"debug":   false,
			"region":  "us-west-2",
		}
		result, err := convertJSONToCty(input)
		must.NoError(t, err)
		must.True(t, result.Type().IsObjectType())
	})
}
