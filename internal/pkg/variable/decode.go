package variable

import (
	"fmt"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/ext/typeexpr"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/convert"
)

func decodeVariableBlock(block *hcl.Block) (*Variable, hcl.Diagnostics) {

	content, diags := block.Body.Content(variableBlockSchema)
	if content == nil {
		return nil, diags
	}

	if diags == nil {
		diags = hcl.Diagnostics{}
	}

	v := &Variable{
		Name:      block.Labels[0],
		DeclRange: block.DefRange,
	}

	// Ensure the variable name is valid. If this isn't checked it will cause
	// problems in future use.
	if !hclsyntax.ValidIdentifier(v.Name) {
		diags = diags.Append(diagnosticInvalidVariableName(v.DeclRange.Ptr()))
	}

	// A variable doesn't need to declare a description. If it does, process
	// this and store it, along with any processing errors.
	if attr, exists := content.Attributes[variableAttributeDescription]; exists {
		val, descDiags := attr.Expr.Value(nil)
		diags = safeDiagnosticsExtend(diags, descDiags)

		if val.Type() == cty.String {
			v.Description = val.AsString()
		} else {
			diags = safeDiagnosticsAppend(diags, &hcl.Diagnostic{
				Severity: hcl.DiagError,
				Summary:  "Invalid type for description",
				Detail: fmt.Sprintf("The description attribute is expected to be of type string, got %s",
					val.Type().FriendlyName()),
				Subject: attr.Range.Ptr(),
			})
		}
	}

	// A variable doesn't need to declare a type. If it does, process this and
	// store it, along with any processing errors.
	if attr, exists := content.Attributes[variableAttributeType]; exists {
		ty, tyDiags := typeexpr.Type(attr.Expr)
		diags = safeDiagnosticsExtend(diags, tyDiags)
		v.Type = ty
	}

	// A variable doesn't need to declare a default. If it does, process this
	// and store it, along with any processing errors.
	if attr, exists := content.Attributes[variableAttributeDefault]; exists {
		val, valDiags := attr.Expr.Value(nil)
		diags = safeDiagnosticsExtend(diags, valDiags)

		// If the found type isn't cty.NilType, then attempt to covert the
		// default variable, so we know they are compatible.
		if v.Type != cty.NilType {
			var err *hcl.Diagnostic
			val, err = convertValUsingType(val, v.Type, attr.Expr.Range().Ptr())
			diags = safeDiagnosticsAppend(diags, err)
		}

		v.Value = val
	}

	return v, diags
}

// convertValUsingType is a wrapper around convert.Convert.
func convertValUsingType(val cty.Value, typ cty.Type, sub *hcl.Range) (cty.Value, *hcl.Diagnostic) {
	newVal, err := convert.Convert(val, typ)
	if err != nil {
		return cty.DynamicVal, diagnosticInvalidValueForType(err, sub)
	}
	return newVal, nil
}
