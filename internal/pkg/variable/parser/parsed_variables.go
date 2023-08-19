package parser

import (
	"encoding/json"
	"errors"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/nomad-pack/internal/pkg/errors/packdiags"
	"github.com/hashicorp/nomad-pack/internal/pkg/variable/parser/config"
	"github.com/hashicorp/nomad-pack/internal/pkg/variable/parser/converter"
	"github.com/hashicorp/nomad-pack/sdk/pack"
	"github.com/hashicorp/nomad-pack/sdk/pack/variables"
	"golang.org/x/exp/maps"
	"golang.org/x/exp/slices"
)

// ParsedVariables wraps the parsed variables returned by parser.Parse and
// provides functionality to access them.
type ParsedVariables struct {
	v1Vars   map[string]map[string]*Variable
	v2Vars   map[PackID]map[VariableID]*Variable
	Metadata *pack.Metadata
	version  *config.ParserVersion
}

func (p *ParsedVariables) IsV2() bool {
	return *p.version == config.V2
}

func (p *ParsedVariables) IsV1() bool {
	return *p.version == config.V1
}

func (p *ParsedVariables) isLoaded() bool {
	return !(p.version == nil)
}

func (p *ParsedVariables) LoadV1Result(in map[string]map[string]*Variable) error {
	if p.isLoaded() {
		return errors.New("already loaded")
	}
	var vPtr config.ParserVersion = config.V1
	p.v1Vars = maps.Clone(in)
	p.version = &vPtr
	return nil
}

func (p *ParsedVariables) LoadV2Result(in map[PackID]map[VariableID]*Variable) error {
	if p.isLoaded() {
		return errors.New("already loaded")
	}
	var vPtr config.ParserVersion = config.V2
	p.v2Vars = maps.Clone(in)
	p.version = &vPtr
	return nil
}

func (p *ParsedVariables) GetVars() map[PackID]map[VariableID]*Variable {
	if !p.isLoaded() {
		return nil
	}
	if *p.version == config.V1 {
		return asV2Vars(p.v1Vars)
	}
	return p.v2Vars
}

func asV2Vars(in map[string]map[string]*Variable) map[PackID]map[VariableID]*Variable {
	var out = make(map[PackID]map[VariableID]*Variable, len(in))
	for k, vs := range in {
		out[PackID(k)] = make(map[VariableID]*Variable, len(vs))
		for vk, v := range vs {
			out[PackID(k)][VariableID(vk)] = v
		}
	}
	return out
}

// NOTE: Beyond here, things get weird.

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
	pVars, diags := asMapOfStringToAny(pv.v2Vars[p.VariablesPath()])
	if diags.HasErrors() {
		return diags
	}

	(*tgt)["_self"] = PackData{
		Pack: p,
		vars: pVars,
		meta: p.Metadata.ConvertToMapInterface(),
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
			diags = packdiags.SafeDiagnosticsAppend(diags, packdiags.DiagFailedToConvertCty(err, cVal.DeclRange.Ptr()))
			continue
		}
		o[string(k)] = val
	}
	return o, diags
}

func (pv ParsedVariables) String() string { return asJSON(pv) }

func asJSON(a any) string {
	return func() string { b, _ := json.MarshalIndent(a, "", "  "); return string(b) }()
}

func (pv ParsedVariables) AsOverrideFile() string {
	var out strings.Builder
	out.WriteString(pv.varFileHeader())

	packnames := maps.Keys(pv.v2Vars)
	slices.Sort(packnames)
	for _, packname := range packnames {
		vs := pv.v2Vars[packname]

		varnames := maps.Keys(vs)
		slices.Sort(varnames)
		for _, varname := range varnames {
			v := vs[varname]
			out.WriteString(v.AsOverrideString(packname))
		}
	}

	return out.String()
}

func (pv ParsedVariables) varFileHeader() string {
	// Use pack metadata to enhance the header if desired.
	// _ = vf.Metadata
	// This value will be added to the top of the varfile
	return ""
}

func (p *ParsedVariables) ConvertVariablesToMapInterface() (map[string]any, hcl.Diagnostics) {

	// Create our output; no matter what we return something.
	out := make(map[string]any)

	// Errors can occur when performing the translation. We want to capture all
	// of these and return them to the user. This allows them to fix problems
	// in a single cycle.
	var diags hcl.Diagnostics

	// Iterate each set of pack variable.
	for packName, variables := range p.v1Vars {

		// packVar collects all variables associated to a pack.
		packVar := map[string]any{}

		// Convert each variable and add this to the pack map.
		for variableName, variable := range variables {
			varInterface, err := converter.ConvertCtyToInterface(variable.Value)
			if err != nil {
				diags = packdiags.SafeDiagnosticsAppend(diags, packdiags.DiagFailedToConvertCty(err, variable.DeclRange.Ptr()))
				continue
			}
			packVar[variableName] = varInterface
		}

		// Add the pack variable to the full output.
		out[packName] = packVar
	}

	return out, diags
}