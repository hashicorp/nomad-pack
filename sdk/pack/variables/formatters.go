// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package variables

import (
	"fmt"
	"strings"

	"github.com/zclconf/go-cty/cty"
)

// printType recursively prints out a cty.Type specification in a format that
// matched the way in which it is defined.
func printType(t cty.Type) string {
	return printTypeR(t)
}

func printTypeR(t cty.Type) string {
	switch {
	case t.IsPrimitiveType():
		return t.FriendlyNameForConstraint()
	case t.IsListType():
		return "list(" + printTypeR(t.ElementType()) + ")"
	case t.IsMapType():
		return "map(" + printTypeR(t.ElementType()) + ")"
	case t.IsSetType():
		return "set(" + printTypeR(t.ElementType()) + ")"
	case t.IsTupleType():
		tts := t.TupleElementTypes()
		tfts := make([]string, len(tts))
		for i, tt := range tts {
			if tt.IsPrimitiveType() {
				tfts[i] = tt.FriendlyNameForConstraint()
			} else {
				tfts[i] = printTypeR(tt)
			}
		}
		return "tuple(" + strings.Join(tfts, ", ") + ")"
	case t.IsObjectType():
		at := t.AttributeTypes()
		ats := make([]string, len(at))
		i := 0
		for n, a := range at {
			if a.IsPrimitiveType() {
				ats[i] = n + " = " + a.FriendlyNameForConstraint()
			} else {
				ats[i] = n + " = " + printTypeR(a)
			}
			i++
		}
		return "object({" + strings.Join(ats, ", ") + "})"
	case t.HasDynamicTypes():
		return ("dynamic")
	default:
		return "«unknown type»"
	}
}

// printDefault recursively prints out a cty.Value specification in a format
// that matched the way it is defined. This allows us to not have to capture
// or replicate the original presentation. However, could this be captured in
// parsing?
func printDefault(v cty.Value) string {
	return printDefaultR(v)
}

func printDefaultR(v cty.Value) string {
	t := v.Type()
	switch {
	case t.IsPrimitiveType():
		return printPrimitiveValue(v)

	case t.IsListType(), t.IsSetType(), t.IsTupleType():
		// TODO, these could be optimized to be non-recursive calls for lists and sets of non-collection type
		acc := make([]string, 0, v.LengthInt())
		v.ForEachElement(func(key cty.Value, val cty.Value) bool { acc = append(acc, printDefaultR(val)); return false })
		return "[" + strings.Join(acc, ", ") + "]"

	case t.IsMapType(), t.IsObjectType():
		acc := make([]string, 0, v.LengthInt())
		v.ForEachElement(
			func(key cty.Value, val cty.Value) bool {
				acc = append(acc, fmt.Sprintf("%s = %s", printDefaultR(key), printDefaultR(val)))
				return false
			},
		)
		return "{" + strings.Join(acc, ", ") + "}"
	default:
		return "«unknown value type»"
	}
}

func printPrimitiveValue(v cty.Value) string {
	vI, _ := ConvertCtyToInterface(v)
	if v.Type() == cty.String {
		return fmt.Sprintf("%q", vI)
	}
	return fmt.Sprintf("%v", vI)
}
