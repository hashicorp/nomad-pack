// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package hclhelp

import (
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/nomad-pack/internal/pkg/errors/packdiags"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/convert"
)

// expressionFromVariableDefinition attempts to convert the string HCL
// expression to a hydrated hclsyntax.Expression.
func ExpressionFromVariableDefinition(file, val string, varType cty.Type) (hclsyntax.Expression, hcl.Diagnostics) {
	switch varType {
	case cty.String, cty.Number, cty.NilType:
		return &hclsyntax.LiteralValueExpr{Val: cty.StringVal(val)}, nil
	default:
		return hclsyntax.ParseExpression([]byte(val), file, hcl.Pos{Line: 1, Column: 1})
	}
}

// convertValUsingType is a wrapper around convert.Convert.
func ConvertValUsingType(val cty.Value, typ cty.Type, sub *hcl.Range) (cty.Value, *hcl.Diagnostic) {
	newVal, err := convert.Convert(val, typ)
	if err != nil {
		return cty.DynamicVal, packdiags.DiagInvalidValueForType(err, sub)
	}
	return newVal, nil
}
