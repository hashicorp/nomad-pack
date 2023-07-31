// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package variable

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/nomad-pack/internal/pkg/errors/packdiags"
	"github.com/hashicorp/nomad-pack/sdk/pack"
	"github.com/hashicorp/nomad-pack/sdk/pack/variables"
	"golang.org/x/exp/maps"
	"golang.org/x/exp/slices"
)

type PackID = pack.PackID
type VariableID = variables.VariableID
type Variable = variables.Variable

const VarEnvPrefix = "NOMAD_PACK_VAR_"

// ParsedVariables wraps the parsed variables returned by parser.Parse and
// provides functionality to access them.
type ParsedVariables struct {
	Vars     map[PackID]map[VariableID]*Variable
	Metadata *pack.Metadata
}

// ToPackTemplateContext creates a PackTemplateContext from this ParsedVariables
func (pv ParsedVariables) ToPackTemplateContext(p *pack.Pack) (PackTemplateContext, hcl.Diagnostics) {
	out := make(PackTemplateContext)
	diags := pv.toPackTemplateContextR(&out, p)
	return out, diags
}

func (pv ParsedVariables) toPackTemplateContextR(tgt *PackTemplateContext, p *pack.Pack) hcl.Diagnostics {
	pVars, diags := asMapOfStringToAny(pv.Vars[p.PackID()])
	if diags.HasErrors() {
		return diags
	}

	(*tgt)["_self"] = PackData{
		Pack: p,
		vars: pVars,
	}

	for _, d := range p.Dependencies() {
		out := make(PackTemplateContext)
		diags.Extend(pv.toPackTemplateContextR(&out, d))
		(*tgt)[d.AliasOrName()] = out
	}

	return diags
}

func asMapOfStringToAny(m map[VariableID]*Variable) (map[string]any, hcl.Diagnostics) {
	var diags hcl.Diagnostics
	o := make(map[string]any)
	for k, cVal := range m {
		val, err := variables.ConvertCtyToInterface(cVal.Value)
		if err != nil {
			diags = safeDiagnosticsAppend(diags, packdiags.DiagFailedToConvertCty(err, cVal.DeclRange.Ptr()))
			continue
		}
		o[string(k)] = val
	}
	return o, diags
}

// ConvertVariablesToMapOfAny translates the parsed variables into their
// native go types. The returned map is always keyed by the pack namespace for
// the variables.
//
// Even though parsing the variable went without error, it is highly possible
// that conversion to native go types can incur an error. If an error is
// returned, it should be considered terminal.
func (p *ParsedVariables) ConvertVariablesToMapOfAny() (map[string]any, hcl.Diagnostics) {

	// Create our output; no matter what we return something.
	out := make(map[string]any)

	// Errors can occur when performing the translation. We want to capture all
	// of these and return them to the user. This allows them to fix problems
	// in a single cycle.
	var diags hcl.Diagnostics

	packNames := maps.Keys(p.Vars)
	slices.Sort(packNames)
	fmt.Println(packNames)
	for _, packName := range packNames {
		fmt.Println(packName)
	}
	// Iterate each set of pack variable.
	for packName, variables := range p.Vars {

		// packVar collects all variables associated to a pack.
		packVar := map[VariableID]any{}

		// Convert each variable and add this to the pack map.
		for variableName, variable := range variables {
			varInterface, err := convertCtyToInterface(variable.Value)
			if err != nil {
				diags = safeDiagnosticsAppend(diags, packdiags.DiagFailedToConvertCty(err, variable.DeclRange.Ptr()))
				continue
			}
			packVar[variableName] = varInterface
		}

		// Add the pack variable to the full output.
		out[packName.String()] = packVar
	}

	return out, diags
}
func (vf ParsedVariables) String() string { return asJSON(vf) }

func asJSON(a any) string {
	return func() string { b, _ := json.MarshalIndent(a, "", "  "); return string(b) }()
}

func (vf ParsedVariables) AsOverrideFile() string {
	var out strings.Builder
	out.WriteString(vf.varFileHeader())

	packnames := maps.Keys(vf.Vars)
	slices.Sort(packnames)
	for _, packname := range packnames {
		vs := vf.Vars[packname]

		varnames := maps.Keys(vs)
		slices.Sort(varnames)
		for _, varname := range varnames {
			v := vs[varname]
			out.WriteString(v.AsOverrideString(packname))
		}
	}

	return out.String()
}

func (vf ParsedVariables) varFileHeader() string {
	// Use pack metadata to enhance the header if desired.
	// _ = vf.Metadata
	// This value will be added to the top of the varfile
	return ""
}
