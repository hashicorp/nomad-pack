// Copyright (c) HashiCorp, Inc.
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

func DecodeVariableBlock(block *hcl.Block) (*variables.Variable, hcl.Diagnostics) {

	content, diags := block.Body.Content(schema.VariableBlockSchema)
	if content == nil {
		return nil, diags
	}

	if diags == nil {
		diags = hcl.Diagnostics{}
	}

	v := &variables.Variable{
		Name:      variables.VariableID(block.Labels[0]),
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

		// If the found type isn't cty.NilType, then attempt to covert the
		// default variable, so we know they are compatible.
		if v.Type != cty.NilType {
			var err *hcl.Diagnostic
			val, err = hclhelp.ConvertValUsingType(val, v.Type, attr.Expr.Range().Ptr())
			diags = packdiags.SafeDiagnosticsAppend(diags, err)
		}
		v.SetDefault(val)
		v.Value = val
	}

	return v, diags
}
