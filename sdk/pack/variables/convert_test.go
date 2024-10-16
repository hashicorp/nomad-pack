// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package variables

import (
	"reflect"
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/shoenig/test/must"
	"github.com/zclconf/go-cty/cty"
)

func TestConvertCtyToInterface(t *testing.T) {
	ci.Parallel(t)

	// test basic type
	testCases := []struct {
		name string
		val  cty.Value
		t    reflect.Kind
	}{
		{"bool", cty.BoolVal(true), reflect.Bool},
		{"string", cty.StringVal("test"), reflect.String},
		{"number", cty.NumberIntVal(1), reflect.Int},
		{"map", cty.MapVal(map[string]cty.Value{"test": cty.BoolVal(true)}), reflect.Map},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ci.Parallel(t) // Parallel has to be set on the subtest also
			res, err := ConvertCtyToInterface(tc.val)
			must.NoError(t, err)

			resType := reflect.TypeOf(res).Kind()
			must.Eq(t, tc.t, resType)
		})
	}

	// test list of list
	t.Run("lists of lists", func(t *testing.T) {
		ci.Parallel(t) // Parallel has to be set on the subtest also
		testListOfList := cty.ListVal([]cty.Value{
			cty.ListVal([]cty.Value{
				cty.BoolVal(true),
			}),
		})

		resListOfList, err := ConvertCtyToInterface(testListOfList)
		must.NoError(t, err)

		tempList, ok := resListOfList.([]any)
		must.True(t, ok)

		_, ok = tempList[0].([]any)
		must.True(t, ok)
	})

	// test list of maps
	t.Run("list of maps", func(t *testing.T) {
		ci.Parallel(t) // Parallel has to be set on the subtest also
		testListOfMaps := cty.ListVal([]cty.Value{
			cty.MapVal(map[string]cty.Value{
				"test": cty.BoolVal(true),
			}),
		})

		resListOfMaps, err := ConvertCtyToInterface(testListOfMaps)
		must.NoError(t, err)

		_, ok := resListOfMaps.([]map[string]any)
		must.True(t, ok)
	})

	// test map of maps
	t.Run("map of maps", func(t *testing.T) {
		ci.Parallel(t) // Parallel has to be set on the subtest also
		testMapOfMaps := cty.MapVal(map[string]cty.Value{
			"test": cty.MapVal(map[string]cty.Value{"test": cty.BoolVal(true)}),
		})

		restMapOfMaps, err := ConvertCtyToInterface(testMapOfMaps)
		must.NoError(t, err)

		tempMapOfMaps, ok := restMapOfMaps.(map[string]any)
		must.True(t, ok)

		_, ok = tempMapOfMaps["test"].(map[string]any)
		must.True(t, ok)
	})

	// test map of objects
	t.Run("map of objects", func(t *testing.T) {
		ci.Parallel(t) // Parallel has to be set on the subtest also
		testMapOfObj := cty.MapVal(map[string]cty.Value{
			"t1": cty.ObjectVal(map[string]cty.Value{"b": cty.BoolVal(true)}),
			"t2": cty.ObjectVal(map[string]cty.Value{"b": cty.BoolVal(false)}),
		})

		restMapOfObj, err := ConvertCtyToInterface(testMapOfObj)
		must.NoError(t, err)

		tempMapOfObj, ok := restMapOfObj.(map[string]any)
		must.True(t, ok)

		tp1, ok := tempMapOfObj["t1"].(map[string]any)
		must.True(t, ok)
		tp2, ok := tempMapOfObj["t2"].(map[string]any)
		must.True(t, ok)

		b1, ok := tp1["b"].(bool)
		must.True(t, ok)
		must.True(t, b1)
		b2, ok := tp2["b"].(bool)
		must.True(t, ok)
		must.False(t, b2)
	})
}
