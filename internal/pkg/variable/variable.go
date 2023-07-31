// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package variable

import (
	"encoding/json"
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

// ToPackTemplateContext creates a PackTemplateContext from this
// ParsedVariables.
// Even though parsing the variable went without error, it is highly
// possible that conversion to native go types can incur an error.
// If an error is returned, it should be considered terminal.
func (pv ParsedVariables) ToPackTemplateContext(p *pack.Pack) (PackTemplateContext, hcl.Diagnostics) {
	out := make(PackTemplateContext)
	diags := pv.toPackTemplateContextR(&out, p)
	return out, diags
}

func (pv ParsedVariables) toPackTemplateContextR(tgt *PackTemplateContext, p *pack.Pack) hcl.Diagnostics {
	pVars, diags := asMapOfStringToAny(pv.Vars[p.VariablesPath()])
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
