// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package decoder

import (
	"fmt"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/ext/typeexpr"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/nomad-pack/internal/pkg/errors/packdiags"
	"github.com/hashicorp/nomad-pack/internal/pkg/variable/internal/hclhelp"
	"github.com/hashicorp/nomad-pack/internal/pkg/variable/schema"
	"github.com/hashicorp/nomad-pack/sdk/pack/variables"
	"github.com/zclconf/go-cty/cty"
)

// DecodeVariableBlock parses a variable definition into a variable. When the
// provided block or its Body is nil, the function returns (nil, nil)
func DecodeVariableBlock(block *hcl.Block) (*variables.Variable, hcl.Diagnostics) {
	if block == nil || block.Body == nil {
		return nil, hcl.Diagnostics{}
	}

	// If block and Body is non-nil, then the block is ready to be parsed
	content, diags := block.Body.Content(schema.VariableBlockSchema)
	if content == nil {
		return nil, diags
	}

	if diags == nil {
		diags = hcl.Diagnostics{}
	}

	v := &variables.Variable{
		Name:      variables.ID(block.Labels[0]),
		DeclRange: block.DefRange,
	}

	// Ensure the variable name is valid. If this isn't checked it will cause
	// problems in future use.
	if !hclsyntax.ValidIdentifier(v.Name.String()) {
		diags = diags.Append(packdiags.DiagInvalidVariableName(v.DeclRange.Ptr()))
	}

	// A variable doesn't need to declare a description. If it does, process
	// this and store it, along with any processing errors.
	if attr, exists := content.Attributes[schema.VariableAttributeDescription]; exists {
		val, descDiags := attr.Expr.Value(nil)
		diags = packdiags.SafeDiagnosticsExtend(diags, descDiags)

		if val.Type() == cty.String {
			v.SetDescription(val.AsString())
		} else {
			diags = packdiags.SafeDiagnosticsAppend(diags, &hcl.Diagnostic{
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
	if attr, exists := content.Attributes[schema.VariableAttributeType]; exists {
		ty, tyDiags := typeexpr.Type(attr.Expr)
		diags = packdiags.SafeDiagnosticsExtend(diags, tyDiags)
		v.SetType(ty)
	}

	// A variable doesn't need to declare a default. If it does, process this
	// and store it, along with any processing errors.
	if attr, exists := content.Attributes[schema.VariableAttributeDefault]; exists {
		val, valDiags := attr.Expr.Value(nil)
		diags = packdiags.SafeDiagnosticsExtend(diags, valDiags)

		// Attempt to convert the default to the variable's declared type
		// to produce an informative error if they are not compatible.
		if shouldCompareDefaultType(v.Type, val.Type()) {
			var err *hcl.Diagnostic
			val, err = hclhelp.ConvertValUsingType(val, v.Type, attr.Expr.Range().Ptr())
			diags = packdiags.SafeDiagnosticsAppend(diags, err)
		}
		v.SetDefault(val)
		v.Value = val
	}

	if diags.HasErrors() {
		return nil, diags
	}

	return v, diags
}

func shouldCompareDefaultType(varType, defaultType cty.Type) bool {
	// if there is no declared type, there's nothing to check against.
	if varType == cty.NilType {
		return false
	}
	// different type names will certainly produce some kind of error,
	// hopefully an informative one.
	if varType.FriendlyName() != defaultType.FriendlyName() {
		return true
	}
	// if they're both objects, and the default has any attribute set,
	// we want to make sure those attributes match the declared type.
	if varType.IsObjectType() &&
		defaultType.IsObjectType() && len(defaultType.AttributeTypes()) > 0 {
		return true
	}
	return false
}
