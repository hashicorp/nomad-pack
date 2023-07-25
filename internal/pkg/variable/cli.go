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
	// Split the name to see if we have a namespace CLI variable for a child
	// pack and set the default packVarName.
	splitName := strings.SplitN(name, ".", 2)
	packVarName := []string{p.cfg.ParentName, name}

	switch len(splitName) {
	case 1:
		// Fallthrough, nothing to do or see.
	case 2:
		// We are dealing with a namespaced variable. Overwrite the preset
		// values of packVarName.
		packVarName[0] = splitName[0]
		packVarName[1] = splitName[1]
	default:
		// We cannot handle a splitName where the variable includes more than
		// one separator.
		return hcl.Diagnostics{
			{
				Severity: hcl.DiagError,
				Summary:  fmt.Sprintf("Invalid %s option", strings.TrimRight(VarEnvPrefix, "_")),
				Detail:   fmt.Sprintf("The given environment variable %s%s=%s is not correctly specified. The variable name must not have more than one dot `.` separator.", VarEnvPrefix, name, rawVal),
			},
		}
	}

	// Generate a filename based on the CLI var, so we have some context for any
	// HCL diagnostics.
	fakeRange := hcl.Range{Filename: fmt.Sprintf("<value for var.%s from environment>", name)}

	// If the variable has not been configured in the root then ignore it. This
	// is a departure from the way in which flags and var-files are handled.
	// The environment might contain NOMAD_PACK_VAR variables used for other
	// packs that might be run on the same system but are not used with this
	// particular pack.
	existing, exists := p.rootVars[packVarName[0]][packVarName[1]]
	if !exists {
		return nil
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
		Name:      packVarName[1],
		Type:      val.Type(),
		Value:     val,
		DeclRange: fakeRange,
	}
	p.envOverrideVars[packVarName[0]] = append(p.envOverrideVars[packVarName[0]], &v)

	return nil
}

func (p *Parser) parseCLIVariable(name string, rawVal string) hcl.Diagnostics {
	// Split the name to see if we have a namespace CLI variable for a child
	// pack and set the default packVarName.
	splitName := strings.SplitN(name, ".", 2)
	packVarName := []string{p.cfg.ParentName, name}

	switch len(splitName) {
	case 1:
		// Fallthrough, nothing to do or see.
	case 2:
		// We are dealing with a namespaced variable. Overwrite the preset
		// values of packVarName.
		packVarName[0] = splitName[0]
		packVarName[1] = splitName[1]
	default:
		// We cannot handle a splitName where the variable includes more than
		// one separator.
		return hcl.Diagnostics{
			{
				Severity: hcl.DiagError,
				Summary:  "Invalid -var option",
				Detail:   fmt.Sprintf("The given -var option %s=%s is not correctly specified. The variable name must not have more than one dot `.` separator.", name, rawVal),
			},
		}
	}

	// Generate a filename based on the CLI var, so we have some context for any
	// HCL diagnostics.
	fakeRange := hcl.Range{Filename: fmt.Sprintf("<value for var.%s from arguments>", name)}

	// If the variable has not been configured in the root then exit. This is a
	// standard requirement, especially because we would be unable to ensure a
	// consistent type.
	existing, exists := p.rootVars[packVarName[0]][packVarName[1]]
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
		Name:      packVarName[1],
		Type:      val.Type(),
		Value:     val,
		DeclRange: fakeRange,
	}
	p.cliOverrideVars[packVarName[0]] = append(p.cliOverrideVars[packVarName[0]], &v)

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
