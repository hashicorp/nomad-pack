// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package variable

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/nomad-pack/sdk/pack"
	"github.com/mitchellh/go-wordwrap"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/convert"
)

const VarEnvPrefix = "NOMAD_PACK_VAR_"

// Variable encapsulates a single variable as defined within a block according
// to variableFileSchema and variableBlockSchema.
type Variable struct {

	// Name is the variable label. This is used to identify variables being
	// overridden and during templating.
	Name string

	// Description is an optional field which provides additional context to
	// users identifying what the variable is used for.
	Description    string
	hasDescription bool

	// Default is an optional field which provides a default value to be used
	// in the absence of a user-provided value. It is only in this struct for
	// documentation purposes
	Default    cty.Value
	hasDefault bool

	// Type represents the concrete cty type of this variable. If the type is
	// unable to be parsed into a cty type, it is invalid.
	Type    cty.Type
	hasType bool

	// Value stores the variable value and is used when converting the cty type
	// value into a Go type value.
	Value cty.Value

	// DeclRange is the position marker of the variable within the file it was
	// read from. This is used for diagnostics.
	DeclRange hcl.Range
}

func (v *Variable) merge(new *Variable) hcl.Diagnostics {
	var diags hcl.Diagnostics
	if new.Default != cty.NilVal {
		v.hasDefault = new.hasDefault
		v.Default = new.Default
	}

	if new.Value != cty.NilVal {
		v.Value = new.Value
	}

	if new.Type != cty.NilType {
		v.hasType = new.hasType
		v.Type = new.Type
	}

	if v.Value != cty.NilVal {
		val, err := convert.Convert(v.Value, v.Type)
		if err != nil {
			switch {
			case new.Type != cty.NilType && new.Value == cty.NilVal:
				diags = safeDiagnosticsAppend(diags,
					diagnosticInvalidDefaultValue(
						fmt.Sprintf("Overriding this variable's type constraint has made its default value invalid: %s.", err),
						new.DeclRange.Ptr(),
					))
			case new.Type == cty.NilType && new.Value != cty.NilVal:
				diags = safeDiagnosticsAppend(diags,
					diagnosticInvalidDefaultValue(
						fmt.Sprintf("The overridden default value for this variable is not compatible with the variable's type constraint: %s.", err),
						new.DeclRange.Ptr(),
					))
			default:
				diags = safeDiagnosticsAppend(diags,
					diagnosticInvalidDefaultValue(
						fmt.Sprintf("This variable's default value is not compatible with its type constraint: %s.", err),
						new.DeclRange.Ptr(),
					))
			}
		} else {
			v.Value = val
		}
	}

	return diags
}

// ParsedVariables wraps the parsed variables returned by parser.Parse and
// provides functionality to access them.
type ParsedVariables struct {
	Vars     map[string]map[string]*Variable
	Metadata *pack.Metadata
}

// ConvertVariablesToMapInterface translates the parsed variables into their
// native go types. The returned map is always keyed by the pack namespace for
// the variables.
//
// Even though parsing the variable went without error, it is highly possible
// that conversion to native go types can incur an error. If an error is
// returned, it should be considered terminal.
func (p *ParsedVariables) ConvertVariablesToMapInterface() (map[string]any, hcl.Diagnostics) {

	// Create our output; no matter what we return something.
	out := make(map[string]any)

	// Errors can occur when performing the translation. We want to capture all
	// of these and return them to the user. This allows them to fix problems
	// in a single cycle.
	var diags hcl.Diagnostics

	// Iterate each set of pack variable.
	for packName, variables := range p.Vars {

		// packVar collects all variables associated to a pack.
		packVar := map[string]any{}

		// Convert each variable and add this to the pack map.
		for variableName, variable := range variables {
			varInterface, err := convertCtyToInterface(variable.Value)
			if err != nil {
				diags = safeDiagnosticsAppend(diags, diagnosticFailedToConvertCty(err, variable.DeclRange.Ptr()))
				continue
			}
			packVar[variableName] = varInterface
		}

		// Add the pack variable to the full output.
		out[packName] = packVar
	}

	return out, diags
}

func (v Variable) String() string         { return asJSON(v) }
func (vf ParsedVariables) String() string { return asJSON(vf) }

func asJSON(a any) string {
	return func() string { b, _ := json.MarshalIndent(a, "", "  "); return string(b) }()
}

func (v Variable) AsOverrideString(packName string) string {
	var out strings.Builder
	out.WriteString(fmt.Sprintf(`# variable "%s.%s"`, packName, v.Name))
	out.WriteByte('\n')
	if v.hasDescription {
		tmp := "description: " + v.Description
		wrapped := wordwrap.WrapString(tmp, 80)
		lines := strings.Split(wrapped, "\n")
		for i, l := range lines {
			lines[i] = "#   " + l
		}
		wrapped = strings.Join(lines, "\n")
		out.WriteString(wrapped)
		out.WriteString("\n")
	}
	if v.hasType {
		out.WriteString(fmt.Sprintf("#   type: %s\n", printType(v.Type)))
	}

	if v.hasDefault {
		out.WriteString(fmt.Sprintf("#   default: %s\n", printDefault(v.Value)))
	}
	if v.hasDefault {
		out.WriteString(fmt.Sprintf("#\n# %s.%s=%s\n\n", packName, v.Name, printDefault(v.Value)))
	}
	out.WriteString("\n")
	return out.String()
}

func (vf ParsedVariables) AsOverrideFile() string {
	var out strings.Builder
	out.WriteString(vf.varFileHeader())
	// TODO: this should have a stable order.
	for p, vs := range vf.Vars {
		for _, v := range vs {
			out.WriteString(v.AsOverrideString(p))
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
