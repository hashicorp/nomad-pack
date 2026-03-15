// Copyright IBM Corp. 2023, 2026
// SPDX-License-Identifier: MPL-2.0

package variables

import (
	"fmt"
	"strings"

	"github.com/zclconf/go-cty/cty"
)

// PrintType recursively prints out a cty.Type specification in a format that
// matched the way in which it is defined.
func PrintType(t cty.Type) string {
	return formatTypeWithDepth(t, 0)
}

// indentStr returns the indentation string for a given depth level.
func indentStr(depth int) string {
	return strings.Repeat("  ", depth)
}

// formatObjectType formats an object type with smart line-breaking for complex objects.
// Produces newlines for readability; all indentation is handled by IndentTypeString.
func formatObjectType(t cty.Type, depth int) string {
	at := t.AttributeTypes()
	if len(at) == 0 {
		return "object({})"
	}

	ats := make([]string, len(at))
	i := 0
	for n, a := range at {
		if a.IsPrimitiveType() {
			ats[i] = n + " = " + a.FriendlyNameForConstraint()
		} else {
			ats[i] = n + " = " + formatTypeWithDepth(a, depth+1)
		}
		i++
	}

	// For simple objects (2 or fewer fields), keep on single line
	if len(ats) <= 2 {
		return "object({" + strings.Join(ats, ", ") + "})"
	}

	// For complex objects, check if single-line version exceeds 70 characters
	singleLine := "object({" + strings.Join(ats, ", ") + "})"
	if len(singleLine) <= 70 {
		return singleLine
	}

	// Use multi-line format for complex/long objects
	// Add extra indentation for nested levels
	indent := indentStr(depth)
	sep := ",\n" + indent
	return "object({\n" + indent + strings.Join(ats, sep) + "\n})"
}

// formatTypeWithDepth formats a type with awareness of nesting depth.
// Used by formatObjectType to properly format nested types.
func formatTypeWithDepth(t cty.Type, depth int) string {
	switch {
	case t.IsPrimitiveType():
		return t.FriendlyNameForConstraint()
	case t.IsListType():
		return "list(" + formatTypeWithDepth(t.ElementType(), depth) + ")"
	case t.IsMapType():
		return "map(" + formatTypeWithDepth(t.ElementType(), depth) + ")"
	case t.IsSetType():
		return "set(" + formatTypeWithDepth(t.ElementType(), depth) + ")"
	case t.IsTupleType():
		tts := t.TupleElementTypes()
		tfts := make([]string, len(tts))
		for i, tt := range tts {
			tfts[i] = formatTypeWithDepth(tt, depth)
		}
		return "tuple(" + strings.Join(tfts, ", ") + ")"
	case t.IsObjectType():
		return formatObjectType(t, depth)
	case t.HasDynamicTypes():
		return "dynamic"
	default:
		return "«unknown type»"
	}
}

// IndentTypeString indents all newlines in a type string by the given number of spaces.
// This is used to properly format multi-line object types in the output.
func IndentTypeString(typeStr string, indent int) string {
	if !strings.Contains(typeStr, "\n") {
		return typeStr
	}
	indentStr := strings.Repeat(" ", indent)
	return strings.ReplaceAll(typeStr, "\n", "\n"+indentStr)
}

// PrintDefault recursively prints out a cty.Value specification in a format
// that matched the way it is defined. This allows us to not have to capture
// or replicate the original presentation. However, could this be captured in
// parsing?
func PrintDefault(v cty.Value) string {
	return printDefaultR(v, 0)
}

// hasComplexElements returns true if the collection contains complex values
// (objects, maps, or nested lists) that benefit from multi-line formatting.
func hasComplexElements(v cty.Value) bool {
	if v.LengthInt() == 0 {
		return false
	}
	hasComplex := false
	v.ForEachElement(func(key cty.Value, val cty.Value) bool {
		if val.Type().IsObjectType() || val.Type().IsMapType() || val.Type().IsListType() {
			hasComplex = true
			return true
		}
		return false
	})
	return hasComplex
}

func printDefaultR(v cty.Value, depth int) string {
	t := v.Type()
	switch {
	case t.IsPrimitiveType():
		return printPrimitiveValue(v)

	case t.IsListType(), t.IsSetType(), t.IsTupleType():
		if v.LengthInt() == 0 {
			return "[]"
		}

		acc := make([]string, 0, v.LengthInt())
		v.ForEachElement(func(key cty.Value, val cty.Value) bool {
			acc = append(acc, printDefaultR(val, depth+1))
			return false
		})

		// Format lists with complex values across multiple lines
		if hasComplexElements(v) && depth <= 1 {
			indent := indentStr(depth + 1)
			sep := ",\n" + indent
			return "[\n" + indent + strings.Join(acc, sep) + "\n" + indentStr(depth) + "]"
		}
		return "[" + strings.Join(acc, ", ") + "]"

	case t.IsMapType(), t.IsObjectType():
		if v.LengthInt() == 0 {
			return "{}"
		}

		acc := make([]string, 0, v.LengthInt())
		v.ForEachElement(
			func(key cty.Value, val cty.Value) bool {
				acc = append(acc, fmt.Sprintf("%s = %s", printDefaultR(key, depth+1), printDefaultR(val, depth+1)))
				return false
			},
		)

		// For objects/maps with multiple entries or at top level, use multi-line format
		if depth <= 1 && len(acc) > 1 {
			indent := indentStr(depth + 1)
			sep := ",\n" + indent
			return "{\n" + indent + strings.Join(acc, sep) + "\n" + indentStr(depth) + "}"
		}
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
