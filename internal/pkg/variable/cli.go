// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package variable

import (
	"fmt"
	"os"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/nomad-pack/internal/pkg/errors/packdiags"
	"github.com/zclconf/go-cty/cty"
)

func (p *Parser) parseEnvVariable(name string, rawVal string) hcl.Diagnostics {
	return p.parseVariableImpl(name, rawVal, p.envOverrideVars, name, "environment")

}
func (p *Parser) parseFlagVariable(name string, rawVal string) hcl.Diagnostics {
	return p.parseVariableImpl(name, rawVal, p.flagOverrideVars, "-var", "arguments")
}

func (p *Parser) parseVariableImpl(name, rawVal string, tgt map[PackID][]*Variable, typeTxt, rangeDesc string) hcl.Diagnostics {
	if rangeDesc == "environment" {
		name = strings.TrimPrefix(name, VarEnvPrefix)
	}

	// Split the name to see if we have a namespace CLI variable for a child
	// pack and set the default packVarName.
	splitName := strings.Split(name, ".")

	if len(splitName) < 2 || splitName[0] != p.cfg.ParentPackID.String() {
		return hcl.Diagnostics{
			{
				Severity: hcl.DiagError,
				Summary:  fmt.Sprintf("Invalid %s option", typeTxt),
				Detail:   fmt.Sprintf("The given %s option %s=%s is not correctly specified. The variable name must be an dot-separated, absolute path to a variable starting with the root pack name %s.", typeTxt, name, rawVal, p.cfg.ParentPackID),
			},
		}
	}

	// Generate a filename based on the incoming var, so we have some context for
	// any HCL diagnostics.

	// Get a reasonable count for the lines in the provided value. You'd think
	// these had to be flat, but naaah.
	lines := strings.Split(rawVal, "\n")
	lc := len(lines)
	endCol := len(lines[lc-1])

	fakeRange := hcl.Range{
		Filename: fmt.Sprintf("<value for var %s from %s>", name, rangeDesc),
		Start:    hcl.Pos{Line: 1, Column: 1, Byte: 0},
		End:      hcl.Pos{Line: lc, Column: endCol, Byte: len(rawVal)},
	}

	varPID := PackID(strings.Join(splitName[:len(splitName)-1], "."))
	varVID := VariableID(splitName[len(splitName)-1])
	// If the variable has not been configured in the root then exit. This is a
	// standard requirement, especially because we would be unable to ensure a
	// consistent type.
	existing, exists := p.rootVars[varPID][varVID]
	if !exists {
		return hcl.Diagnostics{packdiags.DiagMissingRootVar(name, &fakeRange)}
	}

	expr, diags := expressionFromVariableDefinition(fakeRange.Filename, rawVal, existing.Type)
	if diags.HasErrors() {
		return diags
	}

	val, diags := expr.Value(nil)
	if diags.HasErrors() {
		return diags
	}

	// If our stored type isn't cty.NilType then attempt to covert the override
	// variable, so we know they are compatible.
	if existing.Type != cty.NilType {
		var err *hcl.Diagnostic
		val, err = convertValUsingType(val, existing.Type, expr.Range().Ptr())
		if err != nil {
			return hcl.Diagnostics{err}
		}
	}

	// We have a verified override variable.
	v := Variable{
		Name:      varVID,
		Type:      val.Type(),
		Value:     val,
		DeclRange: fakeRange,
	}
	tgt[varPID] = append(tgt[varPID], &v)

	return nil
}

// expressionFromVariableDefinition attempts to convert the string HCL
// expression to a hydrated hclsyntax.Expression.
func expressionFromVariableDefinition(file, val string, varType cty.Type) (hclsyntax.Expression, hcl.Diagnostics) {
	switch varType {
	case cty.String, cty.Number, cty.NilType:
		return &hclsyntax.LiteralValueExpr{Val: cty.StringVal(val)}, nil
	default:
		return hclsyntax.ParseExpression([]byte(val), file, hcl.Pos{Line: 1, Column: 1})
	}
}

func GetVarsFromEnv() map[string]string {
	out := make(map[string]string)

	for _, raw := range os.Environ() {
		if !strings.HasPrefix(raw, VarEnvPrefix) {
			continue
		}
		raw = raw[len(VarEnvPrefix):] // trim the prefix

		eq := strings.Index(raw, "=")
		if eq == -1 {
			// Seems invalid, so we'll ignore it.
			continue
		}

		name := raw[:eq]
		value := raw[eq+1:]
		out[name] = value
	}

	return out
}
