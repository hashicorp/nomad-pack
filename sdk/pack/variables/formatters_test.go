package variables

import (
	"strings"
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/shoenig/test/must"
	"github.com/zclconf/go-cty/cty"
)

func TestFormatters_printType(t *testing.T) {
	ci.Parallel(t)
	testCases := []struct {
		name   string
		input  cty.Type
		expect string
		err    error
	}{
		{
			name:   "string",
			input:  cty.String,
			expect: "string",
		},
		{
			name:   "bool",
			input:  cty.Bool,
			expect: "bool",
		},
		{
			name:   "number",
			input:  cty.Number,
			expect: "number",
		},
		{
			name:   "any",
			input:  cty.DynamicPseudoType,
			expect: "dynamic",
		},
		{
			name:   "list/string",
			input:  cty.List(cty.String),
			expect: "list(string)",
		},
		{
			name:   "list/bool",
			input:  cty.List(cty.Bool),
			expect: "list(bool)",
		},
		{
			name:   "list/number",
			input:  cty.List(cty.Number),
			expect: "list(number)",
		},
		{
			name:   "list/any",
			input:  cty.List(cty.DynamicPseudoType),
			expect: "list(dynamic)",
		},
		{
			name:   "map/string",
			input:  cty.Map(cty.String),
			expect: "map(string)",
		},
		{
			name:   "map/bool",
			input:  cty.Map(cty.Bool),
			expect: "map(bool)",
		},
		{
			name:   "map/number",
			input:  cty.Map(cty.Number),
			expect: "map(number)",
		},
		{
			name:   "map/any",
			input:  cty.Map(cty.DynamicPseudoType),
			expect: "map(dynamic)",
		},
		{
			name:   "set/string",
			input:  cty.Set(cty.String),
			expect: "set(string)",
		},
		{
			name:   "set/bool",
			input:  cty.Set(cty.Bool),
			expect: "set(bool)",
		},
		{
			name:   "set/number",
			input:  cty.Set(cty.Number),
			expect: "set(number)",
		},
		{
			name:   "set/any",
			input:  cty.Set(cty.DynamicPseudoType),
			expect: "set(dynamic)",
		},
		{
			name:   "tuple",
			input:  cty.Tuple([]cty.Type{cty.String, cty.Bool, cty.Number, cty.DynamicPseudoType}),
			expect: "tuple(string, bool, number, dynamic)",
		},
	}
	for _, tc := range testCases {
		tc := tc // capture range variable
		t.Run(tc.name, func(t *testing.T) {
			ci.Parallel(t)
			out := printType(tc.input)
			must.Eq(t, tc.expect, out, must.Sprint(tc.input.FriendlyName()))
		})
	}

	t.Run("object", func(t *testing.T) {
		// Object is special cased because it has a map that prints in
		// non-guaranteed order.
		ci.Parallel(t)
		var tc = struct {
			name   string
			input  cty.Type
			expect string
			err    error
		}{
			name: "object",
			input: cty.Object(map[string]cty.Type{
				"str": cty.String,
				"b":   cty.Bool,
				"num": cty.Number,
				"any": cty.DynamicPseudoType}),
			expect: "object({str = string, b = bool, num = number, any = dynamic})",
		}
		out := printType(tc.input)
		exp := tc.expect
		must.True(t, strings.HasPrefix(out, "object({"))
		out = strings.TrimPrefix(out, "object({")
		exp = strings.TrimPrefix(exp, "object({")

		must.True(t, strings.HasSuffix(out, "})"))
		out = strings.TrimSuffix(out, "})")
		exp = strings.TrimSuffix(exp, "})")

		oParts := strings.Split(out, ", ")
		eParts := strings.Split(exp, ", ")

		must.SliceContainsAll(t, eParts, oParts)
	})
}

func TestFormatters_printDefault(t *testing.T) {
	ci.Parallel(t)
	testCases := []struct {
		name   string
		input  cty.Value
		expect string
		err    error
	}{
		{
			name:   "string",
			input:  cty.StringVal("test"),
			expect: `"test"`,
		},
		{
			name:   "bool/true",
			input:  cty.BoolVal(true),
			expect: "true",
		},
		{
			name:   "bool/false",
			input:  cty.BoolVal(false),
			expect: "false",
		},
		{
			name:   "number/int",
			input:  cty.NumberIntVal(-100),
			expect: "-100",
		},
		{
			name:   "number/uint",
			input:  cty.NumberUIntVal(100),
			expect: "100",
		},
		{
			name:   "number/float/positive",
			input:  cty.NumberFloatVal(0.2),
			expect: "0.2",
		},
		{
			name:   "number/float/negative",
			input:  cty.NumberFloatVal(-0.2),
			expect: "-0.2",
		},
		{
			name:   "number/float/zero",
			input:  cty.NumberFloatVal(0),
			expect: "0",
		},
		{
			name: "list/string",
			input: cty.ListVal([]cty.Value{
				cty.StringVal("a"), cty.StringVal("b"), cty.StringVal("c"),
			}),
			expect: `["a", "b", "c"]`,
		},
		{
			name: "list/bool",
			input: cty.ListVal([]cty.Value{
				cty.BoolVal(true), cty.BoolVal(false), cty.BoolVal(true),
			}),
			expect: `[true, false, true]`,
		},
		{
			name: "list/number",
			input: cty.ListVal([]cty.Value{
				cty.NumberFloatVal(-1.1),
				cty.NumberIntVal(-1),
				cty.NumberIntVal(0),
				cty.NumberUIntVal(0),
				cty.NumberFloatVal(0),
				cty.NumberIntVal(1),
				cty.NumberFloatVal(1.1),
				cty.NumberUIntVal(2),
			}),
			expect: `[-1.1, -1, 0, 0, 0, 1, 1.1, 2]`,
		},
		{
			name: "map/string",
			input: cty.MapVal(map[string]cty.Value{
				"a": cty.StringVal("apple"),
				"b": cty.StringVal("ball"),
			}),
			expect: `{"a" = "apple", "b" = "ball"}`,
		},
		{
			name: "map/bool",
			input: cty.MapVal(map[string]cty.Value{
				"a": cty.BoolVal(false),
				"b": cty.BoolVal(true),
			}),
			expect: `{"a" = false, "b" = true}`,
		},
		{
			name: "map/number",
			input: cty.MapVal(map[string]cty.Value{
				"a": cty.NumberIntVal(0),
				"b": cty.NumberFloatVal(2.4),
			}),
			expect: `{"a" = 0, "b" = 2.4}`,
		},
		{
			name: "set/string",
			input: cty.SetVal([]cty.Value{
				cty.StringVal("a"), cty.StringVal("b"), cty.StringVal("c"),
			}),
			expect: `["a", "b", "c"]`,
		},
		{
			name: "set/bool",
			input: cty.SetVal([]cty.Value{
				cty.BoolVal(true), cty.BoolVal(false), cty.BoolVal(true),
			}),
			expect: `[false, true]`,
		},
		{
			name: "set/number",
			input: cty.SetVal([]cty.Value{
				cty.NumberFloatVal(-1.1),
				cty.NumberIntVal(-1),
				cty.NumberIntVal(0),
				cty.NumberUIntVal(0),
				cty.NumberFloatVal(0),
				cty.NumberIntVal(1),
				cty.NumberFloatVal(1.1),
				cty.NumberUIntVal(2),
			}),
			expect: `[-1.1, -1, 0, 1, 1.1, 2]`,
		},
		{
			name: "tuple",
			input: cty.TupleVal([]cty.Value{
				cty.StringVal("a"),
				cty.BoolVal(true),
				cty.NumberFloatVal(0.2),
				cty.ListVal([]cty.Value{
					cty.StringVal("a"), cty.StringVal("b"), cty.StringVal("c"),
				}),
				cty.ObjectVal(map[string]cty.Value{
					"foo": cty.StringVal("bar"),
				}),
			}),
			expect: `["a", true, 0.2, ["a", "b", "c"], {"foo" = "bar"}]`,
		},
	}
	for _, tc := range testCases {
		tc := tc // capture range variable
		t.Run(tc.name, func(t *testing.T) {
			ci.Parallel(t)
			out := printDefault(tc.input)
			must.Eq(t, tc.expect, out, must.Sprint(tc.input.GoString()))
		})
	}

}
